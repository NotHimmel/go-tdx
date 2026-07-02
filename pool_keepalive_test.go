package tdx

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/NotHimmel/go-tdx/internal/faketdx"
	"github.com/NotHimmel/go-tdx/model"
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
	var drop atomic.Bool
	srv := faketdx.Start(t, func(req []byte) faketdx.Response {
		if drop.Load() {
			return faketdx.Response{CloseConn: true}
		}
		return faketdx.Response{Body: []byte{0x01, 0x00}}
	})
	p, err := NewPoolWith(srv.Addr(), 1, 2*time.Second, WithKeepalive(50*time.Millisecond))
	if err != nil {
		t.Fatalf("NewPoolWith: %v", err)
	}
	defer p.Close()
	drop.Store(true)
	time.Sleep(200 * time.Millisecond) // 让心跳踩到死连接并驱逐
	drop.Store(false)
	err = p.Do(context.Background(), func(c *Client) error {
		_, e := c.SecurityCount(model.MarketSH)
		return e
	})
	if err != nil {
		t.Fatalf("驱逐后 Do 应懒重建成功: %v", err)
	}
}
