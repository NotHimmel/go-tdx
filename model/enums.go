// Package model holds typed structs and enums for the TDX A-share protocol.
package model

// Market 市场代码。
type Market uint16

const (
	MarketSZ Market = 0 // 深圳
	MarketSH Market = 1 // 上海
	MarketBJ Market = 2 // 北京（get_security_list 不稳定，勿依赖）
)

// KlineCategory K 线周期。
type KlineCategory uint16

const (
	Min5    KlineCategory = 0
	Min15   KlineCategory = 1
	Min30   KlineCategory = 2
	Min60   KlineCategory = 3
	Day     KlineCategory = 4
	Week    KlineCategory = 5
	Month   KlineCategory = 6
	Min1    KlineCategory = 7
	Min3    KlineCategory = 8 // 通达信内部用，实际同 Min1
	Year    KlineCategory = 9
	Season  KlineCategory = 10
	YearAlt KlineCategory = 11
)
