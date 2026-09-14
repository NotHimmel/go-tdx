package command

import "encoding/hex"

// SetupCommands 是标准行情服务器的 Hello1 握手。
//
// 2026-07 起，公共服务器仍接受 Hello1，但在客户端继续发送旧版 Setup2/Setup3
// 后会对行情请求返回只有 count 字段的空包。连接建立后只发送 Hello1。
var SetupCommands = func() [][]byte {
	must := func(s string) []byte {
		b, err := hex.DecodeString(s)
		if err != nil {
			panic(err)
		}
		return b
	}
	return [][]byte{
		must("0c0218930001030003000d0001"),
	}
}()
