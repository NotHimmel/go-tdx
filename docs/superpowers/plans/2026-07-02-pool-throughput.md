# go-tdx 连接池吞吐改造 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把 go-tdx 吞吐上限从「1 服务器 × 池 size × 串行」抬到「≤3 服务器 × 池 size × 串行」，并消除死连接循环、Close/Release 竞态与空闲断连延迟税。

**Architecture:** 连接层（transport.Conn）串行语义不动。改动集中在：transport 错误分型（CommError）、池槽位化（slot + 懒建连）、多主机轮转、可选 keepalive。测试用进程内 fake TDX server。

**Tech Stack:** Go 1.26.4，仅标准库。模块 `github.com/NotHimmel/go-tdx`。

## Global Constraints

- 公开 API 签名不变：`NewPool(host string, size int, timeout time.Duration) (*Pool, error)`、`Pool.Do(ctx, fn)`、`Acquire/Release/Close`。消费方 shining_trader 依赖 v0.2.1 发布版，仅用 NewPool + Do。
- 协议字节不改动（parity 哲学）：不做 pipelining，不改 command/codec 包。
- 现有离线 parity 测试必须全绿：`go test ./...`。
- 所有注释、错误消息用中文，风格与现有代码一致。
- 每个任务结束跑 `go test ./... -race` 后提交。

---

### Task 1: transport.CommError 错误分型

**Files:**
- Create: `transport/errors.go`
- Modify: `transport/conn.go`（Execute 内错误包装）
- Test: `transport/errors_test.go`

**Interfaces:**
- Produces: `type CommError struct { Op string; Err error }`，方法 `Error() string`、`Unwrap() error`。Task 3 的 Pool.Do 用 `errors.As(err, &ce)` 判断是否重连重试。

- [ ] **Step 1: 写失败测试**

```go
// transport/errors_test.go
package transport

import (
	"errors"
	"testing"
	"time"
)

// 未连接时 Execute 必须返回 *CommError（Pool 依赖该分型决定重连）。
func TestExecuteNotConnectedIsCommError(t *testing.T) {
	c := NewConn("127.0.0.1", 1, time.Second)
	_, err := c.Execute([]byte{0x0c})
	if err == nil {
		t.Fatal("期望错误，得到 nil")
	}
	var ce *CommError
	if !errors.As(err, &ce) {
		t.Fatalf("期望 *CommError，得到 %T: %v", err, err)
	}
	if ce.Op != "not-connected" {
		t.Fatalf("期望 Op=not-connected，得到 %q", ce.Op)
	}
}

// 连接后对端立刻断开，Execute 的读错误也必须是 *CommError。
func TestExecuteReadErrorIsCommError(t *testing.T) {
	// 本测试在 Task 2 引入 faketdx 后补充网络路径；此处先验证类型系统。
	err := error(&CommError{Op: "read-header", Err: errors.New("EOF")})
	var ce *CommError
	if !errors.As(err, &ce) {
		t.Fatal("errors.As 应命中 *CommError")
	}
	if got := ce.Error(); got != "通信错误(read-header): EOF" {
		t.Fatalf("错误文案不符: %q", got)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd /mnt/data/tz/snap/rust/go-tdx && go test ./transport/ -run TestExecute -v`
Expected: FAIL，`undefined: CommError`

- [ ] **Step 3: 实现 CommError 并改造 Execute**

```go
// transport/errors.go
package transport

import "fmt"

// CommError 表示传输层通信错误（未连接/写失败/读失败/解压失败）。
// 出现该错误说明连接内的字节流状态已不可信，调用方（如 Pool）应弃用
// 该连接并重拨；业务解析错误不属于此类。
type CommError struct {
	Op  string // not-connected / write / read-header / read-body / decompress
	Err error
}

func (e *CommError) Error() string {
	if e.Err == nil {
		return fmt.Sprintf("通信错误(%s)", e.Op)
	}
	return fmt.Sprintf("通信错误(%s): %v", e.Op, e.Err)
}

func (e *CommError) Unwrap() error { return e.Err }
```

`transport/conn.go` 的 Execute 改为（整个函数替换）：

```go
// Execute 发送请求，接收并解压响应，返回解压后的 body。
// 调用方用 cmd.ParseResponse(body) 解析。
// 传输层失败返回 *CommError，此时连接状态不可信，应弃用重拨。
func (c *Conn) Execute(request []byte) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.sock == nil {
		return nil, &CommError{Op: "not-connected"}
	}
	c.sock.SetDeadline(time.Now().Add(c.Timeout))

	if _, err := c.sock.Write(request); err != nil {
		return nil, &CommError{Op: "write", Err: err}
	}
	headerBuf, err := c.recvExact(codec.HeaderSize)
	if err != nil {
		return nil, &CommError{Op: "read-header", Err: err}
	}
	header, err := codec.ParseHeader(headerBuf)
	if err != nil {
		return nil, &CommError{Op: "read-header", Err: err}
	}
	rawBody, err := c.recvExact(int(header.ZipSize))
	if err != nil {
		return nil, &CommError{Op: "read-body", Err: err}
	}
	body, err := codec.DecompressBody(header, rawBody)
	if err != nil {
		return nil, &CommError{Op: "decompress", Err: err}
	}
	return body, nil
}
```

