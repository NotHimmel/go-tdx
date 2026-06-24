package model

import "fmt"

// SecurityBar 单根 K 线（1m/5m/15m/30m/60m/日/周/月/季/年）。
type SecurityBar struct {
	Open   float64 `json:"open"`
	Close  float64 `json:"close"`
	High   float64 `json:"high"`
	Low    float64 `json:"low"`
	Vol    float64 `json:"vol"`    // 成交量（股）
	Amount float64 `json:"amount"` // 成交额（元）

	Year   int `json:"year"`
	Month  int `json:"month"`
	Day    int `json:"day"`
	Hour   int `json:"hour"`
	Minute int `json:"minute"`
}

// DatetimeStr 返回 "YYYY-MM-DD HH:MM" 格式时间。
func (b SecurityBar) DatetimeStr() string {
	return fmt.Sprintf("%d-%02d-%02d %02d:%02d", b.Year, b.Month, b.Day, b.Hour, b.Minute)
}
