# go-tdx

通达信（TDX）A股行情协议的 Go 客户端 —— 从 Python `easy_tdx` 移植数据层，用于构建 Go web 服务 + UI。

范围：**仅 A股标准协议（Windows）**。不含回测/因子/缠论/策略扫描，不含 mac/ex 扩展市场。

## 状态 —— 库已完成

| 层 | 状态 | 备注 |
|---|---|---|
| codec: price/volume/datetime/frame/gbk | ✅ | fixture 字节级 parity |
| codec: price_rules（涨跌停规则） | ✅ | 单元测试 7 例 |
| transport: 连接/握手/帧/ping/best-host | ✅ | 真实服务器验证 |
| 连接池 `tdx.Pool`（Acquire/Release/Do） | ✅ | 多 goroutine 共享 + 限流；多主机分摊、懒重建、可选 keepalive |
| command（全 13 条 A股命令） | ✅ | 12 fixture parity 全过 |
| facade `tdx.Client`（高层 API） | ✅ | live 验证全部方法 |
| model（typed struct，JSON 友好） | ✅ | |
| 演示 CLI (`cmd/tdx`) | ✅ | `go run ./cmd/tdx -code 000001` |

**命令清单**：SecurityCount / SecurityList(All) / SecurityQuotes / PriceLimits /
SecurityBars / IndexBars / MinuteTime / HistoryMinuteTime / Transaction /
HistoryTransaction / FinanceInfo / XdxrInfo / CompanyInfoCategory / CompanyInfoContent /
HistoryFundFlow / BlockInfoMeta / BlockInfoFile / ReportFile / MarketStat。

live 验证：ping→握手→拉真实行情/财务/除权/市场统计/涨跌停，全部正确。

## 连接池

- `NewPool("", size, timeout)`：自动 ping 候选服务器，取延迟最低的至多 3 台轮转分摊，聚合吞吐随服务器数提升；`host` 支持 `"1.2.3.4"` 或 `"1.2.3.4:7709"`。
- `NewPoolWith(host, size, timeout, tdx.WithHosts(...), tdx.WithKeepalive(60*time.Second))`：显式候选列表 / 空闲心跳（对冲服务器掐空闲连接）。
- `Do(ctx, fn)` 仅在传输层错误（`*transport.CommError`，连接不可信）时弃用连接、重拨并重试一次；业务/解析错误原样返回。死连接不回池，由下次 Acquire 懒重建。

## 待办（已超出"库"范围）

- security_list 行业关联（tdxhy.cfg 解析）—— 按需。
- offline .dat 本地读取 —— 按 UI 需求。
- web 层 / 前端 → 见 `../go-tdx-web`。

## 移植约定

- `parse_response(body)` → Go `ParseResponse(body []byte) (T, error)`，body = 解压后字节。
- `_to_df()` 返回 DataFrame → Go 返回 typed struct slice，web 层直接转 JSON。
- sync/async 双镜像 → Go 单实现 + goroutine/context。
- 测试：复用 `easy_tdx/tests/fixtures/*.hex`（解压后 body 的 hex）+ `.json`（期望值），逐字节对照。`testdata/` 软链到该目录。

## 运行

```bash
go test ./...                              # parity 测试
go run ./cmd/tdx -market 0 -code 000001    # 拉 平安银行 日K
```

## 风险点（已验证 OK，改动时复测）

- 价格变长编码（codec/price.go）：类 LEB128 + 符号位 + 差分还原。
- frame zlib：16 字节头，zipsize==unzipsize 时不解压。
- 握手：连接后顺序发 3 条 setup，丢弃响应；部分服务器无响应需容错。
