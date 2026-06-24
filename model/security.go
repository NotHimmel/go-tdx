package model

// SecurityInfo 证券列表条目（来自 SecurityList）。
type SecurityInfo struct {
	Market       Market  `json:"market"`
	Code         string  `json:"code"`
	Name         string  `json:"name"`          // GBK 解码
	VolUnit      uint16  `json:"volunit"`       // 成交量单位（手 = volunit 股）
	DecimalPoint uint8   `json:"decimal_point"` // 价格小数位数
	PreClose     float64 `json:"pre_close"`     // 昨收价（通达信自定义浮点）
}
