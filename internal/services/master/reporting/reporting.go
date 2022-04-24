package reporting

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

	"github.com/sergeii/swat4master/internal/core/instances"
	"github.com/sergeii/swat4master/internal/core/probes"
	"github.com/sergeii/swat4master/internal/core/servers"
	"github.com/sergeii/swat4master/internal/entity/addr"
	"github.com/sergeii/swat4master/internal/entity/details"
	ds "github.com/sergeii/swat4master/internal/entity/discovery/status"
	"github.com/sergeii/swat4master/internal/services/discovery/finding"
	"github.com/sergeii/swat4master/internal/services/monitoring"
	"github.com/sergeii/swat4master/pkg/binutils"
	"github.com/sergeii/swat4master/pkg/gamespy/browsing/query/filter"
)

var MasterResponseChallenge = []byte{0x44, 0x3d, 0x73, 0x7e, 0x6a, 0x59}
var MasterResponseIsAvailable = []byte{0xfe, 0xfd, 0x09, 0x00, 0x00, 0x00, 0x00}

var ErrUnknownRequestType = errors.New("unknown request type")
var ErrInvalidRequestPayload = errors.New("invalid request payload")
var ErrUnknownInstanceID = errors.New("unknown instance id")

type MasterMsg uint8

const (
	MasterMsgChallenge MasterMsg = 0x01
	MasterMsgHeartbeat MasterMsg = 0x03
	MasterMsgKeepalive MasterMsg = 0x08
	MasterMsgAvailable MasterMsg = 0x09
)

func (msg MasterMsg) String() string {
	switch msg {
	case MasterMsgChallenge:
		return "challenge"
	case MasterMsgHeartbeat:
		return "heartbeat"
	case MasterMsgKeepalive:
		return "keepalive"
	case MasterMsgAvailable:
		return "available"
	}
	return fmt.Sprintf("0x%02x", uint8(msg))
}

type MasterReporterService struct {
	servers   servers.Repository
	instances instances.Repository
	finder    *finding.Service
	metrics   *monitoring.MetricService
}

func NewService(
	servers servers.Repository,
	instances instances.Repository,
	finder *finding.Service,
	metrics *monitoring.MetricService,
) *MasterReporterService {
	mrs := &MasterReporterService{
		servers:   servers,
		instances: instances,
		finder:    finder,
		metrics:   metrics,
	}
	return mrs
}

func (mrs *MasterReporterService) DispatchRequest(
	ctx context.Context, req []byte, addr *net.UDPAddr,
) ([]byte, MasterMsg, error) {
	var resp []byte
	var err error
	reqType := MasterMsg(req[0])
	switch reqType {
	case MasterMsgAvailable:
		resp, err = mrs.handleAvailable()
	case MasterMsgChallenge:
		resp, err = mrs.handleChallenge(req)
	case MasterMsgHeartbeat:
		resp, err = mrs.handleHeartbeat(ctx, req, addr)
	case MasterMsgKeepalive:
		resp, err = mrs.handleKeepalive(ctx, req, addr)
	default:
		return nil, reqType, ErrUnknownRequestType
	}
	return resp, reqType, err
}

func (mrs *MasterReporterService) handleAvailable() ([]byte, error) {
	return MasterResponseIsAvailable, nil
}

func (mrs *MasterReporterService) handleChallenge(req []byte) ([]byte, error) {
	instanceID, ok := parseInstanceID(req)
	if !ok {
		return nil, ErrInvalidRequestPayload
	}
	resp := make([]byte, 0, 7)
	resp = append(resp, 0xfe, 0xfd, 0x0a)
	resp = append(resp, instanceID...)
	return resp, nil
}

