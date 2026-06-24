// Package command 实现各协议命令的请求构造与响应解析（无 IO）。
package command

import (
	"encoding/binary"
	"fmt"

	"github.com/NotHimmel/go-tdx/codec"
	"github.com/NotHimmel/go-tdx/model"
)

// SecurityBarsCmd 获取指定股票/指数的 K 线数据。
type SecurityBarsCmd struct {
	Market   model.Market
	Code     string
	Category model.KlineCategory
	Start    uint16
	Count    uint16
	IsIndex  bool // 指数 K 线：每条记录在 vol+amt 后多 4 字节（上涨/下跌家数）
}

// BuildRequest 构造请求包（Header 12 + Payload 28 = 40 字节）。
func (c SecurityBarsCmd) BuildRequest() []byte {
	count := c.Count
	if count == 0 {
		count = 800
	}
	code := []byte(c.Code)
	var code6 [6]byte
	copy(code6[:], code)

	buf := make([]byte, 0, 40)
	buf = le16(buf, 0x010C)
	buf = le32(buf, 0x01016408)
	buf = le16(buf, 0x001C)
	buf = le16(buf, 0x001C)
	buf = le16(buf, 0x052D)
	buf = le16(buf, uint16(c.Market))
	buf = append(buf, code6[:]...)
	buf = le16(buf, uint16(c.Category))
	buf = le16(buf, 1)
	buf = le16(buf, c.Start)
	buf = le16(buf, count)
	buf = le32(buf, 0)
	buf = le32(buf, 0)
	buf = le16(buf, 0)
	return buf
}

// ParseResponse 从解压后的 body 解析 K 线列表。
func (c SecurityBarsCmd) ParseResponse(body []byte) ([]model.SecurityBar, error) {
	if len(body) < 2 {
		return nil, fmt.Errorf("security_bars header: 数据不足")
	}
	retCount := int(binary.LittleEndian.Uint16(body))
	pos := 2
	bars := make([]model.SecurityBar, 0, retCount)
	preDiffBase := 0

	for i := 0; i < retCount; i++ {
		var (
			year, month, day, hour, minute int
			err                            error
		)
		year, month, day, hour, minute, pos, err = codec.GetDatetime(c.Category, body, pos)
		if err != nil {
			return nil, err
		}

		var openDiff, closeDiff, highDiff, lowDiff int
		if openDiff, pos, err = codec.GetPrice(body, pos); err != nil {
			return nil, err
		}
		if closeDiff, pos, err = codec.GetPrice(body, pos); err != nil {
			return nil, err
		}
		if highDiff, pos, err = codec.GetPrice(body, pos); err != nil {
			return nil, err
		}
		if lowDiff, pos, err = codec.GetPrice(body, pos); err != nil {
			return nil, err
		}

		var vol, amount float64
		if vol, pos, err = codec.GetVolume(body, pos); err != nil {
			return nil, err
		}
		if amount, pos, err = codec.GetVolume(body, pos); err != nil {
			return nil, err
		}

		// 指数记录额外 4 字节：上涨家数 + 下跌家数（各 uint16 LE）
		if c.IsIndex {
			pos += 4
		}

		// 差分还原（与 pytdx 完全一致）
		openAbs := openDiff + preDiffBase
		closeAbs := openAbs + closeDiff
		highAbs := openAbs + highDiff
		lowAbs := openAbs + lowDiff
		preDiffBase = openAbs + closeDiff

		bars = append(bars, model.SecurityBar{
			Open:   float64(openAbs) / 1000.0,
			Close:  float64(closeAbs) / 1000.0,
			High:   float64(highAbs) / 1000.0,
			Low:    float64(lowAbs) / 1000.0,
			Vol:    vol,
			Amount: amount,
			Year:   year,
			Month:  month,
			Day:    day,
			Hour:   hour,
			Minute: minute,
		})
	}
	return bars, nil
}

func le16(buf []byte, v uint16) []byte {
	return binary.LittleEndian.AppendUint16(buf, v)
}

func le32(buf []byte, v uint32) []byte {
	return binary.LittleEndian.AppendUint32(buf, v)
}
