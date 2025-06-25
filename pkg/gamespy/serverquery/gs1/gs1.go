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

	"golang.org/x/text/encoding/charmap"
)

var (
	ErrResponseIncomplete = errors.New("response payload is not complete")
	ErrResponseMalformed  = errors.New("response payload contains invalid data")
)

type QueryVersion int

const (
	VerUnknown QueryVersion = iota
	VerVanilla
	VerAM
	VerGS1
)

const (
	FINAL = "\\final\\"
	EOF   = "\\eof\\"
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

type fragment struct {
	isFinal bool
	order   int
	version QueryVersion
	data    []byte
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

	_, err = conn.Write([]byte("\\status\\"))
	if err != nil {
		return Blank, err
	}

	return getResponse(conn)
}

func getResponse(conn *net.UDPConn) (Response, error) {
	fragments := make([][]byte, 0, 4)
	for {
		buffer := make([]byte, bufferSize)
		n, _, err := conn.ReadFromUDP(buffer)
		if err != nil {
			return Blank, err
		}

		rawFragment := buffer[:n]
		if len(rawFragment) == 0 {
			return Blank, ErrResponseIncomplete
		}

		fragments = append(fragments, rawFragment)
		payload, version, err := collectPayload(fragments)
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
		return -1, VerUnknown, fmt.Errorf("%w: failed to parse statusresponse value", ErrResponseMalformed)
	}
	if intValue < 0 {
		return -1, VerUnknown, fmt.Errorf("%w: statusresponse value is negative", ErrResponseMalformed)
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
		return -1, VerUnknown, fmt.Errorf("%w: queryid value is not positive", ErrResponseMalformed)
	default:
		// GS1 fragments are zero indexed
		return intValue, VerGS1, nil
	}
}

func inspectFragment(payload []byte) (fragment, error) {
	// vanilla response
	if bytes.HasSuffix(payload, []byte("\\queryid\\1.1")) {
		inspected := fragment{
			isFinal: true,
			version: VerVanilla,
			order:   1,
			data:    payload,
		}
		return inspected, nil
	}

	// AMMod response
	if bytes.HasPrefix(payload, []byte("\\statusresponse\\")) {
		return inspectAmModFragment(payload)
	}

	// Treat any other payload as GS1
	return inspectGS1Fragment(payload)
}

func inspectAmModFragment(payload []byte) (fragment, error) {
	var isFinal bool
	var param Param
	var err error

	// drop the trailing \eof\ token
	payload = bytes.TrimSuffix(payload, []byte(EOF))

	// determine the fragment number from the leading statusresponse field
	// typically present in the AdminMod's query response
	param, payload, err = consumeParam(payload)
	if err != nil || !bytes.Equal(param.Name, []byte("statusresponse")) {
		return fragment{}, fmt.Errorf("%w: statusresponse field is missing or malformed", ErrResponseMalformed)
	}

	order, version, err := inspectStatusResponse(param.Value)
	if err != nil {
		return fragment{}, err
	}

	// Some AMMod responses may not have a trailing queryid field.
	// Try to remove it from the fragment body
	if lastParam, rest, consumeErr := consumeParamFromRight(payload); consumeErr == nil {
		if bytes.Equal(lastParam.Name, []byte("queryid")) {
			payload = rest
		}
	}

	if bytes.HasSuffix(payload, []byte(FINAL)) {
		isFinal = true
		payload = bytes.TrimSuffix(payload, []byte(FINAL))
	}

	inspected := fragment{
		isFinal: isFinal,
		version: version,
		order:   order,
		data:    payload,
	}

	return inspected, nil
}

func inspectGS1Fragment(payload []byte) (fragment, error) {
	var isFinal bool
	var param Param
	var err error

	if bytes.HasSuffix(payload, []byte(FINAL)) {
		isFinal = true
		payload = bytes.TrimSuffix(payload, []byte(FINAL))
	}

	// determine the fragment number from the trailing queryid field and its numeric value
	param, payload, err = consumeParamFromRight(payload)
	if err != nil || !bytes.Equal(param.Name, []byte("queryid")) {
		return fragment{}, fmt.Errorf("%w: queryid field is missing or malformed", ErrResponseMalformed)
	}

	order, version, err := inspectQueryID(param.Value)
	if err != nil {
		return fragment{}, err
	}

	inspected := fragment{
		isFinal: isFinal,
		version: version,
		order:   order,
		data:    payload,
	}

	return inspected, nil
}

func collectPayload(fragments [][]byte) ([]byte, QueryVersion, error) {
	var version QueryVersion

	payloadSize := 0
	count := -1
	ordered := make(map[int][]byte)

	for _, rawFragment := range fragments {
		inspected, err := inspectFragment(rawFragment)
		if err != nil {
			return nil, VerUnknown, err
		}
		if inspected.order == -1 {
			return nil, VerUnknown, fmt.Errorf("%w: fragment order missing", ErrResponseMalformed)
		}
		if inspected.isFinal {
			count = inspected.order
		}
		version = inspected.version
		ordered[inspected.order] = inspected.data
		payloadSize += len(inspected.data)
	}

	if count == -1 || count != len(ordered) {
		return nil, VerUnknown, ErrResponseIncomplete
	}

	payload := make([]byte, 0, payloadSize)
	for i := 1; i <= count; i++ {
		payload = append(payload, ordered[i]...)
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
			if len(param.Name) <= i+1 {
				continue
			}
			id, err := strconv.Atoi(string(param.Name[i+1:]))
			if err != nil {
				return Blank, ErrResponseMalformed
			}
			if _, ok := playersByID[id]; !ok {
				playersByID[id] = make(map[string]string)
			}
			playersByID[id][string(param.Name[:i])] = latin1(param.Value)
			continue
		}
		// \hostname\Swat4 Server\
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

	for unparsed := data; unparsed != nil; {
		field, unparsed = consumeField(unparsed)
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

func consumeField(payload []byte) ([]byte, []byte) {
	if len(payload) == 0 {
		return nil, nil
	}
	consumed := payload[1:]
	// \field1\field2
	if i := bytes.IndexByte(consumed, '\\'); i != -1 {
		return consumed[:i], consumed[i:]
	}
	return consumed, nil
}

func consumeFieldFromRight(payload []byte) ([]byte, []byte) {
	if len(payload) == 0 {
		return nil, nil
	}
	consumed := payload
	// \field1\field2\...\fieldN
	if i := bytes.LastIndexByte(consumed, '\\'); i != -1 {
		return consumed[i+1:], consumed[:i]
	}
	return consumed, nil
}

func consumeParam(payload []byte) (Param, []byte, error) {
	var name, value []byte

	rest := payload

	// when consuming a param, we expect at least two fields
	name, rest = consumeField(rest)
	if rest == nil {
		return Param{}, nil, errors.New("not enough fields for a param")
	}
	if len(name) == 0 {
		return Param{}, nil, errors.New("param name is empty")
	}

	// the value and the rest of the payload can be empty
	value, rest = consumeField(rest)

	return Param{name, value}, rest, nil
}

func consumeParamFromRight(payload []byte) (Param, []byte, error) {
	var name, value []byte

	rest := payload

	// when consuming a param, we expect at least two fields
	value, rest = consumeFieldFromRight(rest)
	if rest == nil {
		return Param{}, nil, errors.New("not enough fields for a param")
	}

	name, rest = consumeFieldFromRight(rest)
	if len(name) == 0 {
		return Param{}, nil, errors.New("param name is empty")
	}

	return Param{name, value}, rest, nil
}

func latin1(bytes []byte) string {
	encoded, _ := charmap.ISO8859_1.NewDecoder().Bytes(bytes)
	return string(encoded)
}
