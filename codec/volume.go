package codec

import (
	"encoding/binary"
	"fmt"
	"math"
)

// GetVolume 从 data[pos:pos+4] 解码成交量（通达信 4 字节自定义浮点）。
//
// 警告：专为成交量设计，不可用于价格字段（pytdx Bug #3）。
func GetVolume(data []byte, pos int) (float64, int, error) {
	if pos < 0 || pos+4 > len(data) {
		return 0, pos, fmt.Errorf("volume: 数据不足，偏移 %d", pos)
	}
	ivol := binary.LittleEndian.Uint32(data[pos:])
	return decodeVolume(ivol), pos + 4, nil
}

// DecodeVolume 直接解码 4 字节自定义浮点（成交量/股本/昨收等字段共用此编码）。
func DecodeVolume(ivol uint32) float64 { return decodeVolume(ivol) }

func decodeVolume(ivol uint32) float64 {
	if ivol == 0 {
		return 0.0
	}

	logpoint := int((ivol >> 24) & 0xFF)
	hleax := int((ivol >> 16) & 0xFF)
	lheax := int((ivol >> 8) & 0xFF)
	lleax := int(ivol & 0xFF)

	exp := logpoint*2 - 0x7F
	base := pow2(exp)

	expH := logpoint*2 - 0x86
	var hi float64
	if hleax > 0x80 {
		hi = pow2(expH)*128 + float64(hleax&0x7F)*pow2(expH+1)
	} else {
		hi = pow2(expH) * float64(hleax)
	}

	mid := pow2(logpoint*2-0x8E) * float64(lheax)
	lo := pow2(logpoint*2-0x96) * float64(lleax)

	if hleax&0x80 != 0 {
		mid *= 2.0
		lo *= 2.0
	}

	return base + hi + mid + lo
}

func pow2(exp int) float64 {
	if exp >= 0 {
		if exp < 63 {
			return float64(uint64(1) << uint(exp))
		}
		return math.Pow(2, float64(exp))
	}
	if -exp < 63 {
		return 1.0 / float64(uint64(1)<<uint(-exp))
	}
	return math.Pow(2, float64(exp))
}