func (mrs *MasterReporterService) handleHeartbeat(
	ctx context.Context,
	req []byte,
	addr *net.UDPAddr,
) ([]byte, error) {
	instanceID, ok := parseInstanceID(req)
	if !ok {
		return nil, ErrInvalidRequestPayload
	}

	fields, err := parseHeartbeatFields(req[5:])
	if err != nil || len(fields) == 0 {
		return nil, ErrInvalidRequestPayload
	}

	svrAddr, queryPort, err := parseAddrFromParams(addr, fields)
	if err != nil {
		return nil, err
	}

	svr, err := mrs.obtainServerByAddr(ctx, svrAddr, queryPort)
	if err != nil {
		return nil, err
	}

	// remove the server from the list on statechanged=2
	if statechanged, ok := fields["statechanged"]; ok && statechanged == "2" {
		// in this case server missing in the repo should not be considered an error
		// as this may be a result of a race condition triggered by a double request from the server
		if err = mrs.removeServer(ctx, svr, addr, instanceID); err != nil {
			return nil, err
		}
		// because the server is exiting, it does not expect a response from us
		return nil, nil
	}

	// add the reported server to the list
	if err := mrs.reportServer(ctx, svr, addr, instanceID, fields); err != nil {
		return nil, err
	}

	// prepare the packed client address to be used in the response
	clientAddr := make([]byte, 7)
	copy(clientAddr[1:5], addr.IP.To4()) // the first byte is supposed to be null byte, so leave it zero value
	binary.BigEndian.PutUint16(clientAddr[5:7], uint16(addr.Port))
	resp := make([]byte, 28)
	copy(resp[:3], []byte{0xfe, 0xfd, 0x01})  // initial bytes, 3 of them
	copy(resp[3:7], instanceID)               // instance id (4 bytes)
	copy(resp[7:13], MasterResponseChallenge) // challenge, we keep it constant, 6 bytes
	hex.Encode(resp[13:27], clientAddr)       // hex-encoded client address, 14 bytes
	// the last 28th byte remains zero, because the response payload is supposed to be null-terminated

	return resp, nil
}

