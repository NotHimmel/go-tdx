package tdx

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/NotHimmel/go-tdx/transport"
)

// Pool 是 Client 连接池：多 goroutine（如 web 多请求）共享有限连接。
// 单条 TDX 连接内命令串行；池让并发请求分散到多条连接，并对 TDX 服务器限流。
type Pool struct {
	host    string
	timeout time.Duration
	conns   chan *Client

	mu     sync.Mutex
	closed bool
}

// NewPool 创建 size 条连接的池。host 为空自动选最优（所有连接连同一台）。
func NewPool(host string, size int, timeout time.Duration) (*Pool, error) {
	if size <= 0 {
		size = 4
	}
	if host == "" {
		probe, err := New("")
		if err != nil {
			return nil, err
		}
		// 复用探测得到的连接，host 由其底层确定
		host = probe.conn.Host
		probe.Close()
	}
	p := &Pool{host: host, timeout: timeout, conns: make(chan *Client, size)}
	for i := 0; i < size; i++ {
		c, err := NewWithTimeout(host, timeout)
		if err != nil {
			p.Close()
			return nil, fmt.Errorf("连接池初始化失败（第 %d 条）: %w", i+1, err)
		}
		p.conns <- c
	}
	return p, nil
}

// Acquire 取一条连接（阻塞直到可用或 ctx 取消）。用完必须 Release。
func (p *Pool) Acquire(ctx context.Context) (*Client, error) {
	select {
	case c := <-p.conns:
		return c, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Release 归还连接。
func (p *Pool) Release(c *Client) {
	p.mu.Lock()
	closed := p.closed
	p.mu.Unlock()
	if closed {
		c.Close()
		return
	}
	select {
	case p.conns <- c:
	default:
		c.Close() // 池满（不应发生）则直接关
	}
}

// Do 取连接执行 fn 并自动归还。web handler 的推荐入口。
// fn 出错时认为连接可能失效，重连（必要时切换服务器）并重试一次，
// 避免 TDX 丢弃空闲连接/网络抖动后池内死连接导致持续「拉不到数据」。
func (p *Pool) Do(ctx context.Context, fn func(*Client) error) error {
	c, err := p.Acquire(ctx)
	if err != nil {
		return err
	}
	err = fn(c)
	if err != nil {
		if nc, e := p.reconnect(); e == nil {
			c.Close()
			c = nc
			err = fn(c) // 用新连接重试一次（读操作幂等）
		}
	}
	p.Release(c)
	return err
}

// reconnect 重建一条连接：先连原服务器，失败则重选最优服务器（故障转移）。
func (p *Pool) reconnect() (*Client, error) {
	p.mu.Lock()
	host := p.host
	p.mu.Unlock()
	if c, err := NewWithTimeout(host, p.timeout); err == nil {
		return c, nil
	}
	nh := transport.BestHost(nil, transport.DefaultPort, 3*time.Second)
	if nh == "" {
		return nil, fmt.Errorf("无可达 TDX 服务器")
	}
	p.mu.Lock()
	p.host = nh
	p.mu.Unlock()
	return NewWithTimeout(nh, p.timeout)
}

// Close 关闭池内所有连接。
func (p *Pool) Close() {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return
	}
	p.closed = true
	p.mu.Unlock()
	close(p.conns)
	for c := range p.conns {
		c.Close()
	}
}
