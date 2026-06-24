package model

// MinuteBar 今日/历史分时（每分钟一条，共 240 条）。
type MinuteBar struct {
	Price    float64 `json:"price"`
	Vol      int     `json:"vol"`
	Unknown1 int     `json:"_unknown_1"` // 含义未明（疑似均价编码）
}

// TransactionRecord 逐笔成交记录。时间精度到分钟（协议限制）。
type TransactionRecord struct {
	Hour        int     `json:"hour"`
	Minute      int     `json:"minute"`
	Price       float64 `json:"price"`
	Vol         int     `json:"vol"`
	BuyOrSell   int     `json:"buyorsell"`    // 0=买 1=卖 2=中性 8=集合竞价
	UnknownLast int     `json:"unknown_last"`
}
