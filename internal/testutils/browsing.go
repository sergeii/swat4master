package testutils

import (
	"encoding/binary"
	"net"
	"strconv"
	"strings"

	"github.com/sergeii/swat4master/pkg/binutils"
	gscrypt "github.com/sergeii/swat4master/pkg/gamespy/crypt"
	"github.com/sergeii/swat4master/pkg/random"
)

func GenBrowserChallenge(l uint) []byte {
	challenge := make([]byte, l)
	for i := range challenge {
		challenge[i] = uint8(random.RandInt(1, 255)) // nolint:gosec
	}
	return challenge
}

func WithBrowserChallengeLength(l int) func([]byte) int {
	return func(_ []byte) int {
		return l
	}
}

func GenBrowserChallenge8() []byte {
	return GenBrowserChallenge(8)
}

func CalcReqLength(req []byte) int {
	return len(req)
}

func PackBrowserRequest(
	fields []string,
	filters string,
	options []byte,
	getChallenge func() []byte,
	getLengthFunc func([]byte) int,
) []byte {
	req := make([]byte, 0)
	req = append(req, []byte{0x00, 0x00}...) // first two bytes are reserved for request length declaration
	req = append(req, 0x00)                  // request type - always zero
	req = append(req, 0x01)                  // protocol version, always 0x01
	req = append(req, 0x03)                  // encoding version, always 0x03

	// gamespy game version, int 32, always 0
	req = append(req, []byte{0x00, 0x00, 0x00, 0x00}...)

	// gamespy game identifier and game name
	// always seem to be equal for swat
	req = append(req, []byte("swat4")...)
	req = append(req, 0x00)
	req = append(req, []byte("swat4")...)
	req = append(req, 0x00)

	// 8 byte challenge key, random
	req = append(req, getChallenge()...)

	// filters
	req = append(req, []byte(filters)...) // can as well be an empty string
	req = append(req, 0x00)

	fieldsJoined := strings.Join(fields, "\\")
	// fields declaration always start with a backslash
	req = append(req, '\\')
	req = append(req, []byte(fieldsJoined)...)
	req = append(req, 0x00)

	// last 4 bytes, int 32, options; options bitmask - always 0 for swat if the request is coming from the game,
	// since it requests a plain server list. However, tools like gslist send 1 (server list with fields)
	req = append(req, options...)

	// calculate the length and insert the number into the first two bytes as an unsigned short
	binary.BigEndian.PutUint16(req[:2], uint16(getLengthFunc(req))) // nolint:gosec

	return req
}

func UnpackServerList(resp []byte) []map[string]string {
	fieldCount := int(resp[6])
	fields := make([]string, 0, fieldCount)
	unparsed := resp[8:]
	for range fieldCount {
		field, rem := binutils.ConsumeCString(unparsed)
		// consume extra null byte at the end of the field
		unparsed = rem[1:]
		fields = append(fields, string(field))
	}
	servers := make([]map[string]string, 0)
	for unparsed[0] == 0x51 {
		server := map[string]string{
			"host": net.IPv4(unparsed[1], unparsed[2], unparsed[3], unparsed[4]).String(),
			"port": strconv.FormatUint(uint64(binary.BigEndian.Uint16(unparsed[5:7])), 10),
		}
		unparsed = unparsed[7:]
		for i := range fields {
			unparsed = unparsed[1:] // skip leading 0xff
			fieldValue, rem := binutils.ConsumeCString(unparsed)
			server[fields[i]] = string(fieldValue)
			unparsed = rem
		}
		servers = append(servers, server)
	}
	return servers
}

func SendBrowserRequest(address string, filters string) []byte {
	var gameKey [6]byte
	var challenge [8]byte
	copy(challenge[:], GenBrowserChallenge8())
	req := PackBrowserRequest(
		[]string{
			"hostname", "maxplayers", "gametype",
			"gamevariant", "mapname", "hostport",
			"password", "gamever", "statsenabled",
		},
		filters,
		[]byte{0x00, 0x00, 0x00, 0x00},
		func() []byte {
			return challenge[:]
		},
		CalcReqLength,
	)
	resp := SendTCP(address, req)
	copy(gameKey[:], "tG3j8c")
	return gscrypt.Decrypt(gameKey, challenge, resp)
}
