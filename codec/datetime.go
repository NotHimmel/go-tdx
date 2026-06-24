package codec

import (
	"encoding/binary"
	"fmt"

	"github.com/NotHimmel/go-tdx/model"
)

// GetDatetimeMinute 解析分钟级时间戳（4 字节）。返回 year,month,day,hour,minute,newPos。
func GetDatetimeMinute(data []byte, pos int) (int, int, int, int, int, int, error) {
	if pos < 0 || pos+4 > len(data) {
		return 0, 0, 0, 0, 0, pos, fmt.Errorf("minute datetime: 数据不足，偏移 %d", pos)
	}
	zipday := int(binary.LittleEndian.Uint16(data[pos:]))
	tminutes := int(binary.LittleEndian.Uint16(data[pos+2:]))
	year := (zipday >> 11) + 2004
	month := (zipday % 2048) / 100
	day := (zipday % 2048) % 100
	hour := tminutes / 60
	minute := tminutes % 60
	return year, month, day, hour, minute, pos + 4, nil
}

// GetDatetimeDay 解析日期（4 字节 YYYYMMDD）。返回 year,month,day,newPos。
func GetDatetimeDay(data []byte, pos int) (int, int, int, int, error) {
	if pos < 0 || pos+4 > len(data) {
		return 0, 0, 0, pos, fmt.Errorf("day datetime: 数据不足，偏移 %d", pos)
	}
	zipday := int(binary.LittleEndian.Uint32(data[pos:]))
	year := zipday / 10000
	month := (zipday % 10000) / 100
	day := zipday % 100
	return year, month, day, pos + 4, nil
}

// GetDatetime 根据 KlineCategory 选择解析格式。
// 日线及以上时 hour=15, minute=0（收盘时间，与 pytdx 一致）。
func GetDatetime(category model.KlineCategory, data []byte, pos int) (int, int, int, int, int, int, error) {
	c := int(category)
	if c < 4 || c == 7 || c == 8 {
		return GetDatetimeMinute(data, pos)
	}
	year, month, day, newPos, err := GetDatetimeDay(data, pos)
	return year, month, day, 15, 0, newPos, err
}

// GetTime 解析 2 字节时间（分钟数）。返回 hour,minute,newPos。
func GetTime(data []byte, pos int) (int, int, int, error) {
	if pos < 0 || pos+2 > len(data) {
		return 0, 0, pos, fmt.Errorf("trade time: 数据不足，偏移 %d", pos)
	}
	tminutes := int(binary.LittleEndian.Uint16(data[pos:]))
	return tminutes / 60, tminutes % 60, pos + 2, nil
}
