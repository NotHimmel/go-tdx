package tdx

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/NotHimmel/go-tdx/model"
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

// WithKeepalive 开启空闲心跳：每 interval 对空闲超过 interval 的连接发
// 轻量命令（SecurityCount），失败连接立即驱逐为空槽位。用于对冲 TDX
// 服务器掐空闲连接导致的「首个请求必失败 + 重试」延迟税。默认关闭。
func WithKeepalive(interval time.Duration) PoolOption {
	return func(p *Pool) { p.keepalive = interval }
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
	// 调用方 pin 了端口（测试/私有部署场景）时不盲目探测公共服务器列表
	if _, _, err := net.SplitHostPort(prefer); err == nil {
		return nil, fmt.Errorf("所有候选 TDX 服务器均不可达: %w", lastErr)
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

// keepaliveLoop 周期触发空闲心跳，Close 时经 stopKA 退出。
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