注意：ParseHeader 失败与解压失败意味着流已错位，同样归为 CommError。旧文案「未连接，请先调用 Connect()」变为「通信错误(not-connected)」，属可接受变更（无消费方匹配该字符串）。

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./transport/ -run TestExecute -v`
Expected: PASS × 2

- [ ] **Step 5: 全量回归 + 提交**

Run: `go test ./... -race`
Expected: 全部 PASS（parity 测试不受影响）

```bash
git add transport/errors.go transport/errors_test.go transport/conn.go
git commit -m "feat(transport): CommError 传输层错误分型"
```

---

### Task 2: fake TDX server 测试基建 + host:port 支持

**Files:**
- Create: `internal/faketdx/faketdx.go`
- Modify: `tdx.go:30-42`（NewWithTimeout 支持 "host:port"）
- Test: `tdx_hostport_test.go`（模块根，package tdx）

**Interfaces:**
- Consumes: Task 1 的 `*transport.CommError`（faketdx 的 Drop/CloseConn 行为触发它）
- Produces:
  - `faketdx.Start(t *testing.T, handler func(req []byte) Response) *Server`
  - `(*Server).Addr() string` → "127.0.0.1:随机端口"，可直接传给 `NewWithTimeout` / `NewPool`
  - `faketdx.Response{Body []byte; Drop, CloseConn bool; Delay time.Duration}`
  - `tdx.NewWithTimeout("127.0.0.1:12345", timeout)` 解析自定义端口

- [ ] **Step 1: 写 fake server**

```go
// internal/faketdx/faketdx.go
// Package faketdx 提供进程内 fake TDX 服务器，讲 16 字节响应帧协议，
// 供 pool/transport 单测注入丢响应、断连、延迟等故障。
package faketdx

import (
	"encoding/binary"
	"net"
	"sync/atomic"
	"testing"
	"time"
)

// Response 控制对一条数据请求的应答行为。
type Response struct {
	Body      []byte        // 应答 body（不压缩：zipsize == unzipsize）
	Drop      bool          // 读请求但不应答（模拟服务器丢响应）
	CloseConn bool          // 读请求后直接断开（模拟服务器掐连接）
	Delay     time.Duration // 应答前延迟
}

// Server 进程内 fake TDX 服务器。每连接前 3 条请求视为握手，自动回空帧。
type Server struct {
	ln       net.Listener
	handler  func(req []byte) Response
	accepted atomic.Int64 // 接受过的连接数
	requests atomic.Int64 // 收到的数据请求数（不含握手）
}

// Start 启动服务器，t.Cleanup 自动关闭。handler 为 nil 时一律回空 body。
func Start(t *testing.T, handler func(req []byte) Response) *Server {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("faketdx listen: %v", err)
	}
	s := &Server{ln: ln, handler: handler}
	go s.acceptLoop()
	t.Cleanup(func() { ln.Close() })
	return s
}

// Addr 返回 "127.0.0.1:port"，可直接传给 tdx.NewPool / NewWithTimeout。
func (s *Server) Addr() string { return s.ln.Addr().String() }

// Accepted 返回累计接受的连接数。
func (s *Server) Accepted() int { return int(s.accepted.Load()) }

// Requests 返回累计数据请求数（不含握手）。
func (s *Server) Requests() int { return int(s.requests.Load()) }

func (s *Server) acceptLoop() {
	for {
		c, err := s.ln.Accept()
		if err != nil {
			return
		}
		s.accepted.Add(1)
		go s.serve(c)
	}
}

// serve 假设一次 Read 恰好读到一条完整请求。客户端 Execute 严格
// 一问一答且请求为单次小包写入，loopback 上该假设成立。
func (s *Server) serve(c net.Conn) {
	defer c.Close()
	buf := make([]byte, 4096)
	setup := 0
	for {
		n, err := c.Read(buf)
		if err != nil {
			return
		}
		req := append([]byte(nil), buf[:n]...)
		if setup < 3 { // Connect() 的 3 条握手命令
			setup++
			writeFrame(c, req, nil)
			continue
		}
		s.requests.Add(1)
		resp := Response{}
		if s.handler != nil {
			resp = s.handler(req)
		}
		if resp.Delay > 0 {
			time.Sleep(resp.Delay)
		}
		if resp.CloseConn {
			return
		}
		if resp.Drop {
			continue
		}
		writeFrame(c, req, resp.Body)
	}
}

