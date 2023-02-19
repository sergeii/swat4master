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

	"github.com/go-playground/validator/v10"
	"github.com/rs/zerolog"

	"github.com/sergeii/swat4master/internal/core/instances"
	"github.com/sergeii/swat4master/internal/core/probes"
	"github.com/sergeii/swat4master/internal/core/servers"
	"github.com/sergeii/swat4master/internal/entity/addr"
	"github.com/sergeii/swat4master/internal/entity/details"
	ds "github.com/sergeii/swat4master/internal/entity/discovery/status"
	"github.com/sergeii/swat4master/internal/services/discovery/finding"
	"github.com/sergeii/swat4master/internal/services/monitoring"
	"github.com/sergeii/swat4master/internal/services/server"
	"github.com/sergeii/swat4master/pkg/binutils"
	"github.com/sergeii/swat4master/pkg/gamespy/browsing/query/filter"
)

var (
	MasterResponseChallenge   = []byte{0x44, 0x3d, 0x73, 0x7e, 0x6a, 0x59}
	MasterResponseIsAvailable = []byte{0xfe, 0xfd, 0x09, 0x00, 0x00, 0x00, 0x00}
)

var (
	ErrUnknownRequestType    = errors.New("unknown request type")
	ErrInvalidRequestPayload = errors.New("invalid request payload")
	ErrUnknownInstanceID     = errors.New("unknown instance id")
)

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

type Service struct {
	servers        servers.Repository
	instances      instances.Repository
	findingService *finding.Service
	serverService  *server.Service
	metrics        *monitoring.MetricService
	validate       *validator.Validate
	logger         *zerolog.Logger
}

func NewService(
	servers servers.Repository,
	instances instances.Repository,
	serverService *server.Service,
	findingService *finding.Service,
	metrics *monitoring.MetricService,
	validate *validator.Validate,
	logger *zerolog.Logger,
) *Service {
	return &Service{
		servers:        servers,
		instances:      instances,
		serverService:  serverService,
		findingService: findingService,
		metrics:        metrics,
		validate:       validate,
		logger:         logger,
	}
}

func (s *Service) DispatchRequest(
	ctx context.Context, req []byte, addr *net.UDPAddr,
) ([]byte, MasterMsg, error) {
	var resp []byte
	var err error
	reqType := MasterMsg(req[0])
	switch reqType {
	case MasterMsgAvailable:
		resp, err = s.handleAvailable()
	case MasterMsgChallenge:
		resp, err = s.handleChallenge(req)
	case MasterMsgHeartbeat:
		resp, err = s.handleHeartbeat(ctx, req, addr)
	case MasterMsgKeepalive:
		resp, err = s.handleKeepalive(ctx, req, addr)
	default:
		return nil, reqType, ErrUnknownRequestType
	}
	return resp, reqType, err
}

func (s *Service) handleAvailable() ([]byte, error) {
	return MasterResponseIsAvailable, nil
}

func (s *Service) handleChallenge(req []byte) ([]byte, error) {
	instanceID, ok := parseInstanceID(req)
	if !ok {
		return nil, ErrInvalidRequestPayload
	}
	resp := make([]byte, 0, 7)
	resp = append(resp, 0xfe, 0xfd, 0x0a)
	resp = append(resp, instanceID...)
	return resp, nil
}

func (s *Service) handleKeepalive(
	ctx context.Context,
	req []byte,
	addr *net.UDPAddr,
) ([]byte, error) { // nolint: unparam
	instanceID, ok := parseInstanceID(req)
	if !ok {
		return nil, ErrInvalidRequestPayload
	}

	instance, err := s.instances.GetByID(ctx, instanceID)
	if err != nil {
		return nil, err
	}

	// the addressed must match, otherwise it could be a spoofing attempt
	if !instance.GetIP().Equal(addr.IP.To4()) {
		return nil, ErrUnknownInstanceID
	}

	// make sure no other concurrent update is possible
	svr, err := s.servers.Get(ctx, instance.GetAddr())
	if err != nil {
		return nil, err
	}

	// although keepalive request does not provide
	// any additional information about their server such
	// as player count or the scores
	// we still want to bump the server,
	// so it keeps appearing in the list
	svr.Refresh()

	if _, updateErr := s.servers.Update(ctx, svr, func(s *servers.Server) bool {
		s.Refresh()
		return true
	}); updateErr != nil {
		return nil, updateErr
	}

	return nil, nil
}

