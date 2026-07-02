package transport

import (
	"errors"
	"testing"
	"time"
)

// 未连接时 Execute 必须返回 *CommError（Pool 依赖该分型决定重连）。
func TestExecuteNotConnectedIsCommError(t *testing.T) {
	c := NewConn("127.0.0.1", 1, time.Second)
	_, err := c.Execute([]byte{0x0c})
	if err == nil {
		t.Fatal("期望错误，得到 nil")
	}
	var ce *CommError
	if !errors.As(err, &ce) {
		t.Fatalf("期望 *CommError，得到 %T: %v", err, err)
	}
	if ce.Op != "not-connected" {
		t.Fatalf("期望 Op=not-connected，得到 %q", ce.Op)
	}
}

// CommError 的文案与 Unwrap 链。
func TestCommErrorMessageAndUnwrap(t *testing.T) {
	inner := errors.New("EOF")
	err := error(&CommError{Op: "read-header", Err: inner})
	var ce *CommError
	if !errors.As(err, &ce) {
		t.Fatal("errors.As 应命中 *CommError")
	}
	if got := ce.Error(); got != "通信错误(read-header): EOF" {
		t.Fatalf("错误文案不符: %q", got)
	}
	if !errors.Is(err, inner) {
		t.Fatal("Unwrap 链应命中 inner error")
	}
}
