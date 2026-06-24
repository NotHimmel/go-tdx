package command

import (
	"encoding/binary"
	"fmt"

	"github.com/NotHimmel/go-tdx/codec"
	"github.com/NotHimmel/go-tdx/model"
)

// ── 分时 ──

// MinuteTimeCmd 获取今日分时（240 条）。
type MinuteTimeCmd struct {
	Market model.Market
	Code   string
}

func (c MinuteTimeCmd) BuildRequest() []byte {
	buf := hexBytes("0c1b080001010e000e001d05")
	buf = binary.LittleEndian.AppendUint16(buf, uint16(c.Market))
	cb := code6(c.Code)
	buf = append(buf, cb[:]...)
	return binary.LittleEndian.AppendUint32(buf, 0)
}

func (c MinuteTimeCmd) ParseResponse(body []byte) ([]model.MinuteBar, error) {
	return parseMinuteBody(body, 4)
}

// HistoryMinuteTimeCmd 获取历史某日分时（date 格式 YYYYMMDD）。
type HistoryMinuteTimeCmd struct {
	Market model.Market
	Code   string
	Date   uint32
}

func (c HistoryMinuteTimeCmd) BuildRequest() []byte {
	buf := hexBytes("0c0130000101" + "0d000d00b40f")
	buf = binary.LittleEndian.AppendUint32(buf, c.Date)
	buf = append(buf, byte(c.Market))
	cb := code6(c.Code)
	return append(buf, cb[:]...)
}

func (c HistoryMinuteTimeCmd) ParseResponse(body []byte) ([]model.MinuteBar, error) {
	return parseMinuteBody(body, 6)
}

func parseMinuteBody(body []byte, skip int) ([]model.MinuteBar, error) {
	if len(body) < 2 {
		return nil, fmt.Errorf("minute_time header: 数据不足")
	}
	num := int(binary.LittleEndian.Uint16(body))
	pos := skip
	lastPrice := 0
	bars := make([]model.MinuteBar, 0, num)
	for i := 0; i < num; i++ {
		priceDiff, np, err := codec.GetPrice(body, pos)
		if err != nil {
			return nil, err
		}
		pos = np
		unknown1, np2, err := codec.GetPrice(body, pos)
		if err != nil {
			return nil, err
		}
		pos = np2
		vol, np3, err := codec.GetPrice(body, pos)
		if err != nil {
			return nil, err
		}
		pos = np3
		lastPrice += priceDiff
		bars = append(bars, model.MinuteBar{
			Price:    float64(lastPrice) / 100.0,
			Vol:      vol,
			Unknown1: unknown1,
		})
	}
	return bars, nil
}

// ── 逐笔成交 ──

// TransactionCmd 获取当日逐笔成交（分页，每次最多 800 条）。
type TransactionCmd struct {
	Market model.Market
	Code   string
	Start  uint16
	Count  uint16
}

func (c TransactionCmd) BuildRequest() []byte {
	buf := hexBytes("0c170801010" + "10e000e00c50f")
	buf = binary.LittleEndian.AppendUint16(buf, uint16(c.Market))
	cb := code6(c.Code)
	buf = append(buf, cb[:]...)
	buf = binary.LittleEndian.AppendUint16(buf, c.Start)
	return binary.LittleEndian.AppendUint16(buf, c.Count)
}

func (c TransactionCmd) ParseResponse(body []byte) ([]model.TransactionRecord, error) {
	return parseTransactionBody(body, 2, true)
}

// HistoryTransactionCmd 获取历史某日逐笔成交（date 格式 YYYYMMDD，分页）。
type HistoryTransactionCmd struct {
	Market model.Market
	Code   string
	Date   uint32
	Start  uint16
	Count  uint16
}

func (c HistoryTransactionCmd) BuildRequest() []byte {
	buf := hexBytes("0c013001000112001200b50f")
	buf = binary.LittleEndian.AppendUint32(buf, c.Date)
	buf = binary.LittleEndian.AppendUint16(buf, uint16(c.Market))
	cb := code6(c.Code)
	buf = append(buf, cb[:]...)
	buf = binary.LittleEndian.AppendUint16(buf, c.Start)
	return binary.LittleEndian.AppendUint16(buf, c.Count)
}

func (c HistoryTransactionCmd) ParseResponse(body []byte) ([]model.TransactionRecord, error) {
	return parseTransactionBody(body, 6, false)
}

// hasNumOrders=true 时当日逐笔含「成交笔数」字段。
func parseTransactionBody(body []byte, skip int, hasNumOrders bool) ([]model.TransactionRecord, error) {
	if len(body) < 2 {
		return nil, fmt.Errorf("transaction header: 数据不足")
	}
	num := int(binary.LittleEndian.Uint16(body))
	pos := skip
	lastPrice := 0
	recs := make([]model.TransactionRecord, 0, num)
	for i := 0; i < num; i++ {
		hour, minute, np, err := codec.GetTime(body, pos)
		if err != nil {
			return nil, err
		}
		pos = np
		gp := func() (int, error) {
			v, n, e := codec.GetPrice(body, pos)
			pos = n
			return v, e
		}
		priceDiff, e1 := gp()
		vol, e2 := gp()
		if e1 != nil || e2 != nil {
			return nil, fmt.Errorf("transaction 解析失败，偏移 %d", pos)
		}
		if hasNumOrders {
			if _, err = gp(); err != nil { // num_orders
				return nil, err
			}
		}
		buyorsell, e3 := gp()
		unknownLast, e4 := gp()
		if e3 != nil || e4 != nil {
			return nil, fmt.Errorf("transaction 解析失败，偏移 %d", pos)
		}
		lastPrice += priceDiff
		recs = append(recs, model.TransactionRecord{
			Hour:        hour,
			Minute:      minute,
			Price:       float64(lastPrice) / 100.0,
			Vol:         vol,
			BuyOrSell:   buyorsell,
			UnknownLast: unknownLast,
		})
	}
	return recs, nil
}
