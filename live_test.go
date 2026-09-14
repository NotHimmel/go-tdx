package tdx

import (
	"os"
	"testing"
	"time"

	"github.com/NotHimmel/go-tdx/model"
)

// TestLiveHello1SecurityBars 对真实公共服务器执行 Hello1 后拉取贵州茅台日 K。
// 默认测试已验证节点；可用 TDX_LIVE_HOST 覆盖。公网集成测试需显式启用，避免
// 普通单测依赖外部服务。
func TestLiveHello1SecurityBars(t *testing.T) {
	if os.Getenv("TDX_LIVE") != "1" {
		t.Skip("设置 TDX_LIVE=1 运行真实行情握手回归测试")
	}
	host := os.Getenv("TDX_LIVE_HOST")
	if host == "" {
		host = "110.41.147.114"
	}
	c, err := NewWithTimeout(host, 8*time.Second)
	if err != nil {
		t.Fatalf("Hello1 连接 %s: %v", host, err)
	}
	defer c.Close()
	bars, err := c.SecurityBars(model.MarketSH, "600519", model.Day, 0, 5)
	if err != nil {
		t.Fatalf("Hello1 后拉取 600519 日 K: %v", err)
	}
	if len(bars) == 0 {
		t.Fatal("Hello1 后拉取 600519 日 K 返回空数据")
	}
	last := bars[len(bars)-1]
	if last.Year < 2020 || last.Month < 1 || last.Month > 12 || last.Day < 1 || last.Day > 31 ||
		last.Close <= 0 || last.High < last.Low {
		t.Fatalf("末根日 K 异常: %+v", last)
	}
}
