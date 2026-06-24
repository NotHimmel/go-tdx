package model

// SecurityQuote 单只股票实时五档行情。
type SecurityQuote struct {
	Market Market `json:"market"`
	Code   string `json:"code"`

	Price    float64 `json:"price"`     // 现价
	PreClose float64 `json:"pre_close"` // 昨收
	Open     float64 `json:"open"`      // 今开
	High     float64 `json:"high"`      // 最高
	Low      float64 `json:"low"`       // 最低

	Vol    float64 `json:"vol"`     // 总成交量（手）
	CurVol float64 `json:"cur_vol"` // 当前成交量
	Amount float64 `json:"amount"`  // 成交额（元）
	SVol   float64 `json:"s_vol"`   // 内盘（主动卖）
	BVol   float64 `json:"b_vol"`   // 外盘（主动买）

	Active1 uint16 `json:"active1"`
	Active2 uint16 `json:"active2"`

	Bid    [5]float64 `json:"bid"`     // 买一~买五
	BidVol [5]float64 `json:"bid_vol"`
	Ask    [5]float64 `json:"ask"`     // 卖一~卖五
	AskVol [5]float64 `json:"ask_vol"`

	RiseSpeed float64  `json:"rise_speed"` // 涨速
	LimitUp   *float64 `json:"limit_up"`   // 涨停价（业务规则计算）
	LimitDown *float64 `json:"limit_down"` // 跌停价

	Unknown2 int `json:"unknown_2"` // 指数: IndexOpenAmount/100; 个股: 舍入残差
	Unknown3 int `json:"unknown_3"` // 个股: StockOpenAmount/100; 指数: 负值
	Unknown5 int `json:"unknown_5"`
	Unknown6 int `json:"unknown_6"`
	Unknown7 int `json:"unknown_7"`
	Unknown8 int `json:"unknown_8"`

	ServerTime    string  `json:"server_time"`    // HH:MM:SS.mmm
	TradingStatus uint16  `json:"trading_status"` // 0x8020=停牌
	OpenAmount    float64 `json:"open_amount"`    // 集合竞价成交金额（元）
}
