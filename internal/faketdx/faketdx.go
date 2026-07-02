// Package faketdx 提供进程内 fake TDX 服务器，讲 16 字节响应帧协议，
// 供 pool/transport 单测注入丢响应、断连、延迟等故障。
package faketdx

import (
	"encoding/binary"
	"net"
	"sync/atomic"
	"testing"
	"time"
)

// Response 控制对一条数据请求的应答行为。
type Response struct {
	Body      []byte        // 应答 body（不压缩：zipsize == unzipsize）
	Drop      bool          // 读请求但不应答（模拟服务器丢响应）
	CloseConn bool          // 读请求后直接断开（模拟服务器掐连接）
	Delay     time.Duration // 应答前延迟
}

// Server 进程内 fake TDX 服务器。每连接前 3 条请求视为握手，自动回空帧。
type Server struct {
	ln       net.Listener
	handler  func(req []byte) Response
	accepted atomic.Int64 // 接受过的连接数
	requests atomic.Int64 // 收到的数据请求数（不含握手）
}

// Start 启动服务器，t.Cleanup 自动关闭。handler 为 nil 时一律回空 body。
func Start(t *testing.T, handler func(req []byte) Response) *Server {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("faketdx listen: %v", err)
	}
	s := &Server{ln: ln, handler: handler}
	go s.acceptLoop()
	t.Cleanup(func() { ln.Close() })
	return s
}

// Addr 返回 "127.0.0.1:port"，可直接传给 tdx.NewPool / NewWithTimeout。
func (s *Server) Addr() string { return s.ln.Addr().String() }

// Accepted 返回累计接受的连接数。
func (s *Server) Accepted() int { return int(s.accepted.Load()) }

// Requests 返回累计数据请求数（不含握手）。
func (s *Server) Requests() int { return int(s.requests.Load()) }

func (s *Server) acceptLoop() {
	for {
		c, err := s.ln.Accept()
		if err != nil {
			return
		}
		s.accepted.Add(1)
		go s.serve(c)
	}
}

// serve 假设一次 Read 恰好读到一条完整请求。客户端 Execute 严格
// 一问一答且请求为单次小包写入，loopback 上该假设成立。
func (s *Server) serve(c net.Conn) {
	defer c.Close()
	buf := make([]byte, 4096)
	setup := 0
	for {
		n, err := c.Read(buf)
		if err != nil {
			return
		}
		req := append([]byte(nil), buf[:n]...)
		if setup < 3 { // Connect() 的 3 条握手命令
			setup++
			writeFrame(c, req, nil)
			continue
		}
		s.requests.Add(1)
		resp := Response{}
		if s.handler != nil {
			resp = s.handler(req)
		}
		if resp.Delay > 0 {
			time.Sleep(resp.Delay)
		}
		if resp.CloseConn {
			return
		}
		if resp.Drop {
			continue
		}
		writeFrame(c, req, resp.Body)
	}
}

// writeFrame 组 16 字节帧头 + 未压缩 body（zipsize == unzipsize）。
func writeFrame(c net.Conn, req, body []byte) {
	h := make([]byte, 16)
	binary.LittleEndian.PutUint32(h[0:], 7654321)
	var seq uint32
	if len(req) >= 5 {
		seq = binary.LittleEndian.Uint32(req[1:5])
	}
	binary.LittleEndian.PutUint32(h[4:], seq)
	binary.LittleEndian.PutUint32(h[8:], 0)
	binary.LittleEndian.PutUint16(h[12:], uint16(len(body)))
	binary.LittleEndian.PutUint16(h[14:], uint16(len(body)))
	c.Write(append(h, body...))
}
