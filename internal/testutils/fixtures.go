package testutils

import (
	"encoding/binary"
	"log"
	"math/rand"
	"net"
)

func GenRandomIP() net.IP {
	for i := range 10 {
		rand32int := make([]byte, 4)
		binary.BigEndian.PutUint32(rand32int, rand.Uint32()) // nolint: gosec
		randIP := net.IPv4(rand32int[0], rand32int[1], rand32int[2], rand32int[3])
		if !randIP.IsGlobalUnicast() || randIP.IsPrivate() {
			continue
		}
		if i > 4 {
			log.Printf("generated random IP %s with %d attempts", randIP, i+1)
		}
		return randIP
	}
	panic("unable to generate random IP")
}
