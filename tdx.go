// Package tdx 是 go-tdx 的高层门面：屏蔽 conn + command 的手工拼装，
// 供 web 服务等消费方直接调用。
//
//	c, err := tdx.New("")   // 空 host = 自动 ping 选最优服务器
//	defer c.Close()
//	bars, err := c.SecurityBars(model.MarketSZ, "000001", model.Day, 0, 100)
package tdx

import (
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/NotHimmel/go-tdx/codec"
	"github.com/NotHimmel/go-tdx/command"
	"github.com/NotHimmel/go-tdx/model"
	"github.com/NotHimmel/go-tdx/transport"
)

// Client 是面向消费方的高层客户端。非并发安全；多 goroutine 请用连接池（见 Pool）。
type Client struct {
	conn *transport.Conn
}

// New 连接到 host 并完成握手。host 为空时自动 ping 候选列表选最优。
func New(host string) (*Client, error) {
	return NewWithTimeout(host, 8*time.Second)
}

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

// Close 关闭底层连接。
func (c *Client) Close() error { return c.conn.Close() }

// SecurityBars 获取股票 K 线。
func (c *Client) SecurityBars(m model.Market, code string, cat model.KlineCategory, start, count uint16) ([]model.SecurityBar, error) {
	cmd := command.SecurityBarsCmd{Market: m, Code: code, Category: cat, Start: start, Count: count}
	body, err := c.conn.Execute(cmd.BuildRequest())
	if err != nil {
		return nil, err
	}
	return cmd.ParseResponse(body)
}

// IndexBars 获取指数 K 线（记录多 4 字节涨跌家数）。
func (c *Client) IndexBars(m model.Market, code string, cat model.KlineCategory, start, count uint16) ([]model.SecurityBar, error) {
	cmd := command.SecurityBarsCmd{Market: m, Code: code, Category: cat, Start: start, Count: count, IsIndex: true}
	body, err := c.conn.Execute(cmd.BuildRequest())
	if err != nil {
		return nil, err
	}
	return cmd.ParseResponse(body)
}

// exec 是所有命令的统一执行壳。
func (c *Client) exec(req []byte) ([]byte, error) { return c.conn.Execute(req) }

// SecurityCount 返回市场证券总数。
func (c *Client) SecurityCount(m model.Market) (int, error) {
	cmd := command.SecurityCountCmd{Market: m}
	body, err := c.exec(cmd.BuildRequest())
	if err != nil {
		return 0, err
	}
	return cmd.ParseResponse(body)
}

// SecurityList 获取从 start 开始的一页证券列表（每页最多 1000 条）。
func (c *Client) SecurityList(m model.Market, start uint16) ([]model.SecurityInfo, error) {
	cmd := command.SecurityListCmd{Market: m, Start: start}
	body, err := c.exec(cmd.BuildRequest())
	if err != nil {
		return nil, err
	}
	return cmd.ParseResponse(body)
}

// SecurityListAll 翻页拉取某市场全部证券列表。
func (c *Client) SecurityListAll(m model.Market) ([]model.SecurityInfo, error) {
	var all []model.SecurityInfo
	for start := uint16(0); ; start += 1000 {
		page, err := c.SecurityList(m, start)
		if err != nil {
			return all, err
		}
		all = append(all, page...)
		if len(page) < 1000 {
			break
		}
	}
	return all, nil
}

// SecurityQuotes 批量获取实时五档行情（最多 80 只）。
func (c *Client) SecurityQuotes(stocks []command.Stock) ([]model.SecurityQuote, error) {
	cmd := command.SecurityQuotesCmd{Stocks: stocks}
	body, err := c.exec(cmd.BuildRequest())
	if err != nil {
		return nil, err
	}
	return cmd.ParseResponse(body)
}

// PriceLimits 计算涨跌停价。上市初期无限制标的会用日 K 估算上市天数。
func (c *Client) PriceLimits(m model.Market, code, name string, preClose float64) (up, down *float64, err error) {
	listedDays := 0
	if window := codec.GetNoLimitWindowDays(m, code, name); window > 0 {
		bars, e := c.SecurityBars(m, code, model.Day, 0, uint16(window+1))
		if e == nil {
			listedDays = len(bars)
		}
	}
	up, down = codec.ComputePriceLimits(m, code, name, preClose, listedDays)
	return up, down, nil
}

// MinuteTime 获取今日分时（240 条）。
func (c *Client) MinuteTime(m model.Market, code string) ([]model.MinuteBar, error) {
	cmd := command.MinuteTimeCmd{Market: m, Code: code}
	body, err := c.exec(cmd.BuildRequest())
	if err != nil {
		return nil, err
	}
	return cmd.ParseResponse(body)
}

// HistoryMinuteTime 获取历史某日分时（date 格式 YYYYMMDD）。
func (c *Client) HistoryMinuteTime(m model.Market, code string, date uint32) ([]model.MinuteBar, error) {
	cmd := command.HistoryMinuteTimeCmd{Market: m, Code: code, Date: date}
	body, err := c.exec(cmd.BuildRequest())
	if err != nil {
		return nil, err
	}
	return cmd.ParseResponse(body)
}

// Transaction 获取当日逐笔成交（分页）。
func (c *Client) Transaction(m model.Market, code string, start, count uint16) ([]model.TransactionRecord, error) {
	cmd := command.TransactionCmd{Market: m, Code: code, Start: start, Count: count}
	body, err := c.exec(cmd.BuildRequest())
	if err != nil {
		return nil, err
	}
	return cmd.ParseResponse(body)
}

