package command

import (
	"encoding/binary"
	"fmt"

	"github.com/NotHimmel/go-tdx/codec"
	"github.com/NotHimmel/go-tdx/model"
)

// ── 历史资金流向 (Category 22) ──

// HistoryFundFlowCmd 获取历史日线资金流向序列。
type HistoryFundFlowCmd struct {
	Market model.Market
	Code   string
	Start  uint16
	Count  uint16
}

func (c HistoryFundFlowCmd) BuildRequest() []byte {
	buf := make([]byte, 0, 40)
	buf = binary.LittleEndian.AppendUint16(buf, 0x010C)
	buf = binary.LittleEndian.AppendUint32(buf, 0x01016408)
	buf = binary.LittleEndian.AppendUint16(buf, 0x001C)
	buf = binary.LittleEndian.AppendUint16(buf, 0x001C)
	buf = binary.LittleEndian.AppendUint16(buf, 0x052D)
	buf = binary.LittleEndian.AppendUint16(buf, uint16(c.Market))
	cb := code6(c.Code)
	buf = append(buf, cb[:]...)
	buf = binary.LittleEndian.AppendUint16(buf, 22) // category
	buf = binary.LittleEndian.AppendUint16(buf, 1)
	buf = binary.LittleEndian.AppendUint16(buf, c.Start)
	buf = binary.LittleEndian.AppendUint16(buf, c.Count)
	buf = binary.LittleEndian.AppendUint32(buf, 0)
	buf = binary.LittleEndian.AppendUint32(buf, 0)
	return binary.LittleEndian.AppendUint16(buf, 0)
}

func (c HistoryFundFlowCmd) ParseResponse(body []byte) ([]model.HistoricalFundFlow, error) {
	if len(body) < 11 {
		return []model.HistoricalFundFlow{}, nil
	}
	num := int(binary.LittleEndian.Uint16(body[9:11]))
	pos := 11
	out := make([]model.HistoricalFundFlow, 0, num)
	for i := 0; i < num; i++ {
		if pos+36 > len(body) {
			break
		}
		u := func(off int) float64 {
			return codec.DecodeVolume(binary.LittleEndian.Uint32(body[pos+off:]))
		}
		rawDate := int(binary.LittleEndian.Uint32(body[pos:]))
		out = append(out, model.HistoricalFundFlow{
			Year:      rawDate / 10000,
			Month:     (rawDate / 100) % 100,
			Day:       rawDate % 100,
			SuperIn:   u(4),
			LargeIn:   u(8),
			MediumIn:  u(12),
			SmallIn:   u(16),
			SuperOut:  u(20),
			LargeOut:  u(24),
			MediumOut: u(28),
			SmallOut:  u(32),
		})
		pos += 36
	}
	return out, nil
}

// ── 板块文件 ──

// BlockInfoMetaCmd 获取板块文件元数据（大小 + MD5）。
type BlockInfoMetaCmd struct{ Filename string }

func (c BlockInfoMetaCmd) BuildRequest() []byte {
	buf := hexBytes("0c39186900012a002a00c502")
	var payload [40]byte
	copy(payload[:], c.Filename)
	return append(buf, payload[:]...)
}

func (c BlockInfoMetaCmd) ParseResponse(body []byte) (size uint32, hash string, err error) {
	if len(body) < 38 {
		return 0, "", fmt.Errorf("block_info_meta 响应过短: %d", len(body))
	}
	size = binary.LittleEndian.Uint32(body[0:4])
	hash = codec.TrimNulString(body[5:37])
	return size, hash, nil
}

// BlockInfoCmd 分段获取板块文件二进制内容。
type BlockInfoCmd struct {
	Filename string
	Start    uint32
	Length   uint32
}

func (c BlockInfoCmd) BuildRequest() []byte {
	return buildFileChunkReq("0c37186a00016e006e00b906", c.Start, c.Length, c.Filename)
}

func (c BlockInfoCmd) ParseResponse(body []byte) ([]byte, error) {
	if len(body) < 4 {
		return []byte{}, nil
	}
	return body[4:], nil
}

// ── 大文件拉取 ──

// ReportFileCmd 分段获取服务器报表/基础信息文件。
type ReportFileCmd struct {
	Filename string
	Start    uint32
	Length   uint32 // 建议 30000
}

func (c ReportFileCmd) BuildRequest() []byte {
	length := c.Length
	if length == 0 {
		length = 30000
	}
	return buildFileChunkReq("0c37186a00016e006e00b906", c.Start, length, c.Filename)
}

func (c ReportFileCmd) ParseResponse(body []byte) ([]byte, error) {
	if len(body) < 4 {
		return []byte{}, nil
	}
	return body[4:], nil
}

func buildFileChunkReq(header string, start, length uint32, filename string) []byte {
	buf := hexBytes(header)
	buf = binary.LittleEndian.AppendUint32(buf, start)
	buf = binary.LittleEndian.AppendUint32(buf, length)
	var payload [100]byte
	copy(payload[:], filename)
	return append(buf, payload[:]...)
}
