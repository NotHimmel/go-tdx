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
