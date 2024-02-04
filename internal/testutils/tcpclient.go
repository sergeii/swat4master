package testutils

import (
	"net"
	"time"
)

type TCPClient struct {
	conn        net.Conn
	bufSize     int
	readTimeout time.Duration
}

func NewTCPClient(address string, bufSize int, readTimeout time.Duration) *TCPClient {
	conn, err := net.Dial("tcp", address)
	if err != nil {
		panic(err)
	}
	return &TCPClient{
		conn:        conn,
		bufSize:     bufSize,
		readTimeout: readTimeout,
	}
}

func (c *TCPClient) Send(req []byte) ([]byte, error) {
	if _, err := c.conn.Write(req); err != nil {
		return nil, err
	}
	if c.readTimeout > 0 {
		if err := c.conn.SetReadDeadline(time.Now().Add(c.readTimeout)); err != nil {
			return nil, err
		}
	}
	buf := make([]byte, c.bufSize)
	n, err := c.conn.Read(buf)
	if err != nil {
		return nil, err
	}
	return buf[:n], nil
}

func (c *TCPClient) Close() {
	c.conn.Close() // nolint: errcheck
}

func SendTCP(address string, req []byte) []byte {
	c := NewTCPClient(address, 2048, 0)
	defer c.Close()
	resp, err := c.Send(req)
	if err != nil {
		panic(err)
	}
	return resp
}
