package command

import (
	"encoding/hex"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/NotHimmel/go-tdx/model"
)

// loadFixture 读取 testdata/<name>.hex（解压后 body 的 hex）。
func loadFixture(t *testing.T, name string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "testdata", name+".hex"))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	b, err := hex.DecodeString(strings.TrimSpace(string(raw)))
	if err != nil {
		t.Fatalf("decode hex %s: %v", name, err)
	}
	return b
}

func loadJSON(t *testing.T, name string, out any) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "testdata", name+".json"))
	if err != nil {
		t.Fatalf("read json %s: %v", name, err)
	}
	if err := json.Unmarshal(raw, out); err != nil {
		t.Fatalf("unmarshal json %s: %v", name, err)
	}
}

func approx(a, b float64) bool { return math.Abs(a-b) < 1e-6 }

func TestSecurityBarsParity(t *testing.T) {
	body := loadFixture(t, "security_bars")

	var want struct {
		NumRecords int `json:"num_records"`
		First      struct {
			Open  float64 `json:"open"`
			High  float64 `json:"high"`
			Low   float64 `json:"low"`
			Close float64 `json:"close"`
			Vol   float64 `json:"vol"`
		} `json:"first"`
	}
	loadJSON(t, "security_bars", &want)

	cmd := SecurityBarsCmd{Category: model.Day}
	bars, err := cmd.ParseResponse(body)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if len(bars) != want.NumRecords {
		t.Fatalf("num_records: got %d, want %d", len(bars), want.NumRecords)
	}
	f := bars[0]
	if !approx(f.Open, want.First.Open) {
		t.Errorf("open: got %v, want %v", f.Open, want.First.Open)
	}
	if !approx(f.High, want.First.High) {
		t.Errorf("high: got %v, want %v", f.High, want.First.High)
	}
	if !approx(f.Low, want.First.Low) {
		t.Errorf("low: got %v, want %v", f.Low, want.First.Low)
	}
	if !approx(f.Close, want.First.Close) {
		t.Errorf("close: got %v, want %v", f.Close, want.First.Close)
	}
	if !approx(f.Vol, want.First.Vol) {
		t.Errorf("vol: got %v, want %v", f.Vol, want.First.Vol)
	}
}
