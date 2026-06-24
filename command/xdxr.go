package command

import (
	"encoding/binary"
	"fmt"

	"github.com/NotHimmel/go-tdx/codec"
	"github.com/NotHimmel/go-tdx/model"
)

// XdxrInfoCmd 获取除权除息历史记录。
type XdxrInfoCmd struct {
	Market model.Market
	Code   string
}

func (c XdxrInfoCmd) BuildRequest() []byte {
	buf := hexBytes("0c1f187600010b000b000f000100")
	buf = append(buf, byte(c.Market))
	cb := code6(c.Code)
	return append(buf, cb[:]...)
}

func fptr(v float64) *float64 { return &v }

func (c XdxrInfoCmd) ParseResponse(body []byte) ([]model.XdxrRecord, error) {
	if len(body) < 11 {
		return nil, fmt.Errorf("xdxr_info body 过短")
	}
	pos := 9 // 跳过 market+code+未知
	num := int(binary.LittleEndian.Uint16(body[pos:]))
	pos += 2

	recs := make([]model.XdxrRecord, 0, num)
	for i := 0; i < num; i++ {
		if pos+7 > len(body) {
			return nil, fmt.Errorf("xdxr_info record header: 数据不足")
		}
		marketB := body[pos]
		codeStr := codec.TrimNulString(body[pos+1 : pos+7])
		pos += 7
		pos++ // 跳过 1 未知字节

		year, month, day, _, _, np, err := codec.GetDatetime(9, body, pos)
		if err != nil {
			return nil, err
		}
		pos = np
		if pos+1+16 > len(body) {
			return nil, fmt.Errorf("xdxr_info record body: 数据不足")
		}
		category := int(body[pos])
		pos++
		chunk := body[pos : pos+16]
		pos += 16
		if marketB > 2 {
			return nil, fmt.Errorf("xdxr_info 非法 market 值: %d", marketB)
		}

		name := model.XdxrCategoryNames[category]
		if name == "" {
			name = fmt.Sprintf("%d", category)
		}
		rec := model.XdxrRecord{
			Market:   model.Market(marketB),
			Code:     codeStr,
			Year:     year,
			Month:    month,
			Day:      day,
			Category: category,
			Name:     name,
		}

		switch {
		case category == 1:
			rec.Fenhong = fptr(f32(chunk, 0) / 10.0)
			rec.Peigujia = fptr(f32(chunk, 4))
			rec.Songzhuangu = fptr(f32(chunk, 8) / 10.0)
			rec.Peigu = fptr(f32(chunk, 12) / 10.0)
		case category == 11 || category == 12:
			rec.Suogu = fptr(f32(chunk, 8)) // <IIfI 第三字段
		case category == 13 || category == 14:
			rec.Xingquanjia = fptr(f32(chunk, 0)) // <fIfI
			rec.Fenshu = fptr(f32(chunk, 8))
		default:
			// 股本变动类：4 个 uint32，自定义浮点 → 万股
			rec.PanqianLiutong = fptr(codec.DecodeVolume(binary.LittleEndian.Uint32(chunk[0:])))
			rec.QianZongguben = fptr(codec.DecodeVolume(binary.LittleEndian.Uint32(chunk[4:])))
			rec.PanhouLiutong = fptr(codec.DecodeVolume(binary.LittleEndian.Uint32(chunk[8:])))
			rec.HouZongguben = fptr(codec.DecodeVolume(binary.LittleEndian.Uint32(chunk[12:])))
		}
		recs = append(recs, rec)
	}
	return recs, nil
}
