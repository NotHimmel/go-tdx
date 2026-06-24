package codec

import (
	"testing"

	"github.com/NotHimmel/go-tdx/model"
)

func TestComputePriceLimits(t *testing.T) {
	cases := []struct {
		name           string
		market         model.Market
		code, secName  string
		preClose       float64
		listedDays     int
		wantUp, wantDn float64
		wantNil        bool
	}{
		{"主板10%", model.MarketSH, "600000", "浦发银行", 10.0, 0, 11.0, 9.0, false},
		{"创业板20%", model.MarketSZ, "300001", "特锐德", 10.0, 0, 12.0, 8.0, false},
		{"科创板20%", model.MarketSH, "688001", "华兴源创", 10.0, 0, 12.0, 8.0, false},
		{"北交所30%", model.MarketBJ, "830799", "艾融软件", 10.0, 0, 13.0, 7.0, false},
		{"ST5%", model.MarketSH, "600001", "ST示例", 10.0, 0, 10.5, 9.5, false},
		{"指数无限制", model.MarketSH, "000001", "上证指数", 3000.0, 0, 0, 0, true},
		{"上市初期无限制", model.MarketSH, "600999", "新股", 10.0, 3, 0, 0, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			up, dn := ComputePriceLimits(tc.market, tc.code, tc.secName, tc.preClose, tc.listedDays)
			if tc.wantNil {
				if up != nil || dn != nil {
					t.Fatalf("want nil, got %v/%v", up, dn)
				}
				return
			}
			if up == nil || dn == nil {
				t.Fatalf("got nil, want %v/%v", tc.wantUp, tc.wantDn)
			}
			if *up != tc.wantUp || *dn != tc.wantDn {
				t.Errorf("got %.2f/%.2f want %.2f/%.2f", *up, *dn, tc.wantUp, tc.wantDn)
			}
		})
	}
}
