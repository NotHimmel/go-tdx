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
