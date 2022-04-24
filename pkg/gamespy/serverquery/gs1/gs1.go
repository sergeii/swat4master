package gs1

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"sort"
	"strconv"
	"time"

	"github.com/rs/zerolog/log"
	"golang.org/x/text/encoding/charmap"

	"github.com/sergeii/swat4master/pkg/binutils"
)

var ErrResponseIncomplete = errors.New("response payload is not complete")
var ErrResponseMalformed = errors.New("response payload contains invalid data")

type QueryVersion int

const (
	VerUnknown QueryVersion = iota
	VerVanilla
	VerAM
	VerGS1
)

func (ver QueryVersion) String() string {
	switch ver {
	case VerUnknown:
		return "unknown"
	case VerGS1:
		return "gs1"
	case VerAM:
		return "am"
	case VerVanilla:
		return "vanilla"
	default:
		return fmt.Sprintf("%d", ver)
	}
}

type Response struct {
	Fields     map[string]string
	Players    []map[string]string
	Objectives []map[string]string
	Version    QueryVersion
}

type Param struct {
	Name  []byte
	Value []byte
}

var Blank Response

const bufferSize = 2048

func Query(ctx context.Context, addr netip.AddrPort, timeout time.Duration) (Response, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	conn, err := net.DialUDP("udp", nil, net.UDPAddrFromAddrPort(addr))
	if err != nil {
		return Blank, err
	}

	closing := make(chan struct{})
	defer func() {
		close(closing)
		conn.Close() // nolint: errcheck
	}()

	go func() {
		select {
		case <-ctx.Done():
			conn.SetReadDeadline(time.Now()) // nolint: errcheck
		case <-closing:
		}
	}()

	log.Debug().Stringer("src", conn.RemoteAddr()).Msg("Connected to server")

	_, err = conn.Write([]byte("\\status\\"))
	if err != nil {
		return Blank, err
	}

	return getResponse(conn)
}

func getResponse(conn *net.UDPConn) (Response, error) {
	packets := make([][]byte, 0, 4)
	for {
		buffer := make([]byte, bufferSize)
		n, raddr, err := conn.ReadFromUDP(buffer)
		if err != nil {
			return Blank, err
		}

		packet := buffer[:n]
		if len(packet) == 0 {
			return Blank, ErrResponseIncomplete
		}

		log.Debug().Stringer("src", raddr).Int("size", n).Msg("Received data")

		packets = append(packets, packet)
		payload, version, err := collectPayload(packets)
		if err != nil {
			if errors.Is(err, ErrResponseIncomplete) {
				continue
			}
			return Blank, err
		}

		response, err := expandPayload(payload, version)
		if err != nil {
			return Blank, err
		}

		return response, nil
	}
}

func inspectStatusResponse(paramVal []byte) (int, QueryVersion, error) {
	intValue, err := strconv.Atoi(string(paramVal))
	if err != nil {
		return -1, VerUnknown, err
	}
	if intValue < 0 {
		return -1, VerUnknown, ErrResponseMalformed
	}
	// normally, statusresponse field is a sign of AMMod's response
	return intValue + 1, VerAM, nil
}

func inspectQueryID(paramVal []byte) (int, QueryVersion, error) {
	intValue, err := strconv.Atoi(string(paramVal))
	switch {
	case err != nil: // 1.0 or 1.1
		// point-notation of queryid is a sign of vanilla response
		return 1, VerVanilla, nil // nolint: nilerr
	case intValue <= 0:
		return -1, VerUnknown, ErrResponseMalformed
	default:
		// packets are properly indexed by GS1 mod
		return intValue, VerGS1, nil
	}
}

func inspectPacket(packet []byte) (int, bool, QueryVersion, error) {
	var err error

	version := VerUnknown
	order := -1
	isFinal := false

	for _, param := range parseParams(packet) {
		// this is the final packet, so we should expect as many packets
		// as the number of the final packet
		if bytes.Equal(param.Name, []byte("final")) {
			isFinal = true
		}
		// the order for this packet has already been determined
		if order != -1 {
			continue
		}
		// \statusresponse\1
		if bytes.Equal(param.Name, []byte("statusresponse")) {
			order, version, err = inspectStatusResponse(param.Value)
			if err != nil {
				return -1, false, VerUnknown, err
			}
		} else if bytes.Equal(param.Name, []byte("queryid")) { // \queryid\1 or \queryid\1.1
			order, version, err = inspectQueryID(param.Value)
			if err != nil {
				return -1, false, VerUnknown, err
			}
		}
	}

	return order, isFinal, version, nil
}