// writeFrame 组 16 字节帧头 + 未压缩 body（zipsize == unzipsize）。
func writeFrame(c net.Conn, req, body []byte) {
	h := make([]byte, 16)
	binary.LittleEndian.PutUint32(h[0:], 7654321)
	var seq uint32
	if len(req) >= 5 {
		seq = binary.LittleEndian.Uint32(req[1:5])
	}
	binary.LittleEndian.PutUint32(h[4:], seq)
	binary.LittleEndian.PutUint32(h[8:], 0)
	binary.LittleEndian.PutUint16(h[12:], uint16(len(body)))
	binary.LittleEndian.PutUint16(h[14:], uint16(len(body)))
	c.Write(append(h, body...))
}
```

- [ ] **Step 2: 写失败测试（host:port 解析）**

```go
// tdx_hostport_test.go
package tdx

import (
	"testing"
	"time"

	"github.com/NotHimmel/go-tdx/internal/faketdx"
	"github.com/NotHimmel/go-tdx/model"
)

// NewWithTimeout 应支持 "host:port" 形式，端口覆盖 DefaultPort。
func TestNewWithTimeoutHostPort(t *testing.T) {
	srv := faketdx.Start(t, func(req []byte) faketdx.Response {
		return faketdx.Response{Body: []byte{0x02, 0x00}} // SecurityCount = 2
	})
	c, err := NewWithTimeout(srv.Addr(), 2*time.Second)
	if err != nil {
		t.Fatalf("连接 fake server 失败: %v", err)
	}
	defer c.Close()
	n, err := c.SecurityCount(model.MarketSH)
	if err != nil {
		t.Fatalf("SecurityCount: %v", err)
	}
	if n != 2 {
		t.Fatalf("期望 2，得到 %d", n)
	}
}
```

- [ ] **Step 3: 跑测试确认失败**

Run: `go test . -run TestNewWithTimeoutHostPort -v`
Expected: FAIL，连接被拒（faketdx 端口是随机的，而 NewWithTimeout 固定拨 7709）

- [ ] **Step 4: 实现 host:port 解析**

`tdx.go` 的 NewWithTimeout 整体替换：

```go
// NewWithTimeout 同 New，可指定超时。host 支持 "1.2.3.4" 或 "1.2.3.4:7709"，
// 未带端口时用 transport.DefaultPort。
func NewWithTimeout(host string, timeout time.Duration) (*Client, error) {
	if host == "" {
		host = transport.BestHost(nil, transport.DefaultPort, 3*time.Second)
		if host == "" {
			return nil, fmt.Errorf("无可达服务器")
		}
	}
	port := transport.DefaultPort
	if h, p, err := net.SplitHostPort(host); err == nil {
		if n, e := strconv.Atoi(p); e == nil && n > 0 {
			host, port = h, n
		}
	}
	conn := transport.NewConn(host, port, timeout)
	if err := conn.Connect(); err != nil {
		return nil, err
	}
	return &Client{conn: conn}, nil
}
```

imports 增加 `"net"`、`"strconv"`。

- [ ] **Step 5: 跑测试确认通过 + 全量回归 + 提交**

Run: `go test . -run TestNewWithTimeoutHostPort -v && go test ./... -race`
Expected: 全部 PASS

```bash
git add internal/faketdx/faketdx.go tdx_hostport_test.go tdx.go
git commit -m "feat: faketdx 测试服务器 + NewWithTimeout 支持 host:port"
```

---

### Task 3: Pool 槽位化重写（懒建连 / 分型重试 / Close 竞态修复 / 并行拨号）

**Files:**
- Modify: `pool.go`（整文件重写）
- Test: `pool_test.go`（新建）

**Interfaces:**
- Consumes: `*transport.CommError`（Task 1）、`faketdx`（Task 2）、`NewWithTimeout` host:port（Task 2）
- Produces（Task 4/5 在此基础上扩展）:
  - `type slot struct { c *Client; host string; lastUse time.Time }`
  - `type PoolOption func(*Pool)`；`NewPoolWith(host string, size int, timeout time.Duration, opts ...PoolOption) (*Pool, error)`
  - 内部方法 `p.dial(prefer string) (*Client, error)`、`p.putSlot(s *slot)`、`p.clientAddr(c *Client) string`
  - 公开 API `NewPool/Acquire/Release/Do/Close` 签名不变

- [ ] **Step 1: 写失败测试**

```go
// pool_test.go
package tdx

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/NotHimmel/go-tdx/internal/faketdx"
	"github.com/NotHimmel/go-tdx/model"
)

func countBody(n int) []byte { return []byte{byte(n), byte(n >> 8)} }

