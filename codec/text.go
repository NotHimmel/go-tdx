package codec

import (
	"bytes"

	"golang.org/x/text/encoding/simplifiedchinese"
)

// DecodeGBK 将 GBK 字节解码为 UTF-8 字符串，先截断到首个 NUL。
// 解码失败时回退为按字节字符串（errors='replace' 等价行为）。
func DecodeGBK(b []byte) string {
	b = trimNul(b)
	out, err := simplifiedchinese.GBK.NewDecoder().Bytes(b)
	if err != nil {
		return string(b)
	}
	return string(out)
}

// TrimNulString 截断到首个 NUL 并按原字节（UTF-8/ASCII）返回。
func TrimNulString(b []byte) string {
	return string(trimNul(b))
}

func trimNul(b []byte) []byte {
	if i := bytes.IndexByte(b, 0); i >= 0 {
		return b[:i]
	}
	return b
}
