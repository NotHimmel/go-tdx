package codec

import (
	"math"
	"strings"

	"github.com/NotHimmel/go-tdx/model"
)

// GetNoLimitWindowDays 返回上市初期不设涨跌幅限制的交易日窗口（0/1/5）。
func GetNoLimitWindowDays(market model.Market, code, name string) int {
	if isIndexLike(market, code, name) {
		return 0
	}
	if hasPrefix(code, "43", "83", "87", "92") {
		return 1
	}
	if market == model.MarketSH && hasPrefix(code, "60", "68") {
		return 5
	}
	if market == model.MarketSZ && hasPrefix(code, "00", "30") {
		return 5
	}
	return 0
}

func isIndexLike(market model.Market, code, name string) bool {
	if market == model.MarketSH && hasPrefix(code, "000", "880", "881", "882", "883", "884", "885", "999") {
		return true
	}
	if market == model.MarketSZ && hasPrefix(code, "395", "399") {
		return true
	}
	return strings.Contains(name, "指数") || strings.Contains(name, "板块")
}

// ComputePriceLimits 根据板块规则计算涨跌停价。
// 无限制或无法判断时返回 (nil, nil)。listedDays>0 时按上市初期无限制规则优先。
func ComputePriceLimits(market model.Market, code, name string, preClose float64, listedDays int) (*float64, *float64) {
	if preClose <= 0 {
		return nil, nil
	}
	if isIndexLike(market, code, name) {
		return nil, nil
	}
	noLimit := GetNoLimitWindowDays(market, code, name)
	if listedDays > 0 && listedDays <= noLimit {
		return nil, nil
	}

	upper := strings.ToUpper(name)
	limitPct := 0.10
	switch {
	case strings.Contains(upper, "ST"):
		limitPct = 0.05
	case hasPrefix(code, "688", "300", "301"):
		limitPct = 0.20
	case hasPrefix(code, "43", "83", "87", "92"):
		limitPct = 0.30
	}

	up := roundPrice(preClose * (1 + limitPct))
	down := roundPrice(preClose * (1 - limitPct))
	return &up, &down
}

func roundPrice(p float64) float64 {
	return math.Round((p+0.00001)*100) / 100
}

func hasPrefix(s string, prefixes ...string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}
