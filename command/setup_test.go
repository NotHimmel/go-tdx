package command

import (
	"encoding/hex"
	"testing"
)

func TestSetupCommandsOnlyHello1(t *testing.T) {
	if len(SetupCommands) != 1 {
		t.Fatalf("握手应只包含 Hello1，得到 %d 条命令", len(SetupCommands))
	}
	want, err := hex.DecodeString("0c0218930001030003000d0001")
	if err != nil {
		t.Fatal(err)
	}
	if got := hex.EncodeToString(SetupCommands[0]); got != hex.EncodeToString(want) {
		t.Fatalf("Hello1 不符: got %s want %s", got, hex.EncodeToString(want))
	}
}
