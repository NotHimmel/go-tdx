package command

import "encoding/hex"

// SetupCommands 握手命令原始字节（pytdx 移植，真实服务器验证）。
// 连接建立后必须按序发送，每条均需读取并丢弃响应。
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
		must("0c0218940001030003000d0002"),
		must("0c031899000120002000db0fd5d0c9ccd6a4a8af0000008fc22540130000d500c9ccbdf0d7ea00000002"),
	}
}()
