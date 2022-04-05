package testutils

import (
	"encoding/binary"
	"log"
	"math/rand"
	"net"

	"github.com/sergeii/swat4master/pkg/random"
	"github.com/sergeii/swat4master/pkg/slice"
)

func GenServerParams() map[string]string {
	return map[string]string{
		"localip0":     "192.168.10.72",
		"localip1":     "1.1.1.1",
		"localport":    "10481",
		"gamename":     slice.RandomChoice([]string{"swat4", "swat4xp1"}),
		"hostname":     "Swat4 Server",
		"numplayers":   slice.RandomChoice([]string{"0", "1", "10", "16"}),
		"maxplayers":   "16",
		"gametype":     slice.RandomChoice([]string{"VIP Escort", "Rapid Deployment", "Barricaded Suspects", "CO-OP"}),
		"gamevariant":  slice.RandomChoice([]string{"SWAT 4", "SEF", "SWAT 4X"}),
		"mapname":      slice.RandomChoice([]string{"A-Bomb Nightclub", "Food Wall Restaurant", "-EXP- FunTime Amusements"}),
		"hostport":     "10480",
		"password":     slice.RandomChoice([]string{"0", "1"}),
		"statsenabled": slice.RandomChoice([]string{"0", "1"}),
		"gamever":      slice.RandomChoice([]string{"1.0", "1.1"}),
	}
}

func GenExtraServerParams(extra map[string]string) map[string]string {
	params := GenServerParams()
	for k, v := range extra {
		params[k] = v
	}
	return params
}

func WithServerParams(params map[string]string) func() map[string]string {
	return func() map[string]string {
		return params
	}
}

func WithExtraServerParams(extra map[string]string) func() map[string]string {
	return func() map[string]string {
		return GenExtraServerParams(extra)
	}
}

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
	random.Seed() // nolint: errcheck
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