func (s *Service) handleHeartbeat(
	ctx context.Context,
	req []byte,
	connAddr *net.UDPAddr,
) ([]byte, error) {
	instanceID, ok := parseInstanceID(req)
	if !ok {
		return nil, ErrInvalidRequestPayload
	}

	fields, err := s.parseHeartbeatFields(req[5:])
	if err != nil || len(fields) == 0 {
		return nil, ErrInvalidRequestPayload
	}

	svrAddr, queryPort, err := parseAddrFromParams(connAddr, fields)
	if err != nil {
		return nil, err
	}

	// remove the server from the list on statechanged=2
	if statechanged, ok := fields["statechanged"]; ok && statechanged == "2" {
		return s.removeServer(ctx, connAddr, svrAddr, instanceID)
	}

	svr, err := s.obtainServerByAddr(ctx, svrAddr, queryPort)
	if err != nil {
		return nil, err
	}

	// add the reported server to the list
	return s.reportServer(ctx, connAddr, svr, instanceID, fields)
}

func (s *Service) removeServer(
	ctx context.Context,
	connAddr *net.UDPAddr,
	svrAddr addr.Addr,
	instanceID string,
) ([]byte, error) {
	svr, err := s.servers.Get(ctx, svrAddr)
	if err != nil {
		switch {
		case errors.Is(err, servers.ErrServerNotFound):
			s.logger.Info().
				Stringer("src", connAddr).Str("instance", fmt.Sprintf("% x", instanceID)).
				Msg("Removed server not found")
			return nil, nil
		default:
			return nil, err
		}
	}

	instance, err := s.instances.GetByID(ctx, instanceID)
	if err != nil {
		switch {
		// this could be a race condition - ignore
		case errors.Is(err, instances.ErrInstanceNotFound):
			s.logger.Info().
				Stringer("src", connAddr).
				Stringer("server", svr).
				Str("instance", fmt.Sprintf("% x", instanceID)).
				Msg("Instance for removed server not found")
			return nil, nil
		default:
			return nil, err
		}
	}
	// make sure to verify the "owner" of the provided instance id
	if instance.GetDottedIP() != svr.GetDottedIP() {
		return nil, ErrUnknownInstanceID
	}

	if err = s.servers.Remove(ctx, svr, func(s *servers.Server) bool {
		return true
	}); err != nil {
		return nil, err
	}
	if err = s.instances.RemoveByID(ctx, instanceID); err != nil {
		return nil, err
	}

	s.logger.Info().
		Stringer("src", connAddr).Str("instance", fmt.Sprintf("% x", instanceID)).
		Msg("Successfully removed server on request")

	s.metrics.ReporterRemovals.Inc()

	// because the server is exiting, it does not expect a response from us
	return nil, nil
}

