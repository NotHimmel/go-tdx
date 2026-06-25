package codec

import (
	"encoding/binary"
	"strings"

	"github.com/NotHimmel/go-tdx/model"
)

// ParseBlockDat 解析通达信 .dat 板块文件。
//
// 格式：384 字节头(跳过) + 2 字节 count + 每条记录 2813 字节
// 记录: 9s 名称 + H 股票数 + H 类型 + 2800s 代码区(每只 7 字节)。
// filename 用于推断分类: zs→行业/指数(0), gn→概念(2), fg→风格(3)。
func ParseBlockDat(data []byte, filename string) []model.TdxBlock {
	if len(data) < 386 {
		return nil
	}
	pos := 384
	count := int(binary.LittleEndian.Uint16(data[pos:]))
	pos += 2

	category := 0
	switch {
	case strings.Contains(filename, "gn"):
		category = 2
	case strings.Contains(filename, "fg"):
		category = 3
	}

	const recSize = 2813
	out := make([]model.TdxBlock, 0, count)
	for i := 0; i < count; i++ {
		if pos+recSize > len(data) {
			break
		}
		name := DecodeGBK(data[pos : pos+9])
		stockCount := int(binary.LittleEndian.Uint16(data[pos+9:]))
		codesStart := pos + 13

		actual := stockCount
		if actual > 400 { // 2800/7
			actual = 400
		}
		codes := make([]string, 0, actual)
		for j := 0; j < actual; j++ {
			cs := codesStart + j*7
			code := TrimNulString(data[cs : cs+7])
			if code != "" {
				codes = append(codes, code)
			}
		}
		out = append(out, model.TdxBlock{
			Name:     name,
			Category: category,
			Count:    stockCount,
			Codes:    codes,
		})
		pos += recSize
	}
	return out
}
