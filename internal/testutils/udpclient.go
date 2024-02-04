package testutils

import (
	"net"
	"time"
)

type UDPClient struct {
	conn        net.Conn
	readTimeout time.Duration
	bufSize     int
	LocalAddr   *net.UDPAddr
}

func NewUDPClient(address string, bufSize int, readTimeout time.Duration) *UDPClient {
	conn, _ := net.Dial("udp", address)
	return &UDPClient{
		conn:        conn,
		readTimeout: readTimeout,
		bufSize:     bufSize,
		LocalAddr:   conn.LocalAddr().(*net.UDPAddr), // nolint: forcetypeassert
	}
}

func (c *UDPClient) Send(req []byte) ([]byte, error) {
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

func (c *UDPClient) Close() {
	c.conn.Close() // nolint: errcheck
}

func SendUDP(address string, req []byte) []byte {
	c := NewUDPClient(address, 1024, 0)
	defer c.Close()
	resp, err := c.Send(req)
	if err != nil {
		panic(err)
	}
	return resp
}