// 并发 Do 全部成功（-race 下验证池的线程安全）。
func TestPoolDoConcurrent(t *testing.T) {
	srv := faketdx.Start(t, func(req []byte) faketdx.Response {
		return faketdx.Response{Body: countBody(7)}
	})
	p, err := NewPool(srv.Addr(), 4, 2*time.Second)
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	defer p.Close()
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := p.Do(context.Background(), func(c *Client) error {
				n, e := c.SecurityCount(model.MarketSH)
				if e != nil {
					return e
				}
				if n != 7 {
					t.Errorf("期望 7，得到 %d", n)
				}
				return nil
			})
			if err != nil {
				t.Errorf("Do: %v", err)
			}
		}()
	}
	wg.Wait()
}

// 服务器断连（CommError）→ Do 自动重拨并重试成功。
func TestPoolRetryOnCommError(t *testing.T) {
	var mu sync.Mutex
	killed := false
	srv := faketdx.Start(t, func(req []byte) faketdx.Response {
		mu.Lock()
		defer mu.Unlock()
		if !killed {
			killed = true
			return faketdx.Response{CloseConn: true}
		}
		return faketdx.Response{Body: countBody(7)}
	})
	p, err := NewPool(srv.Addr(), 1, 2*time.Second)
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	defer p.Close()
	err = p.Do(context.Background(), func(c *Client) error {
		_, e := c.SecurityCount(model.MarketSH)
		return e
	})
	if err != nil {
		t.Fatalf("期望重试成功，得到 %v", err)
	}
}

// 业务错误不触发重连：连接数保持初始值，fn 只执行一次。
func TestPoolBusinessErrorNoRetry(t *testing.T) {
	srv := faketdx.Start(t, nil)
	p, err := NewPool(srv.Addr(), 2, 2*time.Second)
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	defer p.Close()
	base := srv.Accepted()
	calls := 0
	bizErr := errors.New("业务错误")
	err = p.Do(context.Background(), func(c *Client) error {
		calls++
		return bizErr
	})
	if !errors.Is(err, bizErr) {
		t.Fatalf("期望业务错误原样返回，得到 %v", err)
	}
	if calls != 1 {
		t.Fatalf("业务错误不应重试，fn 执行了 %d 次", calls)
	}
	if srv.Accepted() != base {
		t.Fatalf("业务错误不应触发重连，新增连接 %d", srv.Accepted()-base)
	}
}

// Close 与并发 Do/Release 无 panic（旧实现存在 send on closed channel 竞态）。
func TestPoolCloseRace(t *testing.T) {
	srv := faketdx.Start(t, func(req []byte) faketdx.Response {
		return faketdx.Response{Body: countBody(1)}
	})
	p, err := NewPool(srv.Addr(), 4, 2*time.Second)
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
				p.Do(ctx, func(c *Client) error {
					_, e := c.SecurityCount(model.MarketSH)
					return e
				})
				cancel()
			}
		}()
	}
	time.Sleep(10 * time.Millisecond)
	p.Close()
	wg.Wait() // 只要求不 panic、不死锁
}

