package command

import (
	"encoding/binary"
	"fmt"
	"math"

	"github.com/NotHimmel/go-tdx/codec"
	"github.com/NotHimmel/go-tdx/model"
)

const finScale = 10000.0 // 财务数据单位：万元/万股

// finFloat 读 little-endian float32。
func f32(b []byte, off int) float64 {
	return float64(math.Float32frombits(binary.LittleEndian.Uint32(b[off:])))
}

// FinanceInfoCmd 获取单只股票最新财务数据。
type FinanceInfoCmd struct {
	Market model.Market
	Code   string
}

func (c FinanceInfoCmd) BuildRequest() []byte {
	buf := hexBytes("0c1f187600010b000b0010000100")
	buf = append(buf, byte(c.Market))
	cb := code6(c.Code)
	return append(buf, cb[:]...)
}

// 财务体：<f H H I I + 30f = 4+2+2+4+4 + 120 = 136 字节。
const finBodySize = 4 + 2 + 2 + 4 + 4 + 30*4

func (c FinanceInfoCmd) ParseResponse(body []byte) (model.FinanceInfo, error) {
	pos := 2 // 跳过记录数
	if pos+7 > len(body) {
		return model.FinanceInfo{}, fmt.Errorf("finance_info header: 数据不足")
	}
	marketB := body[pos]
	codeStr := codec.TrimNulString(body[pos+1 : pos+7])
	pos += 7
	if pos+finBodySize > len(body) {
		return model.FinanceInfo{}, fmt.Errorf("finance_info body: 数据不足")
	}
	d := body[pos:]
	// 解析定长字段
	liutongGuben := f32(d, 0)
	province := binary.LittleEndian.Uint16(d[4:])
	industry := binary.LittleEndian.Uint16(d[6:])
	updatedDate := binary.LittleEndian.Uint32(d[8:])
	ipoDate := binary.LittleEndian.Uint32(d[12:])
	// 30 个 float32 从偏移 16 起
	fl := func(i int) float64 { return f32(d, 16+i*4) }
	if marketB > 2 {
		return model.FinanceInfo{}, fmt.Errorf("finance_info 非法 market 值: %d", marketB)
	}

	return model.FinanceInfo{
		Market:             model.Market(marketB),
		Code:               codeStr,
		LiutongGuben:       liutongGuben * finScale,
		Province:           province,
		Industry:           industry,
		UpdatedDate:        updatedDate,
		IpoDate:            ipoDate,
		ZongGuben:          fl(0) * finScale,
		GuojiaGu:           fl(1) * finScale,
		FaqirenFarenGu:     fl(2) * finScale,
		FarenGu:            fl(3) * finScale,
		BGu:                fl(4) * finScale,
		HGu:                fl(5) * finScale,
		ZhigongGu:          fl(6) * finScale,
		ZongZichan:         fl(7) * finScale,
		LiudongZichan:      fl(8) * finScale,
		GudingZichan:       fl(9) * finScale,
		WuxingZichan:       fl(10) * finScale,
		GudongRenshu:       fl(11),
		LiudongFuzhai:      fl(12) * finScale,
		ChangqiFuzhai:      fl(13) * finScale,
		ZibenGongjijin:     fl(14) * finScale,
		JingZichan:         fl(15) * finScale,
		ZhuyingShouru:      fl(16) * finScale,
		ZhuyingLirun:       fl(17) * finScale,
		YingshouZhangkuan:  fl(18) * finScale,
		YingyeLirun:        fl(19) * finScale,
		TouziShouyu:        fl(20) * finScale,
		JingyingXianjinliu: fl(21) * finScale,
		ZongXianjinliu:     fl(22) * finScale,
		Cunhuo:             fl(23) * finScale,
		LirunZonghe:        fl(24) * finScale,
		ShuihouLirun:       fl(25) * finScale,
		JingLirun:          fl(26) * finScale,
		WeifenLirun:        fl(27) * finScale,
		MeigujingZichan:    fl(28),
		Reserve2:           fl(29),
	}, nil
}
