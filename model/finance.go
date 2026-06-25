package model

// XdxrRecord 除权除息记录（一只股票可有多条）。
// 可选字段用指针表达「无值」（对应 Python 的 None）。
type XdxrRecord struct {
	Market   Market `json:"market"`
	Code     string `json:"code"`
	Year     int    `json:"year"`
	Month    int    `json:"month"`
	Day      int    `json:"day"`
	Category int    `json:"category"`
	Name     string `json:"name"`

	// category == 1（除权除息）
	Fenhong     *float64 `json:"fenhong,omitempty"`     // 每股分红（元）
	Peigujia    *float64 `json:"peigujia,omitempty"`    // 配股价
	Songzhuangu *float64 `json:"songzhuangu,omitempty"` // 每股送转股
	Peigu       *float64 `json:"peigu,omitempty"`       // 每股配股

	Suogu *float64 `json:"suogu,omitempty"` // category 11/12 缩股比例

	Xingquanjia *float64 `json:"xingquanjia,omitempty"` // category 13/14 行权价
	Fenshu      *float64 `json:"fenshu,omitempty"`

	// category 2..10（股本变动类，单位：万股）
	PanqianLiutong *float64 `json:"panqian_liutong,omitempty"`
	QianZongguben  *float64 `json:"qian_zongguben,omitempty"`
	PanhouLiutong  *float64 `json:"panhou_liutong,omitempty"`
	HouZongguben   *float64 `json:"hou_zongguben,omitempty"`
}

// XdxrCategoryNames 除权除息事件类型名称。
var XdxrCategoryNames = map[int]string{
	1: "除权除息", 2: "送配股上市", 3: "非流通股上市", 4: "未知股本变动",
	5: "股本变化", 6: "增发新股", 7: "股份回购", 8: "增发新股上市",
	9: "转配股上市", 10: "可转债上市", 11: "扩缩股", 12: "非流通股缩股",
	13: "送认购权证", 14: "送认沽权证",
}

// FinanceInfo 最新财务数据（单只股票）。股本单位万股，金额单位元。
type FinanceInfo struct {
	Market Market `json:"market"`
	Code   string `json:"code"`

	LiutongGuben    float64 `json:"liutong_guben"`
	ZongGuben       float64 `json:"zong_guben"`
	GuojiaGu        float64 `json:"guojia_gu"`
	FaqirenFarenGu  float64 `json:"faqiren_faren_gu"`
	FarenGu         float64 `json:"faren_gu"`
	BGu             float64 `json:"b_gu"`
	HGu             float64 `json:"h_gu"`
	ZhigongGu       float64 `json:"zhigong_gu"`

	Province     uint16 `json:"province"`
	Industry     uint16 `json:"industry"`
	UpdatedDate  uint32 `json:"updated_date"`
	IpoDate      uint32 `json:"ipo_date"`
	GudongRenshu float64 `json:"gudong_renshu"`

	ZongZichan     float64 `json:"zong_zichan"`
	LiudongZichan  float64 `json:"liudong_zichan"`
	GudingZichan   float64 `json:"guding_zichan"`
	WuxingZichan   float64 `json:"wuxing_zichan"`
	LiudongFuzhai  float64 `json:"liudong_fuzhai"`
	ChangqiFuzhai  float64 `json:"changqi_fuzhai"`
	ZibenGongjijin float64 `json:"ziben_gongjijin"`
	JingZichan     float64 `json:"jing_zichan"`

	ZhuyingShouru     float64 `json:"zhuying_shouru"`
	ZhuyingLirun      float64 `json:"zhuying_lirun"`
	YingshouZhangkuan float64 `json:"yingshou_zhangkuan"`
	YingyeLirun       float64 `json:"yingye_lirun"`
	TouziShouyu       float64 `json:"touzi_shouyu"`
	JingyingXianjinliu float64 `json:"jingying_xianjinliu"`
	ZongXianjinliu    float64 `json:"zong_xianjinliu"`
	Cunhuo            float64 `json:"cunhuo"`
	LirunZonghe       float64 `json:"lirun_zonghe"`
	ShuihouLirun      float64 `json:"shuihou_lirun"`
	JingLirun         float64 `json:"jing_lirun"`
	WeifenLirun       float64 `json:"weifen_lirun"`

	MeigujingZichan float64 `json:"meigujing_zichan"` // 每股净资产
	Reserve2        float64 `json:"reserve2"`
}

// CompanyInfoCategory 公司信息文件目录条目。
type CompanyInfoCategory struct {
	Name     string `json:"name"`
	Filename string `json:"filename"`
	Start    uint32 `json:"start"`
	Length   uint32 `json:"length"`
}

// TdxBlock 通达信板块（行业/地域/概念/风格）。
type TdxBlock struct {
	Name     string   `json:"name"`     // 板块名称
	Category int      `json:"category"` // 0=行业/指数 1=地域 2=概念 3=风格
	Count    int      `json:"count"`    // 成分股数量
	Codes    []string `json:"codes"`    // 6 位代码列表
}
