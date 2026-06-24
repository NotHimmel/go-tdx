package codec

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"io"
)

// HeaderSize 响应帧固定头长度。
const HeaderSize = 16

// FrameHeader 16 字节响应帧头。
//
// 布局（小端）：
//
//	0:  uint32 magic     协议魔数，恒为 7654321 (0x0074CBB1)
//	4:  uint32 seqID      ZipFlag(1B) + 请求 bytes 1-4 回显(3B)
//	8:  uint32 method     SeqID 高字节 + 保留 + Method(2B)
//	12: uint16 zipsize    body 实际长度
//	14: uint16 unzipsize  解压后长度（== zipsize 表示未压缩）
type FrameHeader struct {
	Magic     uint32
	SeqID     uint32
	Method    uint32
	ZipSize   uint16
	UnzipSize uint16
}

// ParseHeader 解析 16 字节响应帧头。
func ParseHeader(buf []byte) (FrameHeader, error) {
	if len(buf) < HeaderSize {
		return FrameHeader{}, fmt.Errorf("frame header: 数据不足，需要 %d 字节，实际 %d", HeaderSize, len(buf))
	}
	return FrameHeader{
		Magic:     binary.LittleEndian.Uint32(buf[0:]),
		SeqID:     binary.LittleEndian.Uint32(buf[4:]),
		Method:    binary.LittleEndian.Uint32(buf[8:]),
		ZipSize:   binary.LittleEndian.Uint16(buf[12:]),
		UnzipSize: binary.LittleEndian.Uint16(buf[14:]),
	}, nil
}

// DecompressBody 按需 zlib 解压 body。zipsize == unzipsize 时直接返回原始字节。
func DecompressBody(h FrameHeader, rawBody []byte) ([]byte, error) {
	if len(rawBody) != int(h.ZipSize) {
		return nil, fmt.Errorf("frame body 长度不符: header=%d, actual=%d", h.ZipSize, len(rawBody))
	}
	var body []byte
	if h.ZipSize == h.UnzipSize {
		body = rawBody
	} else {
		r, err := zlib.NewReader(bytes.NewReader(rawBody))
		if err != nil {
			return nil, fmt.Errorf("frame body zlib 解压失败: %w", err)
		}
		defer r.Close()
		body, err = io.ReadAll(r)
		if err != nil {
			return nil, fmt.Errorf("frame body zlib 解压失败: %w", err)
		}
	}
	if len(body) != int(h.UnzipSize) {
		return nil, fmt.Errorf("frame body 解压长度不符: header=%d, actual=%d", h.UnzipSize, len(body))
	}
	return body, nil
}
