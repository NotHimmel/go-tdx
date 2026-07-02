// Package transport 提供同步 TCP 连接、握手与帧收发。
package transport

import (
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/NotHimmel/go-tdx/codec"
	"github.com/NotHimmel/go-tdx/command"
)

// Conn 通达信 TCP 连接（goroutine 安全：execute 串行化）。
type Conn struct {
	Host    string
	Port    int
	Timeout time.Duration

	mu   sync.Mutex
	sock net.Conn
}

// NewConn 创建连接对象（未连接）。port<=0 用 DefaultPort，timeout<=0 用 8s。
func NewConn(host string, port int, timeout time.Duration) *Conn {
	if port <= 0 {
		port = DefaultPort
	}
	if timeout <= 0 {
		timeout = 8 * time.Second
	}
	return &Conn{Host: host, Port: port, Timeout: timeout}
}

// Connect 建立 TCP 连接并完成握手（发送 3 条 setup 命令，丢弃响应）。
func (c *Conn) Connect() error {
	addr := net.JoinHostPort(c.Host, fmt.Sprintf("%d", c.Port))
	sock, err := net.DialTimeout("tcp", addr, c.Timeout)
	if err != nil {
		return fmt.Errorf("无法连接 %s: %w", addr, err)
	}
	c.sock = sock
	if err := c.sendSetup(); err != nil {
		sock.Close()
		c.sock = nil
		return err
	}
	return nil
}

// Close 关闭连接。
func (c *Conn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.sock != nil {
		err := c.sock.Close()
		c.sock = nil
		return err
	}
	return nil
}

// Execute 发送请求，接收并解压响应，返回解压后的 body。
// 调用方用 cmd.ParseResponse(body) 解析。
// 传输层失败返回 *CommError，此时连接状态不可信，应弃用重拨。
func (c *Conn) Execute(request []byte) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.sock == nil {
		return nil, &CommError{Op: "not-connected"}
	}
	c.sock.SetDeadline(time.Now().Add(c.Timeout))

	if _, err := c.sock.Write(request); err != nil {
		return nil, &CommError{Op: "write", Err: err}
	}
	headerBuf, err := c.recvExact(codec.HeaderSize)
	if err != nil {
		return nil, &CommError{Op: "read-header", Err: err}
	}
	header, err := codec.ParseHeader(headerBuf)
	if err != nil {
		return nil, &CommError{Op: "read-header", Err: err}
	}
	rawBody, err := c.recvExact(int(header.ZipSize))
	if err != nil {
		return nil, &CommError{Op: "read-body", Err: err}
	}
	body, err := codec.DecompressBody(header, rawBody)
	if err != nil {
		return nil, &CommError{Op: "decompress", Err: err}
	}
	return body, nil
}

func (c *Conn) sendSetup() error {
	for _, cmd := range command.SetupCommands {
		if _, err := c.sock.Write(cmd); err != nil {
			return fmt.Errorf("握手发送失败: %w", err)
		}
		c.sock.SetDeadline(time.Now().Add(c.Timeout))
		hdrBuf, err := c.recvExact(codec.HeaderSize)
		if err != nil {
			// 部分服务器握手无响应，忽略
			continue
		}
		hdr, err := codec.ParseHeader(hdrBuf)
		if err != nil {
			continue
		}
		if hdr.ZipSize > 0 {
			c.recvExact(int(hdr.ZipSize))
		}
	}
	return nil
}

func (c *Conn) recvExact(n int) ([]byte, error) {
	buf := make([]byte, n)
	if _, err := io.ReadFull(c.sock, buf); err != nil {
		return nil, err
	}
	return buf, nil
}