func collectPayload(packets [][]byte) ([]byte, QueryVersion, error) {
	var version QueryVersion

	payloadSize := 0
	count := -1
	ordered := make(map[int][]byte)

	for _, packet := range packets {
		order, isFinal, ver, err := inspectPacket(packet)
		if err != nil {
			return nil, VerUnknown, err
		}
		if order == -1 {
			return nil, VerUnknown, ErrResponseMalformed
		} else if isFinal {
			count = order
		}
		version = ver
		ordered[order] = packet
		payloadSize += len(packet)
	}

	if count == -1 || count != len(ordered) {
		return nil, VerUnknown, ErrResponseIncomplete
	}

	payload := make([]byte, 0, payloadSize)
	for i := 1; i <= count; i++ {
		packet := normalizePacket(ordered[i])
		payload = append(payload, packet...)
	}

	return payload, version, nil
}

func expandPayload(payload []byte, version QueryVersion) (Response, error) {
	objectives := make([]map[string]string, 0)
	playersByID := make(map[int]map[string]string)
	fields := make(map[string]string)

	for _, param := range parseParams(payload) {
		// \obj_Neutralize_All_Enemies\0\
		if bytes.Equal(param.Name[:4], []byte("obj_")) {
			if len(param.Name) > 4 {
				objectives = append(objectives, map[string]string{
					"name":   string(param.Name[4:]),
					"status": string(param.Value),
				})
			}
			continue
		}
		// \player_2\James_Bond_007\
		if i := bytes.IndexByte(param.Name, '_'); i != -1 {
			if len(param.Name) > i+1 {
				id, err := strconv.Atoi(string(param.Name[i+1:]))
				if err != nil {
					return Blank, ErrResponseMalformed
				}
				if _, ok := playersByID[id]; !ok {
					playersByID[id] = make(map[string]string)
				}
				playersByID[id][string(param.Name[:i])] = latin1(param.Value)
			}
			continue
		}
		fields[string(param.Name)] = latin1(param.Value)
	}

	players := collectPlayers(playersByID)

	return Response{
		Fields:     fields,
		Objectives: objectives,
		Players:    players,
		Version:    version,
	}, nil
}

func parseParams(data []byte) []Param {
	var field []byte
	fields := make([][]byte, 0, 16)
	unparsed := data[1:] // skip leading \
	for unparsed != nil {
		field, unparsed = binutils.ConsumeString(unparsed, '\\')
		fields = append(fields, field)
	}
	params := make([]Param, 0, (len(fields)/2)+1)
	for i := 1; i < len(fields); i += 2 {
		params = append(params, Param{fields[i-1], fields[i]})
	}
	return params
}

func collectPlayers(playersByID map[int]map[string]string) []map[string]string {
	if len(playersByID) == 0 {
		return nil
	}
	players := make([]map[string]string, 0, len(playersByID))
	sortedIDs := make([]int, 0, len(playersByID))
	for id := range playersByID {
		sortedIDs = append(sortedIDs, id)
	}
	sort.Ints(sortedIDs)
	for _, id := range sortedIDs {
		players = append(players, playersByID[id])
	}
	return players
}

func normalizePacket(packet []byte) []byte {
	// remove the leading \statusresponse\ field and its value
	if bytes.Equal(packet[:16], []byte("\\statusresponse\\")) {
		packet = packet[16:]
		if i := bytes.IndexByte(packet, '\\'); i != -1 {
			packet = packet[i:]
		}
	}
	// remove the trailing \eof\ token
	if bytes.Equal(packet[len(packet)-5:], []byte("\\eof\\")) {
		packet = packet[:len(packet)-5]
	}
	return packet
}

func latin1(bytes []byte) string {
	encoded, _ := charmap.ISO8859_1.NewDecoder().Bytes(bytes) // nolint: nosnakecase
	return string(encoded)
}
