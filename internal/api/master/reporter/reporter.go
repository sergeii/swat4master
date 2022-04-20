package reporter

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"strconv"

	"github.com/rs/zerolog/log"

	"github.com/sergeii/swat4master/internal/aggregate"
	"github.com/sergeii/swat4master/internal/server"
	"github.com/sergeii/swat4master/internal/server/memory"
	"github.com/sergeii/swat4master/pkg/binutils"
	"github.com/sergeii/swat4master/pkg/gamespy/query/filter"
)

const (
	MasterMsgChallenge = 0x01
	MasterMsgHeartbeat = 0x03
	MasterMsgKeepalive = 0x08
	MasterMsgAvailable = 0x09
)

var MasterResponseChallenge = []byte{0x44, 0x3d, 0x73, 0x7e, 0x6a, 0x59}
var MasterResponseIsAvailable = []byte{0xfe, 0xfd, 0x09, 0x00, 0x00, 0x00, 0x00}

var ErrUnknownRequestType = errors.New("unknown request type")
var ErrInvalidChallengeRequest = errors.New("invalid challenge request")
var ErrInvalidHeartbeatRequest = errors.New("invalid heartbeat request")
var ErrInvalidKeepaliveRequest = errors.New("invalid keepalive request")

type MasterReporterService struct {
	servers server.Repository
}

type Option func(mrs *MasterReporterService) error

func NewService(cfgs ...Option) (*MasterReporterService, error) {
	mrs := &MasterReporterService{}
	for _, cfg := range cfgs {
		if err := cfg(mrs); err != nil {
			return nil, err
		}
	}
	return mrs, nil
}

func WithServerRepository(repo server.Repository) Option {
	return func(mrs *MasterReporterService) error {
		mrs.servers = repo
		return nil
	}
}

func WithMemoryServerRepositiory() Option {
	repo := memory.New()
	return WithServerRepository(repo)
}

func (mrs *MasterReporterService) DispatchRequest(ctx context.Context, req []byte, addr *net.UDPAddr) ([]byte, error) {
	switch req[0] {
	case MasterMsgAvailable:
		return mrs.handleAvailable(ctx, req, addr)
	case MasterMsgChallenge:
		return mrs.handleChallenge(ctx, req, addr)
	case MasterMsgHeartbeat:
		return mrs.handleHeartbeat(ctx, req, addr)
	case MasterMsgKeepalive:
		return mrs.handleKeepalive(ctx, req, addr)
	}
	return nil, ErrUnknownRequestType
}

func (mrs *MasterReporterService) handleAvailable(ctx context.Context, req []byte, addr *net.UDPAddr) ([]byte, error) {
	return MasterResponseIsAvailable, nil
}

func (mrs *MasterReporterService) handleChallenge(ctx context.Context, req []byte, addr *net.UDPAddr) ([]byte, error) {
	instanceID, ok := parseInstanceID(req)
	if !ok {
		return nil, ErrInvalidChallengeRequest
	}
	resp := make([]byte, 0, 7)
	resp = append(resp, 0xfe, 0xfd, 0x0a)
	resp = append(resp, instanceID...) // instance id
	return resp, nil
}

func (mrs *MasterReporterService) handleHeartbeat(ctx context.Context, req []byte, addr *net.UDPAddr) ([]byte, error) {
	instanceID, ok := parseInstanceID(req)
	if !ok {
		return nil, ErrInvalidHeartbeatRequest
	}

	params, err := parseHeartbeatFields(req[5:])
	if err != nil || len(params) == 0 {
		return nil, ErrInvalidHeartbeatRequest
	}

	svr, err := parseServerFromParams(addr, params)
	if err != nil {
		return nil, err
	}

	// remove the server from the list on statechanged=2
	if statechanged, ok := params["statechanged"]; ok && statechanged == "2" {
		// in this case server missing in the repo should not be considered an error
		// as this may be a result of a race condition triggered by a double request from the server
		if err := mrs.removeServer(svr, addr, instanceID); err != nil && !errors.Is(err, server.ErrServerNotFound) {
			return nil, err
		}
		// because the server is exiting, it does not expect a response from us
		return nil, nil
	}

	// add the reported server to the list
	return mrs.reportServer(svr, addr, instanceID, params)
}

func (mrs *MasterReporterService) handleKeepalive(ctx context.Context, req []byte, addr *net.UDPAddr) ([]byte, error) {
	instanceID, ok := parseInstanceID(req)
	if !ok {
		return nil, ErrInvalidKeepaliveRequest
	}
	if err := mrs.servers.Renew(addr.IP.String(), string(instanceID)); err != nil {
		return nil, err
	}
	return nil, nil
}