func (s *Service) reportServer(
	ctx context.Context,
	connAddr *net.UDPAddr,
	svr servers.Server,
	instanceID string,
	fields map[string]string,
) ([]byte, error) {
	instance, err := instances.New(instanceID, svr.GetIP(), svr.GetGamePort())
	if err != nil {
		s.logger.Error().
			Err(err).
			Stringer("src", connAddr).Str("instance", fmt.Sprintf("% x", instanceID)).
			Msg("Failed to create an instance")
		return nil, err
	}

	info, err := details.NewInfoFromParams(fields)
	if err != nil {
		s.logger.Error().
			Err(err).
			Stringer("src", connAddr).Str("instance", fmt.Sprintf("% x", instanceID)).
			Msg("Failed to parse reported fields")
		return nil, ErrInvalidRequestPayload
	}
	if validateErr := info.Validate(s.validate); validateErr != nil {
		s.logger.Error().
			Err(validateErr).
			Stringer("src", connAddr).Str("instance", fmt.Sprintf("% x", instanceID)).
			Msg("Failed to validate reported fields")
		return nil, ErrInvalidRequestPayload
	}

	svr.UpdateInfo(info)
	svr.UpdateDiscoveryStatus(ds.Master | ds.Info)

	if svr, err = s.serverService.CreateOrUpdate(ctx, svr, func(conflict *servers.Server) {
		// in case of conflict, just do all the same
		conflict.UpdateInfo(info)
		conflict.UpdateDiscoveryStatus(ds.Master | ds.Info)
	}); err != nil {
		s.logger.Error().
			Err(err).
			Stringer("src", connAddr).Str("instance", fmt.Sprintf("% x", instanceID)).
			Msg("Failed to add server to repository")
		return nil, err
	}

	if err := s.instances.Add(ctx, instance); err != nil {
		s.logger.Error().
			Err(err).
			Stringer("src", connAddr).Str("instance", fmt.Sprintf("% x", instanceID)).
			Msg("Failed to add instance to repository")
		return nil, err
	}

	// attempt to discover query port for newly reported servers
	if err := s.maybeDiscoverPort(ctx, svr); err != nil {
		// it's not critical if we fail here, so don't return an error but log it
		s.logger.Error().
			Err(err).
			Stringer("src", connAddr).Str("instance", fmt.Sprintf("% x", instanceID)).
			Stringer("server", svr).
			Msg("Failed to add server for port discovery")
	}

	s.logger.Info().
		Stringer("src", connAddr).
		Str("instance", fmt.Sprintf("% x", instanceID)).
		Stringer("server", svr).
		Msg("Successfully reported server")

	// prepare the packed client address to be used in the response
	clientAddr := make([]byte, 7)
	copy(clientAddr[1:5], connAddr.IP.To4()) // the first byte is supposed to be null byte, so leave it zero value
	binary.BigEndian.PutUint16(clientAddr[5:7], uint16(connAddr.Port))
	// the last 28th byte remains zero, because the response payload is supposed to be null-terminated
	resp := make([]byte, 28)
	copy(resp[:3], []byte{0xfe, 0xfd, 0x01})  // initial bytes, 3 of them
	copy(resp[3:7], instanceID)               // instance id (4 bytes)
	copy(resp[7:13], MasterResponseChallenge) // challenge, we keep it constant, 6 bytes
	hex.Encode(resp[13:27], clientAddr)       // hex-encoded client address, 14 bytes

	return resp, nil
}

func (s *Service) maybeDiscoverPort(ctx context.Context, pending servers.Server) error {
	var err error
	// the server has either already go its port discovered
	// or it is currently in the queue
	if !pending.HasNoDiscoveryStatus(ds.Port | ds.PortRetry) {
		return nil
	}

	if err = s.findingService.DiscoverPort(ctx, pending.GetAddr(), probes.NC, probes.NC); err != nil {
		return err
	}

	pending.UpdateDiscoveryStatus(ds.PortRetry)

	if _, err := s.serverService.Update(ctx, pending, func(conflict *servers.Server) bool {
		// while we were updating this server,
		// it's got the port, or it was put in the queue.
		// In such a case, resolve the conflict by not doing anything
		if conflict.HasDiscoveryStatus(ds.Port | ds.PortRetry) {
			return false
		}
		conflict.UpdateDiscoveryStatus(ds.PortRetry)
		return true
	}); err != nil {
		return err
	}

	return nil
}

func (s *Service) obtainServerByAddr(
	ctx context.Context,
	svrAddr addr.Addr,
	queryPort int,
) (servers.Server, error) {
	svr, err := s.servers.Get(ctx, svrAddr)
	if err != nil {
		switch {
		case errors.Is(err, servers.ErrServerNotFound):
			// create new server
			if svr, err = servers.NewFromAddr(svrAddr, queryPort); err != nil {
				return servers.Blank, err
			}
			return svr, nil
		default:
			return servers.Blank, err
		}
	}
	return svr, nil
}

func parseInstanceID(req []byte) (string, bool) {
	if len(req) < 5 {
		return "", false
	}
	return string(req[1:5]), true
}

func (s *Service) parseHeartbeatFields(payload []byte) (map[string]string, error) {
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
			s.logger.Debug().
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
