package command

import (
	"encoding/binary"
	"fmt"

	"github.com/NotHimmel/go-tdx/codec"
	"github.com/NotHimmel/go-tdx/model"
)

// Stock 是 (市场, 代码) 二元组。
type Stock struct {
	Market model.Market
	Code   string
}

// SecurityQuotesCmd 批量获取实时五档行情（最多 80 只/次）。
type SecurityQuotesCmd struct {
	Stocks []Stock
}

func (c SecurityQuotesCmd) BuildRequest() []byte {
	n := len(c.Stocks)
	payloadLen := uint16(n*7 + 12)
	buf := make([]byte, 0, n*7+18)
	buf = binary.LittleEndian.AppendUint16(buf, 0x010C)
	buf = binary.LittleEndian.AppendUint32(buf, 0x02006320)
	buf = binary.LittleEndian.AppendUint16(buf, payloadLen)
	buf = binary.LittleEndian.AppendUint16(buf, payloadLen)
	buf = binary.LittleEndian.AppendUint32(buf, 0x0005053E)
	buf = binary.LittleEndian.AppendUint32(buf, 0)
	buf = binary.LittleEndian.AppendUint16(buf, 0)
	buf = binary.LittleEndian.AppendUint16(buf, uint16(n))
	for _, s := range c.Stocks {
		buf = append(buf, byte(s.Market))
		cb := code6(s.Code)
		buf = append(buf, cb[:]...)
	}
	return buf
}

func formatServerTime(raw int) string {
	hours, frac := raw/1_000_000, raw%1_000_000
	totalMillis := frac * 3600 / 1000
	minutes, remainder := totalMillis/60_000, totalMillis%60_000
	seconds, millis := remainder/1000, remainder%1000
	return fmt.Sprintf("%02d:%02d:%02d.%03d", hours, minutes, seconds, millis)
}

func (c SecurityQuotesCmd) ParseResponse(body []byte) ([]model.SecurityQuote, error) {
	pos := 2 // 跳过 b1 cb 魔数
	if pos+2 > len(body) {
		return nil, fmt.Errorf("security_quotes header: 数据不足")
	}
	num := int(binary.LittleEndian.Uint16(body[pos:]))
	pos += 2

	out := make([]model.SecurityQuote, 0, num)
	gp := func() (int, error) {
		v, np, err := codec.GetPrice(body, pos)
		pos = np
		return v, err
	}

	for i := 0; i < num; i++ {
		if pos+9 > len(body) {
			return nil, fmt.Errorf("security_quotes record header: 数据不足，偏移 %d", pos)
		}
		marketB := body[pos]
		codeB := codec.TrimNulString(body[pos+1 : pos+7])
		active1 := binary.LittleEndian.Uint16(body[pos+7:])
		pos += 9

		var err error
		var priceRaw, lastCloseDiff, openDiff, highDiff, lowDiff int
		var unknown0, vol, curVol int
		if priceRaw, err = gp(); err != nil {
			return nil, err
		}
		if lastCloseDiff, err = gp(); err != nil {
			return nil, err
		}
		if openDiff, err = gp(); err != nil {
			return nil, err
		}
		if highDiff, err = gp(); err != nil {
			return nil, err
		}
		if lowDiff, err = gp(); err != nil {
			return nil, err
		}
		if unknown0, err = gp(); err != nil {
			return nil, err
		}
		if _, err = gp(); err != nil { // unknown_1
			return nil, err
		}
		if vol, err = gp(); err != nil {
			return nil, err
		}
		if curVol, err = gp(); err != nil {
			return nil, err
		}

		amount, _, errA := codec.GetVolume(body, pos)
		if errA != nil {
			return nil, errA
		}
		pos += 4

		var sVol, bVol, unknown2, unknown3 int
		if sVol, err = gp(); err != nil {
			return nil, err
		}
		if bVol, err = gp(); err != nil {
			return nil, err
		}
		if unknown2, err = gp(); err != nil {
			return nil, err
		}
		if unknown3, err = gp(); err != nil {
			return nil, err
		}

		var bid, ask, bv, av [5]float64
		for lvl := 0; lvl < 5; lvl++ {
			bidD, e1 := gp()
			askD, e2 := gp()
			bvN, e3 := gp()
			avN, e4 := gp()
			if e1 != nil || e2 != nil || e3 != nil || e4 != nil {
				return nil, fmt.Errorf("security_quotes 五档解析失败")
			}
			bid[lvl] = float64(priceRaw+bidD) / 100.0
			ask[lvl] = float64(priceRaw+askD) / 100.0
			bv[lvl] = float64(bvN)
			av[lvl] = float64(avN)
		}

		if pos+2 > len(body) {
			return nil, fmt.Errorf("security_quotes tail flag: 数据不足")
		}
		tradingStatus := binary.LittleEndian.Uint16(body[pos:])
		pos += 2
		var unknown5, unknown6, unknown7, unknown8 int
		if unknown5, err = gp(); err != nil {
			return nil, err
		}
		if unknown6, err = gp(); err != nil {
			return nil, err
		}
		if unknown7, err = gp(); err != nil {
			return nil, err
		}
		if unknown8, err = gp(); err != nil {
			return nil, err
		}
		if pos+4 > len(body) {
			return nil, fmt.Errorf("security_quotes tail: 数据不足")
		}
		riseSpeedRaw := int16(binary.LittleEndian.Uint16(body[pos:]))
		active2 := binary.LittleEndian.Uint16(body[pos+2:])
		pos += 4

		if marketB > 2 {
			return nil, fmt.Errorf("security_quotes 非法 market 值: %d", marketB)
		}
		p := float64(priceRaw)
		out = append(out, model.SecurityQuote{
			Market:        model.Market(marketB),
			Code:          codeB,
			Price:         p / 100.0,
			PreClose:      float64(priceRaw+lastCloseDiff) / 100.0,
			Open:          float64(priceRaw+openDiff) / 100.0,
			High:          float64(priceRaw+highDiff) / 100.0,
			Low:           float64(priceRaw+lowDiff) / 100.0,
			Vol:           float64(vol),
			CurVol:        float64(curVol),
			Amount:        amount,
			SVol:          float64(sVol),
			BVol:          float64(bVol),
			Active1:       active1,
			Active2:       active2,
			Bid:           bid,
			BidVol:        bv,
			Ask:           ask,
			AskVol:        av,
			RiseSpeed:     float64(riseSpeedRaw) / 100.0,
			Unknown2:      unknown2,
			Unknown3:      unknown3,
			Unknown5:      unknown5,
			Unknown6:      unknown6,
			Unknown7:      unknown7,
			Unknown8:      unknown8,
			ServerTime:    formatServerTime(unknown0),
			TradingStatus: tradingStatus,
			OpenAmount:    float64(unknown3) * 100.0,
		})
	}
	return out, nil
}