func (mrs *MasterReporterService) removeServer(
	svr *aggregate.GameServer,
	addr *net.UDPAddr,
	instanceID []byte,
) error {
	if err := mrs.servers.Remove(svr.GetDottedIP(), string(instanceID)); err != nil {
		log.Error().
			Err(err).
			Stringer("src", addr).Str("instance", fmt.Sprintf("% x", instanceID)).
			Msg("Failed to remove server on request")
		return err
	}
	log.Info().
		Stringer("src", addr).Str("instance", fmt.Sprintf("% x", instanceID)).
		Msg("Successfully removed server on request")
	return nil
}

func (mrs *MasterReporterService) reportServer(
	svr *aggregate.GameServer,
	connAddr *net.UDPAddr,
	instanceID []byte,
	params map[string]string,
) ([]byte, error) {
	if err := mrs.servers.Report(svr, string(instanceID), params); err != nil {
		log.Error().
			Err(err).
			Stringer("src", connAddr).Str("instance", fmt.Sprintf("% x", instanceID)).
			Msg("Failed to report server")
		return nil, err
	}
	log.Info().
		Stringer("src", connAddr).
		Str("instance", fmt.Sprintf("% x", instanceID)).
		Str("server", svr.GetAddr()).
		Msg("Successfully reported server")
	// prepare the packed client address to be used in the response
	clientAddr := make([]byte, 7)
	copy(clientAddr[1:5], connAddr.IP.To4()) // the first byte is supposed to be null byte, so leave it zero value
	binary.BigEndian.PutUint16(clientAddr[5:7], uint16(connAddr.Port))
	resp := make([]byte, 28)
	copy(resp[:3], []byte{0xfe, 0xfd, 0x01})  // initial bytes, 3 of them
	copy(resp[3:7], instanceID)               // instance id (4 bytes)
	copy(resp[7:13], MasterResponseChallenge) // challenge, we keep it constant, 6 bytes
	hex.Encode(resp[13:27], clientAddr)       // hex-encoded client address, 14 bytes
	// the last 28th byte remains zero, because the response payload is supposed to be null-terminated
	return resp, nil
}

func parseInstanceID(req []byte) ([]byte, bool) {
	if len(req) < 5 {
		return nil, false
	}
	instanceID := req[1:5]
	// there cannot be nulls in the instance id
	for _, b := range instanceID {
		if b == 0x00 {
			return nil, false
		}
	}
	return instanceID, true
}

func parseHeartbeatFields(payload []byte) (map[string]string, error) {
	var fieldBin, valueBin []byte
	params := make(map[string]string)
	unparsed := payload
	// Collect field pairs of name and value delimited by null
	// Then validate the fields against the predefined list of allowed names
	// then put the pairs into a map
	for len(unparsed) > 0 && unparsed[0] != 0x00 {
		fieldBin, unparsed = binutils.ConsumeCString(unparsed)
		field := string(fieldBin)
		// only save those params that belong to the predefined list of reportable params
		if !isReportableField(field) {
			log.Debug().
				Str("field", field).
				Msg("Field is not accepted for reporting")
			continue
		}
		// there should be another c string in the slice after the field name, which is the field's value
		// Throw an error if that's not the case
		if len(unparsed) == 0 || unparsed[0] == 0x00 {
			return nil, ErrInvalidHeartbeatRequest
		}
		valueBin, unparsed = binutils.ConsumeCString(unparsed)
		value := string(bytes.ToValidUTF8(valueBin, []byte{'?'}))
		params[field] = value
	}
	return params, nil
}

func parseServerFromParams(addr *net.UDPAddr, params map[string]string) (*aggregate.GameServer, error) {
	gamePort, ok1 := parseNumericField(params, "hostport")
	queryPort, ok2 := parseNumericField(params, "localport")
	if !ok1 || !ok2 {
		return nil, ErrInvalidHeartbeatRequest
	}
	svr, err := aggregate.NewGameServer(addr.IP.To4(), gamePort, queryPort)
	if err != nil {
		return nil, err
	}
	return svr, nil
}

func parseNumericField(params map[string]string, field string) (int, bool) {
	valueStr, ok := params[field]
	if !ok {
		return 0, false
	}
	valueInt, err := strconv.Atoi(valueStr)
	if err != nil {
		return 0, false
	}
	return valueInt, true
}

func isReportableField(field string) bool {
	switch field {
	case
		"localip0",
		"localip1",
		"localport",
		"natneg",
		"statechanged":
		return true
	}
	return filter.IsQueryField(field)
}
