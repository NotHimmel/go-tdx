package command

import (
	"strings"
	"testing"

	"github.com/NotHimmel/go-tdx/model"
)

func TestSecurityCountParity(t *testing.T) {
	body := loadFixture(t, "security_count")
	var want struct {
		Count int `json:"count"`
	}
	loadJSON(t, "security_count", &want)
	got, err := SecurityCountCmd{}.ParseResponse(body)
	if err != nil {
		t.Fatal(err)
	}
	if got != want.Count {
		t.Fatalf("count: got %d want %d", got, want.Count)
	}
}

func TestSecurityListParity(t *testing.T) {
	body := loadFixture(t, "security_list")
	var want struct {
		Num   int `json:"num_records"`
		First struct {
			Code     string  `json:"code"`
			Name     string  `json:"name"`
			PreClose float64 `json:"pre_close"`
		} `json:"first"`
	}
	loadJSON(t, "security_list", &want)
	got, err := SecurityListCmd{Market: model.MarketSH}.ParseResponse(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != want.Num {
		t.Fatalf("num: got %d want %d", len(got), want.Num)
	}
	if got[0].Code != want.First.Code {
		t.Errorf("code: got %q want %q", got[0].Code, want.First.Code)
	}
	if got[0].Name != want.First.Name {
		t.Errorf("name: got %q want %q", got[0].Name, want.First.Name)
	}
	if !approx(got[0].PreClose, want.First.PreClose) {
		t.Errorf("pre_close: got %v want %v", got[0].PreClose, want.First.PreClose)
	}
}

func TestSecurityQuotesParity(t *testing.T) {
	body := loadFixture(t, "security_quotes")
	var want struct {
		Num   int `json:"num_records"`
		First struct {
			Code     string  `json:"code"`
			PreClose float64 `json:"pre_close"`
			Unknown2 int     `json:"unknown_2"`
			Unknown3 int     `json:"unknown_3"`
		} `json:"first"`
	}
	loadJSON(t, "security_quotes", &want)
	got, err := SecurityQuotesCmd{Stocks: []Stock{{model.MarketSH, "600000"}}}.ParseResponse(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != want.Num {
		t.Fatalf("num: got %d want %d", len(got), want.Num)
	}
	f := got[0]
	if f.Code != want.First.Code {
		t.Errorf("code: got %q want %q", f.Code, want.First.Code)
	}
	if !approx(f.PreClose, want.First.PreClose) {
		t.Errorf("pre_close: got %v want %v", f.PreClose, want.First.PreClose)
	}
	if f.Unknown2 != want.First.Unknown2 {
		t.Errorf("unknown_2: got %d want %d", f.Unknown2, want.First.Unknown2)
	}
	if f.Unknown3 != want.First.Unknown3 {
		t.Errorf("unknown_3: got %d want %d", f.Unknown3, want.First.Unknown3)
	}
}

func TestMinuteTimeParity(t *testing.T) {
	body := loadFixture(t, "minute_time")
	var want struct {
		Num   int `json:"num_records"`
		First struct {
			Price    float64 `json:"price"`
			Vol      int     `json:"vol"`
			Unknown1 int     `json:"_unknown_1"`
		} `json:"first"`
	}
	loadJSON(t, "minute_time", &want)
	got, err := MinuteTimeCmd{}.ParseResponse(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != want.Num {
		t.Fatalf("num: got %d want %d", len(got), want.Num)
	}
	f := got[0]
	if !approx(f.Price, want.First.Price) || f.Vol != want.First.Vol || f.Unknown1 != want.First.Unknown1 {
		t.Errorf("first: got %+v want %+v", f, want.First)
	}
}

func TestHistoryMinuteTimeParity(t *testing.T) {
	body := loadFixture(t, "history_minute_time")
	var want struct {
		Num   int `json:"num_records"`
		First struct {
			Price float64 `json:"price"`
			Vol   int     `json:"vol"`
		} `json:"first"`
	}
	loadJSON(t, "history_minute_time", &want)
	got, err := HistoryMinuteTimeCmd{}.ParseResponse(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != want.Num {
		t.Fatalf("num: got %d want %d", len(got), want.Num)
	}
	if !approx(got[0].Price, want.First.Price) || got[0].Vol != want.First.Vol {
		t.Errorf("first: got %+v want %+v", got[0], want.First)
	}
}

func TestTransactionParity(t *testing.T) {
	body := loadFixture(t, "transaction")
	var want struct {
		Num   int `json:"num_records"`
		First struct {
			Hour   int     `json:"hour"`
			Minute int     `json:"minute"`
			Price  float64 `json:"price"`
			Vol    int     `json:"vol"`
		} `json:"first"`
	}
	loadJSON(t, "transaction", &want)
	got, err := TransactionCmd{}.ParseResponse(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != want.Num {
		t.Fatalf("num: got %d want %d", len(got), want.Num)
	}
	f := got[0]
	if f.Hour != want.First.Hour || f.Minute != want.First.Minute || !approx(f.Price, want.First.Price) || f.Vol != want.First.Vol {
		t.Errorf("first: got %+v want %+v", f, want.First)
	}
}

func TestHistoryTransactionParity(t *testing.T) {
	body := loadFixture(t, "history_transaction")
	var want struct {
		Num   int `json:"num_records"`
		First struct {
			Hour   int     `json:"hour"`
			Minute int     `json:"minute"`
			Price  float64 `json:"price"`
			Vol    int     `json:"vol"`
		} `json:"first"`
	}
	loadJSON(t, "history_transaction", &want)
	got, err := HistoryTransactionCmd{}.ParseResponse(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != want.Num {
		t.Fatalf("num: got %d want %d", len(got), want.Num)
	}
	f := got[0]
	if f.Hour != want.First.Hour || f.Minute != want.First.Minute || !approx(f.Price, want.First.Price) || f.Vol != want.First.Vol {
		t.Errorf("first: got %+v want %+v", f, want.First)
	}
}

func TestFinanceInfoParity(t *testing.T) {
	body := loadFixture(t, "finance_info")
	var want struct {
		LiutongGuben    float64 `json:"liutong_guben"`
		ZongGuben       float64 `json:"zong_guben"`
		MeigujingZichan float64 `json:"meigujing_zichan"`
	}
	loadJSON(t, "finance_info", &want)
	got, err := FinanceInfoCmd{}.ParseResponse(body)
	if err != nil {
		t.Fatal(err)
	}
	if !approx(got.LiutongGuben, want.LiutongGuben) {
		t.Errorf("liutong_guben: got %v want %v", got.LiutongGuben, want.LiutongGuben)
	}
	if !approx(got.ZongGuben, want.ZongGuben) {
		t.Errorf("zong_guben: got %v want %v", got.ZongGuben, want.ZongGuben)
	}
	if !approx(got.MeigujingZichan, want.MeigujingZichan) {
		t.Errorf("meigujing_zichan: got %v want %v", got.MeigujingZichan, want.MeigujingZichan)
	}
}

func TestXdxrInfoParity(t *testing.T) {
	body := loadFixture(t, "xdxr_info")
	var want struct {
		Num   int `json:"num_records"`
		First struct {
			Year     int `json:"year"`
			Month    int `json:"month"`
			Day      int `json:"day"`
			Category int `json:"category"`
		} `json:"first"`
	}
	loadJSON(t, "xdxr_info", &want)
	got, err := XdxrInfoCmd{}.ParseResponse(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != want.Num {
		t.Fatalf("num: got %d want %d", len(got), want.Num)
	}
	f := got[0]
	if f.Year != want.First.Year || f.Month != want.First.Month || f.Day != want.First.Day || f.Category != want.First.Category {
		t.Errorf("first: got %+v want %+v", f, want.First)
	}
}

func TestCompanyInfoCategoryParity(t *testing.T) {
	body := loadFixture(t, "company_info_category")
	var want struct {
		Num   int `json:"num_records"`
		First struct {
			Filename string `json:"filename"`
			Start    uint32 `json:"start"`
			Length   uint32 `json:"length"`
		} `json:"first"`
	}
	loadJSON(t, "company_info_category", &want)
	got, err := CompanyInfoCategoryCmd{}.ParseResponse(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != want.Num {
		t.Fatalf("num: got %d want %d", len(got), want.Num)
	}
	f := got[0]
	if f.Filename != want.First.Filename || f.Start != want.First.Start || f.Length != want.First.Length {
		t.Errorf("first: got %+v want %+v", f, want.First)
	}
}

func TestCompanyInfoContentParity(t *testing.T) {
	body := loadFixture(t, "company_info_content")
	var want struct {
		Length int    `json:"length"`
		Prefix string `json:"prefix"`
	}
	loadJSON(t, "company_info_content", &want)
	got, err := CompanyInfoContentCmd{}.ParseResponse(body)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got, want.Prefix) {
		t.Errorf("prefix mismatch:\n got %q\nwant %q", got[:min(len(got), 80)], want.Prefix)
	}
}
