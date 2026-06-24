// Command tdx 是 go-tdx 的最小演示 CLI：选最优服务器，拉一支股票的日 K。
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/NotHimmel/go-tdx/command"
	"github.com/NotHimmel/go-tdx/model"
	"github.com/NotHimmel/go-tdx/transport"
)

func main() {
	market := flag.Int("market", int(model.MarketSZ), "市场 0=SZ 1=SH 2=BJ")
	code := flag.String("code", "000001", "6 位股票代码")
	count := flag.Int("count", 10, "K 线条数")
	host := flag.String("host", "", "指定服务器（空=自动选最优）")
	flag.Parse()

	h := *host
	if h == "" {
		fmt.Fprintln(os.Stderr, "ping 候选服务器中…")
		h = transport.BestHost(nil, transport.DefaultPort, 3*time.Second)
		if h == "" {
			fmt.Fprintln(os.Stderr, "无可达服务器")
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "最优服务器: %s\n", h)
	}

	conn := transport.NewConn(h, transport.DefaultPort, 8*time.Second)
	if err := conn.Connect(); err != nil {
		fmt.Fprintln(os.Stderr, "连接失败:", err)
		os.Exit(1)
	}
	defer conn.Close()

	cmd := command.SecurityBarsCmd{
		Market:   model.Market(*market),
		Code:     *code,
		Category: model.Day,
		Start:    0,
		Count:    uint16(*count),
	}
	body, err := conn.Execute(cmd.BuildRequest())
	if err != nil {
		fmt.Fprintln(os.Stderr, "请求失败:", err)
		os.Exit(1)
	}
	bars, err := cmd.ParseResponse(body)
	if err != nil {
		fmt.Fprintln(os.Stderr, "解析失败:", err)
		os.Exit(1)
	}
	for _, b := range bars {
		fmt.Printf("%s  O=%.2f H=%.2f L=%.2f C=%.2f V=%.0f\n",
			b.DatetimeStr(), b.Open, b.High, b.Low, b.Close, b.Vol)
	}
}
