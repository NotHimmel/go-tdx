package transport

import "fmt"

// CommError 表示传输层通信错误（未连接/写失败/读失败/解压失败）。
// 出现该错误说明连接内的字节流状态已不可信，调用方（如 Pool）应弃用
// 该连接并重拨；业务解析错误不属于此类。
type CommError struct {
	Op  string // not-connected / write / read-header / read-body / decompress
	Err error
}

func (e *CommError) Error() string {
	if e.Err == nil {
		return fmt.Sprintf("通信错误(%s)", e.Op)
	}
	return fmt.Sprintf("通信错误(%s): %v", e.Op, e.Err)
}

func (e *CommError) Unwrap() error { return e.Err }
