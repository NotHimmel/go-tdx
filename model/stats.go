package model

// MarketStat 全市场涨跌统计概况。
type MarketStat struct {
	UpCount        int     `json:"up_count"`
	DownCount      int     `json:"down_count"`
	NeutralCount   int     `json:"neutral_count"`
	SuspendedCount int     `json:"suspended_count"` // total-(up+down+neutral) 残差
	TotalCount     int     `json:"total_count"`
	TotalAmount    float64 `json:"total_amount"`
	TotalVolume    float64 `json:"total_volume"`
	TotalMarketCap float64 `json:"total_market_cap"` // 来自 880001
	LimitUpCount   int     `json:"limit_up_count"`   // 来自 880006 close
	LimitDownCount int     `json:"limit_down_count"` // 来自 880006 open
}

// HistoricalFundFlow 历史日线资金流向条目（金额单位元）。
type HistoricalFundFlow struct {
	Year  int `json:"year"`
	Month int `json:"month"`
	Day   int `json:"day"`

	SuperIn   float64 `json:"super_in"`
	SuperOut  float64 `json:"super_out"`
	LargeIn   float64 `json:"large_in"`
	LargeOut  float64 `json:"large_out"`
	MediumIn  float64 `json:"medium_in"`
	MediumOut float64 `json:"medium_out"`
	SmallIn   float64 `json:"small_in"`
	SmallOut  float64 `json:"small_out"`
}

// MainNetInflow 当日主力净流入（超大单+大单）。
func (f HistoricalFundFlow) MainNetInflow() float64 {
	return (f.SuperIn + f.LargeIn) - (f.SuperOut + f.LargeOut)
}
