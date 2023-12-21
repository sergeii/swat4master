package testutils

import (
	"encoding/binary"
	"log"
	"math/rand"
	"net"

	"github.com/sergeii/swat4master/pkg/random"
)

func GenRandomIP() net.IP {
	var i int
	for i = 0; i < 10; i++ {
		rand32int := make([]byte, 4)
		binary.BigEndian.PutUint32(rand32int, rand.Uint32()) // nolint: gosec
		randIP := net.IPv4(rand32int[0], rand32int[1], rand32int[2], rand32int[3])
		if !randIP.IsGlobalUnicast() || randIP.IsPrivate() {
			continue
		}
		log.Printf("generated random IP %s with %d attempts", randIP, i+1)
		return randIP
	}
	panic("unable to generate random IP")
}

func StandardAddr() (net.IP, int) {
	return net.ParseIP("1.1.1.1"), 10481
}

func WithRandomAddr() func() (net.IP, int) {
	return func() (net.IP, int) {
		randPort := random.RandInt(1, 65535)
		return GenRandomIP(), randPort
	}
}

func WithCustomAddr(ip string, port int) func() (net.IP, int) {
	return func() (net.IP, int) {
		return net.ParseIP(ip), port
	}
}