// 死连接不回池：故障期后池自动恢复（懒建连）。
func TestPoolLazyRecovery(t *testing.T) {
	var mu sync.Mutex
	broken := true
	srv := faketdx.Start(t, func(req []byte) faketdx.Response {
		mu.Lock()
		defer mu.Unlock()
		if broken {
			return faketdx.Response{CloseConn: true}
		}
		return faketdx.Response{Body: countBody(7)}
	})
	p, err := NewPool(srv.Addr(), 1, 2*time.Second)
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	defer p.Close()
	// 故障期：Do 可能失败（重试的新连接也被掐）
	p.Do(context.Background(), func(c *Client) error {
		_, e := c.SecurityCount(model.MarketSH)
		return e
	})
	mu.Lock()
	broken = false
	mu.Unlock()
	// 恢复期：懒建连应让 Do succeed
	err = p.Do(context.Background(), func(c *Client) error {
		_, e := c.SecurityCount(model.MarketSH)
		return e
	})
	if err != nil {
		t.Fatalf("故障恢复后 Do 应成功，得到 %v", err)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test . -run TestPool -v`
Expected: TestPoolBusinessErrorNoRetry FAIL（旧实现任何错误都重连重试）；TestPoolCloseRace 可能 panic；其余可能通过

- [ ] **Step 3: 重写 pool.go**

整文件替换：

```go
package tdx

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/NotHimmel/go-tdx/transport"
)

// slot 是池中一个连接位。c 为 nil 表示该位当前无活连接，
// 由下一个 Acquire 懒拨号补上 —— 死连接因此永远不会在池内循环。
type slot struct {
	c       *Client
	host    string // 该槽位偏好的服务器（"host" 或 "host:port"）
	lastUse time.Time
}

// Pool 是 Client 连接池：多 goroutine（如 web 多请求）共享有限连接。
// 单条 TDX 连接内命令串行；池让并发请求分散到多条连接（可跨多台服务器），
// 并对单台 TDX 服务器限流。
type Pool struct {
	timeout   time.Duration
	slots     chan *slot
	keepalive time.Duration
	stopKA    chan struct{}

	mu     sync.Mutex
	hosts  []string // 候选服务器，按延迟升序；槽位轮转分配
	closed bool
}

// PoolOption 配置 NewPoolWith 的可选项。
type PoolOption func(*Pool)

// WithHosts 显式指定候选服务器列表（覆盖自动探测与 host 参数）。
func WithHosts(hosts []string) PoolOption {
	return func(p *Pool) { p.hosts = append([]string(nil), hosts...) }
}

// maxAutoHosts 自动探测时最多使用的服务器数。
const maxAutoHosts = 3

// NewPool 创建 size 条连接的池。host 为空自动 ping 候选列表，
// 取延迟最低的至多 maxAutoHosts 台服务器轮转分摊。
func NewPool(host string, size int, timeout time.Duration) (*Pool, error) {
	return NewPoolWith(host, size, timeout)
}

// NewPoolWith 同 NewPool，可附加选项（WithHosts / WithKeepalive）。
// 并行拨号全部槽位；只要有一条成功即可用，失败槽位由后续 Acquire 懒补。
func NewPoolWith(host string, size int, timeout time.Duration, opts ...PoolOption) (*Pool, error) {
	if size <= 0 {
		size = 4
	}
	p := &Pool{
		timeout: timeout,
		slots:   make(chan *slot, size),
		stopKA:  make(chan struct{}),
	}
	for _, o := range opts {
		o(p)
	}
	if len(p.hosts) == 0 {
		if host != "" {
			p.hosts = []string{host}
		} else {
			rs := transport.PingAll(nil, transport.DefaultPort, 3*time.Second)
			if len(rs) == 0 {
				return nil, fmt.Errorf("无可达 TDX 服务器")
			}
			n := min(len(rs), maxAutoHosts)
			for i := 0; i < n; i++ {
				p.hosts = append(p.hosts, rs[i].Host)
			}
		}
	}
	clients := make([]*Client, size)
	var wg sync.WaitGroup
	for i := 0; i < size; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if c, err := NewWithTimeout(p.hosts[i%len(p.hosts)], timeout); err == nil {
				clients[i] = c
			}
		}(i)
	}
	wg.Wait()
	ok := 0
	for i := 0; i < size; i++ {
		s := &slot{c: clients[i], host: p.hosts[i%len(p.hosts)], lastUse: time.Now()}
		if s.c != nil {
			ok++
		}
		p.slots <- s
	}
	if ok == 0 {
		p.Close()
		return nil, fmt.Errorf("连接池初始化失败：%d 条连接全部失败", size)
	}
	if p.keepalive > 0 {
		go p.keepaliveLoop()
	}
	return p, nil
}

// Acquire 取一条连接（阻塞直到可用或 ctx 取消）。用完必须 Release。
// 空槽位在此懒拨号；拨号失败时空槽位归还，错误返回给调用方。
func (p *Pool) Acquire(ctx context.Context) (*Client, error) {
	select {
	case s := <-p.slots:
		if s.c != nil {
			return s.c, nil
		}
		c, err := p.dial(s.host)
		if err != nil {
			p.putSlot(&slot{host: s.host})
			return nil, err
		}
		return c, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Release 归还连接。
func (p *Pool) Release(c *Client) {
	if c == nil {
		return
	}
	p.putSlot(&slot{c: c, host: p.clientAddr(c), lastUse: time.Now()})
}

// Do 取连接执行 fn 并自动归还。web handler 的推荐入口。
// 仅当 fn 返回传输层错误（*transport.CommError，连接状态不可信）时，
// 弃用旧连接、重拨并重试一次（读操作幂等）；业务/解析错误原样返回。
func (p *Pool) Do(ctx context.Context, fn func(*Client) error) error {
	c, err := p.Acquire(ctx)
	if err != nil {
		return err
	}
	err = fn(c)
	var ce *transport.CommError
	if err == nil || !errors.As(err, &ce) {
		p.Release(c)
		return err
	}
	// 传输层错误：连接不可信，关闭并重拨重试一次
	host := p.clientAddr(c)
	c.Close()
	if ctx.Err() != nil { // 调用方已取消，不值得重试
		p.putSlot(&slot{host: host})
		return err
	}
	nc, derr := p.dial(host)
	if derr != nil {
		p.putSlot(&slot{host: host})
		return err
	}
	err = fn(nc)
	if err != nil && errors.As(err, &ce) {
		nc.Close()
		p.putSlot(&slot{host: host})
		return err
	}
	p.Release(nc)
	return err
}

// dial 建一条连接：先连偏好服务器，失败则依次尝试其余候选（槽级故障转移）；
// 全部失败再重新 ping 探测一台并加入候选。
func (p *Pool) dial(prefer string) (*Client, error) {
	p.mu.Lock()
	hosts := append([]string(nil), p.hosts...)
	p.mu.Unlock()
	ordered := make([]string, 0, len(hosts)+1)
	if prefer != "" {
		ordered = append(ordered, prefer)
	}
	for _, h := range hosts {
		if h != prefer {
			ordered = append(ordered, h)
		}
	}
	var lastErr error
	for _, h := range ordered {
		c, err := NewWithTimeout(h, p.timeout)
		if err == nil {
			return c, nil
		}
		lastErr = err
	}
	if nh := transport.BestHost(nil, transport.DefaultPort, 3*time.Second); nh != "" {
		if c, err := NewWithTimeout(nh, p.timeout); err == nil {
			p.mu.Lock()
			p.hosts = append(p.hosts, nh)
			p.mu.Unlock()
			return c, nil
		}
	}
	return nil, fmt.Errorf("所有候选 TDX 服务器均不可达: %w", lastErr)
}

// putSlot 归还槽位。池已关闭时直接关连接，不向 channel 发送，
// 从而消除旧实现 Release 与 Close 之间 send-on-closed-channel 的竞态。
func (p *Pool) putSlot(s *slot) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		if s.c != nil {
			s.c.Close()
		}
		return
	}
	select {
	case p.slots <- s:
	default: // 容量恒定等于槽位总数，理论不可达；防御性关闭
		if s.c != nil {
			s.c.Close()
		}
	}
}

