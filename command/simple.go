package command

import (
	"encoding/binary"
	"fmt"

	"github.com/NotHimmel/go-tdx/codec"
	"github.com/NotHimmel/go-tdx/model"
)

// hexBytes 解析十六进制字面量（panic on error，仅用于固定头常量）。
func hexBytes(s string) []byte {
	out := make([]byte, len(s)/2)
	for i := 0; i < len(out); i++ {
		out[i] = hexNibble(s[2*i])<<4 | hexNibble(s[2*i+1])
	}
	return out
}

func hexNibble(c byte) byte {
	switch {
	case c >= '0' && c <= '9':
		return c - '0'
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10
	}
	return 0
}

func code6(code string) [6]byte {
	var b [6]byte
	copy(b[:], code)
	return b
}

// ── SecurityCount ──

// SecurityCountCmd 返回指定市场证券总数。
type SecurityCountCmd struct{ Market model.Market }

func (c SecurityCountCmd) BuildRequest() []byte {
	buf := hexBytes("0c0c186c000108000800" + "4e04")
	buf = binary.LittleEndian.AppendUint16(buf, uint16(c.Market))
	return append(buf, 0x75, 0xc7, 0x33, 0x01)
}

func (c SecurityCountCmd) ParseResponse(body []byte) (int, error) {
	if len(body) < 2 {
		return 0, fmt.Errorf("security_count: 数据不足")
	}
	return int(binary.LittleEndian.Uint16(body)), nil
}

// ── SecurityList ──

const securityListRecordSize = 29

// SecurityListCmd 获取从 start 开始的证券列表（每页最多 1000 条）。
type SecurityListCmd struct {
	Market model.Market
	Start  uint16
}

func (c SecurityListCmd) BuildRequest() []byte {
	buf := hexBytes("0c0118640101060006005004")
	buf = binary.LittleEndian.AppendUint16(buf, uint16(c.Market))
	buf = binary.LittleEndian.AppendUint16(buf, c.Start)
	buf = binary.LittleEndian.AppendUint16(buf, 0)
	return buf
}

func (c SecurityListCmd) ParseResponse(body []byte) ([]model.SecurityInfo, error) {
	if len(body) < 2 {
		return nil, fmt.Errorf("security_list header: 数据不足")
	}
	num := int(binary.LittleEndian.Uint16(body))
	pos := 2
	out := make([]model.SecurityInfo, 0, num)
	for i := 0; i < num; i++ {
		if pos+securityListRecordSize > len(body) {
			return nil, fmt.Errorf("security_list record: 数据不足，偏移 %d", pos)
		}
		rec := body[pos : pos+securityListRecordSize]
		// <6sH8s4sBI4s
		code := codec.TrimNulString(rec[0:6])
		volunit := binary.LittleEndian.Uint16(rec[6:8])
		name := codec.DecodeGBK(rec[8:16])
		decimalPoint := rec[20]
		preCloseRaw := binary.LittleEndian.Uint32(rec[21:25])
		out = append(out, model.SecurityInfo{
			Market:       c.Market,
			Code:         code,
			Name:         name,
			VolUnit:      volunit,
			DecimalPoint: decimalPoint,
			PreClose:     codec.DecodeVolume(preCloseRaw),
		})
		pos += securityListRecordSize
	}
	return out, nil
}
