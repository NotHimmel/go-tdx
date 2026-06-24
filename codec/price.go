// Package codec 实现通达信 TCP 协议编解码（价格/成交量/时间/帧）。
package codec

import "fmt"

// GetPrice 解码一个变长有符号整数（类 LEB128）。
//
// 协议规则：
//   - 首字节：bit7=继续标记，bit6=符号（1=负），bit5~0=低 6 位数据
//   - 后续字节：bit7=继续标记，bit6~0=7 位数据
//   - 低位在前
//
// 返回 (value, newPos, err)。
func GetPrice(data []byte, pos int) (int, int, error) {
	bitShift := 6
	start := pos
	if pos < 0 || pos >= len(data) {
		return 0, pos, fmt.Errorf("price varint 截断: offset=%d", start)
	}
	b := data[pos]
	value := int(b & 0x3F)
	negative := b&0x40 != 0

	if b&0x80 != 0 {
		for {
			pos++
			if pos >= len(data) {
				return 0, start, fmt.Errorf("price varint 截断: offset=%d", start)
			}
			b = data[pos]
			value |= int(b&0x7F) << bitShift
			bitShift += 7
			if b&0x80 == 0 {
				break
			}
		}
	}
	pos++
	if negative {
		value = -value
	}
	return value, pos, nil
}

// PutPrice 将整数编码为变长格式（用于构造请求包）。
func PutPrice(value int) []byte {
	negative := value < 0
	if negative {
		value = -value
	}

	first := byte(value & 0x3F)
	value >>= 6
	if negative {
		first |= 0x40
	}
	if value != 0 {
		first |= 0x80
	}

	out := []byte{first}
	for value != 0 {
		b := byte(value & 0x7F)
		value >>= 7
		if value != 0 {
			b |= 0x80
		}
		out = append(out, b)
	}
	return out
}