// clientAddr 重建拨号地址：非默认端口时带上端口（faketdx / 自定义部署）。
func (p *Pool) clientAddr(c *Client) string {
	if c.conn.Port != transport.DefaultPort {
		return net.JoinHostPort(c.conn.Host, fmt.Sprintf("%d", c.conn.Port))
	}
	return c.conn.Host
}

// Close 关闭池：停 keepalive、标记关闭并排空关闭现有连接。
// channel 本身不 close，晚归还的连接由 putSlot 直接关闭。
func (p *Pool) Close() {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return
	}
	p.closed = true
	close(p.stopKA)
	p.mu.Unlock()
	for {
		select {
		case s := <-p.slots:
			if s.c != nil {
				s.c.Close()
			}
		default:
			return
		}
	}
}
```

注意：`keepaliveLoop` 在 Task 5 实现；本任务先放一个空实现避免编译错误：

```go
// keepaliveLoop 见 Task 5；占位以便 keepalive 选项先行编译。
func (p *Pool) keepaliveLoop() {}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `go test . -run TestPool -v -race`
Expected: 5 个 TestPool* 全 PASS

- [ ] **Step 5: 全量回归 + 提交**

Run: `go test ./... -race`
Expected: 全部 PASS

```bash
git add pool.go pool_test.go
git commit -m "feat: 连接池槽位化——懒建连、错误分型重试、Close 竞态修复、并行拨号"
```

---

### Task 4: 多主机分摊

**Files:**
- Modify: `pool.go`（已在 Task 3 完成 hosts 机制，本任务验证行为并补文档）
- Test: `pool_multihost_test.go`（新建）

**Interfaces:**
- Consumes: Task 3 的 `NewPoolWith` + `WithHosts`、faketdx `Accepted()`

- [ ] **Step 1: 写失败测试**

```go
// pool_multihost_test.go
package tdx

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/NotHimmel/go-tdx/internal/faketdx"
	"github.com/NotHimmel/go-tdx/model"
)

// WithHosts 双服务器 → 槽位轮转，两台各分到连接。
func TestPoolMultiHostDistribution(t *testing.T) {
	h := func(req []byte) faketdx.Response {
		return faketdx.Response{Body: []byte{0x07, 0x00}}
	}
	srvA := faketdx.Start(t, h)
	srvB := faketdx.Start(t, h)
	p, err := NewPoolWith("", 4, 2*time.Second, WithHosts([]string{srvA.Addr(), srvB.Addr()}))
	if err != nil {
		t.Fatalf("NewPoolWith: %v", err)
	}
	defer p.Close()
	if srvA.Accepted() != 2 || srvB.Accepted() != 2 {
		t.Fatalf("期望 2/2 轮转分配，得到 A=%d B=%d", srvA.Accepted(), srvB.Accepted())
	}
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := p.Do(context.Background(), func(c *Client) error {
				_, e := c.SecurityCount(model.MarketSH)
				return e
			}); err != nil {
				t.Errorf("Do: %v", err)
			}
		}()
	}
	wg.Wait()
}

// 部分主机不可达：只要有一台可用，NewPoolWith 成功且 Do 可用。
func TestPoolPartialHostFailure(t *testing.T) {
	srv := faketdx.Start(t, func(req []byte) faketdx.Response {
		return faketdx.Response{Body: []byte{0x07, 0x00}}
	})
	p, err := NewPoolWith("", 4, 1*time.Second, WithHosts([]string{srv.Addr(), "127.0.0.1:1"}))
	if err != nil {
		t.Fatalf("一台可达即应成功: %v", err)
	}
	defer p.Close()
	for i := 0; i < 8; i++ {
		if err := p.Do(context.Background(), func(c *Client) error {
			_, e := c.SecurityCount(model.MarketSH)
			return e
		}); err != nil {
			t.Fatalf("Do 第 %d 次: %v", i, err)
		}
	}
}

// 全部主机不可达：NewPool 报错。
func TestPoolAllHostsFail(t *testing.T) {
	_, err := NewPool("127.0.0.1:1", 2, 500*time.Millisecond)
	if err == nil {
		t.Fatal("全部不可达应报错")
	}
}
```

