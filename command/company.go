package command

import (
	"encoding/binary"
	"fmt"

	"github.com/NotHimmel/go-tdx/codec"
	"github.com/NotHimmel/go-tdx/model"
	"golang.org/x/text/encoding/simplifiedchinese"
)

const companyCategoryRecordSize = 152 // 64 name + 80 filename + 4 start + 4 length

// CompanyInfoCategoryCmd 获取公司信息文件目录。
type CompanyInfoCategoryCmd struct {
	Market model.Market
	Code   string
}

func (c CompanyInfoCategoryCmd) BuildRequest() []byte {
	buf := hexBytes("0c0f109b00010e000e00cf02")
	buf = binary.LittleEndian.AppendUint16(buf, uint16(c.Market))
	cb := code6(c.Code)
	buf = append(buf, cb[:]...)
	return binary.LittleEndian.AppendUint32(buf, 0)
}

func (c CompanyInfoCategoryCmd) ParseResponse(body []byte) ([]model.CompanyInfoCategory, error) {
	if len(body) < 2 {
		return nil, fmt.Errorf("company_info_category body 过短")
	}
	num := int(binary.LittleEndian.Uint16(body))
	pos := 2
	out := make([]model.CompanyInfoCategory, 0, num)
	for i := 0; i < num; i++ {
		if pos+companyCategoryRecordSize > len(body) {
			return nil, fmt.Errorf("company_info_category record: 数据不足")
		}
		rec := body[pos : pos+companyCategoryRecordSize]
		out = append(out, model.CompanyInfoCategory{
			Name:     codec.DecodeGBK(rec[0:64]),
			Filename: codec.DecodeGBK(rec[64:144]),
			Start:    binary.LittleEndian.Uint32(rec[144:148]),
			Length:   binary.LittleEndian.Uint32(rec[148:152]),
		})
		pos += companyCategoryRecordSize
	}
	return out, nil
}

// CompanyInfoContentCmd 按文件名/偏移/长度读取公司信息文本（GBK）。
type CompanyInfoContentCmd struct {
	Market   model.Market
	Code     string
	Filename string
	Offset   uint32
	Length   uint32
}

func (c CompanyInfoContentCmd) BuildRequest() []byte {
	fname, _ := simplifiedchinese.GBK.NewEncoder().Bytes([]byte(c.Filename))
	var fnamePadded [80]byte
	copy(fnamePadded[:], fname)

	buf := hexBytes("0c07109c000168006800d002")
	buf = binary.LittleEndian.AppendUint16(buf, uint16(c.Market))
	cb := code6(c.Code)
	buf = append(buf, cb[:]...)
	buf = binary.LittleEndian.AppendUint16(buf, 0)
	buf = append(buf, fnamePadded[:]...)
	buf = binary.LittleEndian.AppendUint32(buf, c.Offset)
	buf = binary.LittleEndian.AppendUint32(buf, c.Length)
	return binary.LittleEndian.AppendUint32(buf, 0)
}

func (c CompanyInfoContentCmd) ParseResponse(body []byte) (string, error) {
	if len(body) < 12 {
		return "", fmt.Errorf("company_info_content body 过短")
	}
	length := int(binary.LittleEndian.Uint16(body[10:12]))
	if 12+length > len(body) {
		return "", fmt.Errorf("company_info_content: 数据不足，需要 %d", length)
	}
	return codec.DecodeGBK(body[12 : 12+length]), nil
}