func (mrs *MasterReporterService) handleKeepalive(
	ctx context.Context,
	req []byte,
	addr *net.UDPAddr,
) ([]byte, error) { // nolint: unparam
	instanceID, ok := parseInstanceID(req)
	if !ok {
		return nil, ErrInvalidRequestPayload
	}
	instance, err := mrs.instances.GetByID(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	// the addressed must match, otherwise it could be a spoofing attempt
	if !instance.GetIP().Equal(addr.IP.To4()) {
		return nil, ErrUnknownInstanceID
	}
	server, err := mrs.servers.GetByAddr(ctx, instance.GetAddr())
	if err != nil {
		return nil, err
	}
	if err = mrs.servers.AddOrUpdate(ctx, server); err != nil {
		return nil, err
	}
	return nil, nil
}

func (mrs *MasterReporterService) removeServer(
	ctx context.Context,
	svr servers.Server,
	addr *net.UDPAddr,
	instanceID string,
) error {
	instance, err := mrs.instances.GetByID(ctx, instanceID)
	if err != nil {
		// this could be a race condition - ignore
		if errors.Is(err, instances.ErrInstanceNotFound) {
			return nil
		}
		return err
	}
	// make sure to verify the "owner" of the provided instance id
	if instance.GetDottedIP() != svr.GetDottedIP() {
		return ErrUnknownInstanceID
	}
	if err = mrs.servers.Remove(ctx, svr); err != nil {
		return err
	}
	if err = mrs.instances.RemoveByID(ctx, instanceID); err != nil {
		return err
	}
	mrs.metrics.ReporterRemovals.Inc()
	log.Info().
		Stringer("src", addr).Str("instance", fmt.Sprintf("% x", instanceID)).
		Msg("Successfully removed server on request")
	return nil
}

func (mrs *MasterReporterService) reportServer(
	ctx context.Context,
	svr servers.Server,
	connAddr *net.UDPAddr,
	instanceID string,
	fields map[string]string,
) error {
	instance, err := instances.New(instanceID, svr.GetIP(), svr.GetGamePort())
	if err != nil {
		log.Error().
			Err(err).
			Stringer("src", connAddr).Str("instance", fmt.Sprintf("% x", instanceID)).
			Msg("Failed to create an instance")
		return err
	}

	info, err := details.NewInfoFromParams(fields)
	if err != nil {
		log.Error().
			Err(err).
			Stringer("src", connAddr).Str("instance", fmt.Sprintf("% x", instanceID)).
			Msg("Failed to parse reported fields")
		return err
	}
	svr.UpdateInfo(info)
	svr.UpdateDiscoveryStatus(ds.Master | ds.Info)

	// discover query port for newly reporter servers
	if svr.HasNoDiscoveryStatus(ds.Port | ds.PortRetry) {
		if err := mrs.finder.DiscoverPort(ctx, svr.GetAddr(), probes.NC, probes.NC); err != nil {
			log.Error().
				Err(err).
				Stringer("src", connAddr).Str("instance", fmt.Sprintf("% x", instanceID)).
				Stringer("server", svr).
				Msg("Failed to add server for port discovery")
			return err
		}
		svr.UpdateDiscoveryStatus(ds.PortRetry)
	}

	if err := mrs.servers.AddOrUpdate(ctx, svr); err != nil {
		log.Error().
			Err(err).
			Stringer("src", connAddr).Str("instance", fmt.Sprintf("% x", instanceID)).
			Msg("Failed to add server to repository")
		return err
	}
	if err := mrs.instances.Add(ctx, instance); err != nil {
		log.Error().
			Err(err).
			Stringer("src", connAddr).Str("instance", fmt.Sprintf("% x", instanceID)).
			Msg("Failed to add instance to repository")
		return err
	}

	log.Info().
		Stringer("src", connAddr).
		Str("instance", fmt.Sprintf("% x", instanceID)).
		Stringer("server", svr.GetAddr()).
		Msg("Successfully reported server")

	return nil
}

func (mrs *MasterReporterService) obtainServerByAddr(
	ctx context.Context,
	svrAddr addr.Addr,
	queryPort int,
) (servers.Server, error) {
	svr, err := mrs.servers.GetByAddr(ctx, svrAddr)
	if errors.Is(err, servers.ErrServerNotFound) {
		if svr, err = servers.NewFromAddr(svrAddr, queryPort); err != nil {
			return servers.Blank, err
		}
	} else if err != nil {
		return servers.Blank, err
	}
	return svr, nil
}

func parseInstanceID(req []byte) (string, bool) {
	if len(req) < 5 {
		return "", false
	}
	instanceID := req[1:5]
	// there cannot be nulls in the instance id
	for _, b := range instanceID {
		if b == 0x00 {
			return "", false
		}
	}
	return string(instanceID), true
}

func parseHeartbeatFields(payload []byte) (map[string]string, error) {
	var nameBin, valueBin []byte
	fields := make(map[string]string)
	unparsed := payload
	// Collect field pairs of name and value delimited by null
	// Then validate the fields against the predefined list of allowed names
	// then put the pairs into a map
	for len(unparsed) > 0 && unparsed[0] != 0x00 {
		nameBin, unparsed = binutils.ConsumeCString(unparsed)
		name := string(nameBin)
		// only save those params that belong to the predefined list of reportable params
		if !isReportableField(name) {
			log.Debug().
				Str("field", name).
				Msg("Field is not accepted for reporting")
			continue
		}
		// there should be another c string in the slice after the field name, which is the field's value
		// Throw an error if that's not the case
		if len(unparsed) == 0 || unparsed[0] == 0x00 {
			return nil, ErrInvalidRequestPayload
		}
		valueBin, unparsed = binutils.ConsumeCString(unparsed)
		value := string(bytes.ToValidUTF8(valueBin, []byte{'?'}))
		fields[name] = value
	}
	return fields, nil
}

func parseAddrFromParams(connAddr *net.UDPAddr, fields map[string]string) (addr.Addr, int, error) {
	gamePort, ok1 := parseNumericField(fields, "hostport")
	queryPort, ok2 := parseNumericField(fields, "localport")
	if !ok1 || !ok2 {
		return addr.Blank, -1, ErrInvalidRequestPayload
	}
	svrAddr, err := addr.New(connAddr.IP, gamePort)
	if err != nil {
		return addr.Blank, -1, err
	}
	return svrAddr, queryPort, nil
}

func parseNumericField(fields map[string]string, fieldName string) (int, bool) {
	valueStr, ok := fields[fieldName]
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