- [ ] **Step 2: 跑测试**

Run: `go test . -run "TestPoolMultiHost|TestPoolPartial|TestPoolAllHosts" -v -race`
Expected: Task 3 实现正确则直接 PASS。TestPoolAllHostsFail 可能慢（dial 失败后走 BestHost 真机探测 3s）——若超过 5s，给 dial 加短路：候选全失败且 `len(hosts)` 中含显式端口（测试场景）时跳过 BestHost。实现见 Step 3。

- [ ] **Step 3: （仅当 Step 2 发现 BestHost 拖慢失败路径）dial 短路**

`NewPool 全部失败` 路径发生在 NewPoolWith 内（并行 NewWithTimeout，不经 dial），不触发 BestHost，无需改动。若 TestPoolLazyRecovery 或其他测试因 dial → BestHost 变慢（fake server 场景 prefer host 总是可达，正常不触发），在 `dial` 的 BestHost 分支前加：

```go
	// 显式候选全部失败时，若调用方 pin 了端口（测试/私有部署场景），
	// 不再盲目探测公共服务器列表。
	if _, _, err := net.SplitHostPort(prefer); err == nil {
		return nil, fmt.Errorf("所有候选 TDX 服务器均不可达: %w", lastErr)
	}
```

- [ ] **Step 4: 全量回归 + 提交**

Run: `go test ./... -race`
Expected: 全部 PASS

```bash
git add pool.go pool_multihost_test.go
git commit -m "feat: 连接池多主机轮转分摊 + 部分失败容忍测试"
```

---

### Task 5: Keepalive 选项

**Files:**
- Modify: `pool.go`（WithKeepalive + keepaliveLoop/pingIdle 实现）
- Test: `pool_keepalive_test.go`（新建）

**Interfaces:**
- Consumes: Task 3 的 slot.lastUse、putSlot、stopKA
- Produces: `WithKeepalive(interval time.Duration) PoolOption`

- [ ] **Step 1: 写失败测试**

```go
// pool_keepalive_test.go
package tdx

import (
	"testing"
	"time"

	"github.com/NotHimmel/go-tdx/internal/faketdx"
)

// 开启 keepalive 后，空闲连接周期性收到心跳请求（SecurityCount）。
func TestPoolKeepalive(t *testing.T) {
	srv := faketdx.Start(t, func(req []byte) faketdx.Response {
		return faketdx.Response{Body: []byte{0x01, 0x00}}
	})
	p, err := NewPoolWith(srv.Addr(), 2, 2*time.Second, WithKeepalive(50*time.Millisecond))
	if err != nil {
		t.Fatalf("NewPoolWith: %v", err)
	}
	defer p.Close()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if srv.Requests() >= 2 { // 两条空闲连接各至少 1 次心跳
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("2s 内未观察到心跳，收到数据请求 %d 条", srv.Requests())
}

// 心跳失败的连接被置空槽位，后续 Do 懒重建成功。
func TestPoolKeepaliveEvictsDead(t *testing.T) {
	var drop bool
	srv := faketdx.Start(t, func(req []byte) faketdx.Response {
		if drop {
			return faketdx.Response{CloseConn: true}
		}
		return faketdx.Response{Body: []byte{0x01, 0x00}}
	})
	p, err := NewPoolWith(srv.Addr(), 1, 2*time.Second, WithKeepalive(50*time.Millisecond))
	if err != nil {
		t.Fatalf("NewPoolWith: %v", err)
	}
	defer p.Close()
	drop = true
	time.Sleep(200 * time.Millisecond) // 让心跳踩到死连接并驱逐
	drop = false
	err = p.Do(t.Context(), func(c *Client) error {
		_, e := c.SecurityCount(1)
		return e
	})
	if err != nil {
		t.Fatalf("驱逐后 Do 应懒重建成功: %v", err)
	}
}
```

注意：`drop` 无锁读写在本测试中数据竞争良性但 -race 会报——用 `sync/atomic.Bool` 替代：声明 `var drop atomic.Bool`，读 `drop.Load()`，写 `drop.Store(true/false)`。

