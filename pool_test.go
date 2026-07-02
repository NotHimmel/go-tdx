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
	// 恢复期：懒建连应让 Do 成功
	err = p.Do(context.Background(), func(c *Client) error {
		_, e := c.SecurityCount(model.MarketSH)
		return e
	})
	if err != nil {
		t.Fatalf("故障恢复后 Do 应成功，得到 %v", err)
	}
}
