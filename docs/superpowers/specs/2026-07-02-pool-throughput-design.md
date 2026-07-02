# go-tdx 连接池吞吐改造设计

日期：2026-07-02
状态：已定稿，待实现

## 背景与问题

go-tdx 的吞吐天花板 = 池大小 × 单连接串行速度：

- `transport.Conn.Execute` 互斥锁串行（transport/conn.go:16），单连接内命令严格顺序执行。
- `Pool` 是唯一并发手段（pool.go:14），所有连接连同一台 TDX 服务器。
- `Pool.Do` 对任何错误（含业务/解析错误）都触发重连+重试；reconnect 失败时会把死连接还回池里，后续调用者继续踩坑。
- `NewPool` 串行拨号 size 条连接，启动延迟 = size × RTT。
- `Close`/`Release` 存在竞态：Release 检查 closed 标志与向 channel 发送之间，Close 可能关闭 channel，导致 send on closed channel panic。
- TDX 服务器会掐空闲连接，当前无 keepalive，空闲后首个请求必然失败一次（隐性延迟税：失败 + 重连 + 重试）。

## 否决的方案

**协议 pipelining（单连接并发）**：帧头 SeqID 只回显请求 bytes 1-4，同命令并发请求无法区分；且 `sendSetup` 已证实部分服务器会丢响应（transport/conn.go:100 注释），FIFO 匹配一旦丢一条响应即永久错位。风险不可控，违背本库与 easy_tdx 字节级 parity 的保守设计哲学。不做。

**只调大池 size**：不解决死连接循环、启动串行慢、单服务器限流问题。不做。

## 选定方案：Pool 深度改造

连接层串行语义不动，全部改动集中在池层与错误分型。

### 1. 槽位化池（pool.go）

- `conns chan *Client` 改为 `slots chan *slot`，`slot{c *Client; host string}`，channel 容量恒定等于池 size。
- 懒建连：slot.c == nil 时在 Acquire 内拨号。失败的连接从槽位清除（置 nil），不再把死连接还回池。
- `NewPool` 并行拨号全部槽位；只要 ≥1 条成功即返回成功，失败槽位置 nil 留待懒补；全部失败才报错。

### 2. 多主机分摊

- `host == ""`：`transport.PingAll` 排序后取前 3 台可达主机，槽位按轮转分配主机 → 每台服务器限流分摊，聚合吞吐随主机数线性提升。
- `host != ""`：保持单主机行为，完全向后兼容。
- 槽位重拨失败时，按候选主机表顺序换下一台（故障转移粒度从池级降到槽级）。

### 3. 错误分型（transport 包）

- 新增 `transport.CommError`（包装底层错误，实现 `Unwrap`），`Conn.Execute` 的 write / header / body / 未连接错误全部用它包装。
- `Pool.Do` 用 `errors.As` 判断：
  - `*CommError` → 关闭旧连接、槽位重拨、用新连接重试 fn 一次（读操作幂等）。
  - 其他错误（业务/解析）→ 原样返回，不重连不重试。

### 4. Close/Release 竞态修复

- `Close` 不再 close(channel)。改为：置 closed 标志（互斥锁保护）后，非阻塞排空 channel 并逐个关闭连接。
- `Release` 在 closed 后收到的连接直接 Close，不向 channel 发送。send 只在持有「未关闭」证明的路径上发生（Release 全程持锁检查 + 发送用 select default 兜底）。

### 5. Keepalive（可选）

- `WithKeepalive(interval time.Duration)` 选项：后台 goroutine 周期性对池内空闲槽位执行轻量命令（SecurityCount），失败即把槽位置 nil 待懒补。
- 默认关闭，零行为变化。shining_trader 可开 60s。

## API 兼容性

| 现有 API | 状态 |
|---|---|
| `NewPool(host, size, timeout)` | 签名不变，内部行为升级（并行拨号、host=="" 多主机） |
| `Pool.Do(ctx, fn)` | 签名不变，重试条件收窄为 CommError |
| `Acquire/Release/Close` | 签名不变 |
| `NewPoolWith(host, size, timeout, opts ...PoolOption)` | 新增，承载 WithKeepalive / WithHosts 等选项 |

消费方 shining_trader 仅使用 `NewPool` + `Do`，无需改动。

## 测试策略

进程内 fake TDX server（net.Listener，讲 16 字节帧协议：magic 7654321、seqID 回显、zipsize==unzipsize 不压缩路径），可注入行为：正常应答、丢响应、读到一半断连、延迟应答。

用例：
1. 并发 Do 正确性（N goroutine × M 命令，-race）。
2. 服务器断连后 Do 自动重拨重试成功；死连接不回池。
3. 业务错误（fn 返回非 CommError）不触发重连。
4. Close 与并发 Release/Do 无 panic、无泄漏（-race）。
5. host=="" 多主机轮转分配；单主机路径不变。
6. NewPool 部分拨号失败仍可用；全部失败报错。

现有 parity 测试（离线 fixture）不受影响，须全绿。

## 预期效果

- 吞吐上限：`1 服务器 × size × 串行` → `≤3 服务器 × size × 串行`。
- 启动延迟：size × RTT → ~1 × RTT。
- 消除死连接循环与空闲断连的隐性延迟税。

## 未来工作（本次不做）

- 协议 pipelining：需先对真实服务器做响应顺序/丢包行为测量，才能评估可行性。
- 动态池扩缩容。