- [ ] **Step 2: 跑测试确认失败**

Run: `go test . -run TestPoolKeepalive -v`
Expected: FAIL，`undefined: WithKeepalive`

- [ ] **Step 3: 实现 keepalive**

`pool.go` 增加选项，并替换 Task 3 的空 keepaliveLoop：

```go
// WithKeepalive 开启空闲心跳：每 interval 对空闲超过 interval 的连接发
// 轻量命令（SecurityCount），失败连接立即驱逐为空槽位。用于对冲 TDX
// 服务器掐空闲连接导致的「首个请求必失败 + 重试」延迟税。默认关闭。
func WithKeepalive(interval time.Duration) PoolOption {
	return func(p *Pool) { p.keepalive = interval }
}

func (p *Pool) keepaliveLoop() {
	t := time.NewTicker(p.keepalive)
	defer t.Stop()
	for {
		select {
		case <-p.stopKA:
			return
		case <-t.C:
			p.pingIdle()
		}
	}
}

// pingIdle 非阻塞轮询池内空闲槽位：足够空闲的连接发一次心跳，
// 失败即驱逐（置空槽位，懒重建）。正在被业务使用的连接不在池内，天然跳过。
func (p *Pool) pingIdle() {
	for i := 0; i < cap(p.slots); i++ {
		select {
		case s := <-p.slots:
			if s.c == nil || time.Since(s.lastUse) < p.keepalive {
				p.putSlot(s)
				continue
			}
			if _, err := s.c.SecurityCount(model.MarketSH); err != nil {
				s.c.Close()
				p.putSlot(&slot{host: s.host})
			} else {
				s.lastUse = time.Now()
				p.putSlot(s)
			}
		default:
			return
		}
	}
}
```

`pool.go` imports 增加 `"github.com/NotHimmel/go-tdx/model"`。

- [ ] **Step 4: 跑测试确认通过 + 全量回归**

Run: `go test . -run TestPoolKeepalive -v -race && go test ./... -race`
Expected: 全部 PASS

- [ ] **Step 5: 提交**

```bash
git add pool.go pool_keepalive_test.go
git commit -m "feat: 连接池可选 keepalive——空闲心跳 + 死连接驱逐"
```

---

### Task 6: 全量验证 + 文档同步

**Files:**
- Modify: `README.md`（Pool 段落补多主机/keepalive 说明，如已有 Pool 文档）
- Modify: `pool.go`（如验证发现问题）

- [ ] **Step 1: 全量测试矩阵**

Run:
```bash
cd /mnt/data/tz/snap/rust/go-tdx
go vet ./...
go test ./... -race -count=2
go build ./...
```
Expected: vet 无告警；测试全 PASS（-count=2 排除顺序依赖）；构建成功。

- [ ] **Step 2: 消费方兼容性验证**

shining_trader 依赖 v0.2.1 发布版（无 replace），签名未变即兼容。做一次临时 replace 编译验证后还原：

```bash
cd /mnt/data/tz/snap/rust/shining_trader
go mod edit -replace github.com/NotHimmel/go-tdx=../go-tdx
go build ./... && echo COMPAT-OK
go mod edit -dropreplace github.com/NotHimmel/go-tdx
git checkout go.mod 2>/dev/null || true
```
Expected: `COMPAT-OK`

- [ ] **Step 3: README 更新（若有 Pool 段落）**

在 README 的 Pool 说明处补充（措辞按现有文风调整）：

```markdown
- `NewPool("", size, timeout)`：自动 ping 候选服务器，取延迟最低的至多 3 台轮转分摊，聚合吞吐随服务器数提升。
- `NewPoolWith(host, size, timeout, tdx.WithKeepalive(60*time.Second))`：空闲心跳，对冲服务器掐空闲连接。
- `Do` 仅在传输层错误（连接不可信）时重拨重试一次；业务/解析错误原样返回。
```

- [ ] **Step 4: 提交收尾**

```bash
git add README.md
git commit -m "docs: README 补充连接池多主机与 keepalive 说明"
```

---

## Self-Review 记录

- Spec 覆盖：槽位化（Task 3）、多主机（Task 3 实现 + Task 4 验证）、错误分型（Task 1 + Task 3）、Close 竞态（Task 3）、keepalive（Task 5）、并行拨号（Task 3）、API 兼容（Global Constraints + Task 6 Step 2）、测试策略六用例（Task 3/4/5 覆盖 1-6）。无缺口。
- 类型一致性：`slot{c,host,lastUse}`、`putSlot`、`clientAddr`、`NewPoolWith`、`WithHosts`、`WithKeepalive` 在各任务间签名一致；`keepaliveLoop` Task 3 占位 → Task 5 实现。
- 已知取舍：faketdx「一次 Read = 一条请求」假设仅在 loopback 单写场景成立，已注释说明。