// HistoryTransaction 获取历史某日逐笔成交（分页）。
func (c *Client) HistoryTransaction(m model.Market, code string, date uint32, start, count uint16) ([]model.TransactionRecord, error) {
	cmd := command.HistoryTransactionCmd{Market: m, Code: code, Date: date, Start: start, Count: count}
	body, err := c.exec(cmd.BuildRequest())
	if err != nil {
		return nil, err
	}
	return cmd.ParseResponse(body)
}

// FinanceInfo 获取最新财务数据。
func (c *Client) FinanceInfo(m model.Market, code string) (model.FinanceInfo, error) {
	cmd := command.FinanceInfoCmd{Market: m, Code: code}
	body, err := c.exec(cmd.BuildRequest())
	if err != nil {
		return model.FinanceInfo{}, err
	}
	return cmd.ParseResponse(body)
}

// XdxrInfo 获取除权除息记录。
func (c *Client) XdxrInfo(m model.Market, code string) ([]model.XdxrRecord, error) {
	cmd := command.XdxrInfoCmd{Market: m, Code: code}
	body, err := c.exec(cmd.BuildRequest())
	if err != nil {
		return nil, err
	}
	return cmd.ParseResponse(body)
}

// CompanyInfoCategory 获取公司信息文件目录。
func (c *Client) CompanyInfoCategory(m model.Market, code string) ([]model.CompanyInfoCategory, error) {
	cmd := command.CompanyInfoCategoryCmd{Market: m, Code: code}
	body, err := c.exec(cmd.BuildRequest())
	if err != nil {
		return nil, err
	}
	return cmd.ParseResponse(body)
}

// CompanyInfoContent 读取公司信息文本（GBK→UTF-8）。
func (c *Client) CompanyInfoContent(m model.Market, code, filename string, offset, length uint32) (string, error) {
	cmd := command.CompanyInfoContentCmd{Market: m, Code: code, Filename: filename, Offset: offset, Length: length}
	body, err := c.exec(cmd.BuildRequest())
	if err != nil {
		return "", err
	}
	return cmd.ParseResponse(body)
}

// HistoryFundFlow 获取历史日线资金流向序列（Category 22 直连）。
func (c *Client) HistoryFundFlow(m model.Market, code string, start, count uint16) ([]model.HistoricalFundFlow, error) {
	cmd := command.HistoryFundFlowCmd{Market: m, Code: code, Start: start, Count: count}
	body, err := c.exec(cmd.BuildRequest())
	if err != nil {
		return nil, err
	}
	return cmd.ParseResponse(body)
}

// BlockInfoMeta 获取板块文件元数据。
func (c *Client) BlockInfoMeta(filename string) (size uint32, hash string, err error) {
	cmd := command.BlockInfoMetaCmd{Filename: filename}
	body, e := c.exec(cmd.BuildRequest())
	if e != nil {
		return 0, "", e
	}
	return cmd.ParseResponse(body)
}

// BlockInfoFile 完整下载板块文件（自动分片）。
func (c *Client) BlockInfoFile(filename string) ([]byte, error) {
	size, _, err := c.BlockInfoMeta(filename)
	if err != nil {
		return nil, err
	}
	const chunk = 0x7530 // 30000
	var out []byte
	for off := uint32(0); off < size; off += chunk {
		n := uint32(chunk)
		if off+n > size {
			n = size - off
		}
		cmd := command.BlockInfoCmd{Filename: filename, Start: off, Length: n}
		body, e := c.exec(cmd.BuildRequest())
		if e != nil {
			return out, e
		}
		data, _ := cmd.ParseResponse(body)
		out = append(out, data...)
	}
	return out, nil
}

// BlockList 下载并解析板块文件（如 block_gn.dat 概念 / block_zs.dat 行业指数）。
func (c *Client) BlockList(filename string) ([]model.TdxBlock, error) {
	data, err := c.BlockInfoFile(filename)
	if err != nil {
		return nil, err
	}
	return codec.ParseBlockDat(data, filename), nil
}

// MarketStat 获取 A 股全市场涨跌统计概况（基于 880005/880001/880006）。
func (c *Client) MarketStat() (model.MarketStat, error) {
	quotes, err := c.SecurityQuotes([]command.Stock{
		{Market: model.MarketSH, Code: "880005"},
		{Market: model.MarketSH, Code: "880001"},
		{Market: model.MarketSH, Code: "880006"},
	})
	if err != nil {
		return model.MarketStat{}, err
	}
	if len(quotes) == 0 {
		return model.MarketStat{}, fmt.Errorf("无法获取市场统计数据")
	}
	q := quotes[0]
	up, down, neutral, total := int(q.Price), int(q.Open), int(q.Low), int(q.High)
	stat := model.MarketStat{
		UpCount:        up,
		DownCount:      down,
		NeutralCount:   neutral,
		SuspendedCount: max(0, total-up-down-neutral),
		TotalCount:     total,
		TotalAmount:    q.Amount,
		TotalVolume:    q.Vol,
	}
	if len(quotes) > 1 {
		stat.TotalMarketCap = quotes[1].Price * 1e10
	}
	if len(quotes) > 2 {
		stat.LimitDownCount = int(quotes[2].Open)
		stat.LimitUpCount = int(quotes[2].Price)
	}
	return stat, nil
}
