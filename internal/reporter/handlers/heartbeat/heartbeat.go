package heartbeat

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net"
	"strconv"

	"github.com/sergeii/swat4master/internal/core/entities/addr"
	"github.com/sergeii/swat4master/internal/core/entities/master"
	"github.com/sergeii/swat4master/internal/core/usecases/removeserver"
	"github.com/sergeii/swat4master/internal/core/usecases/reportserver"
	"github.com/sergeii/swat4master/internal/reporter"
	"github.com/sergeii/swat4master/internal/services/monitoring"
	"github.com/sergeii/swat4master/pkg/binutils"
	"github.com/sergeii/swat4master/pkg/gamespy/browsing/query/filter"
)

type Handler struct {
	reportServerUC reportserver.UseCase
	removeServerUC removeserver.UseCase
	metrics        *monitoring.MetricService
}

func New(
	dispatcher *reporter.Dispatcher,
	metrics *monitoring.MetricService,
	reportServerUC reportserver.UseCase,
	removeServerUC removeserver.UseCase,
) (Handler, error) {
	handler := Handler{
		reportServerUC: reportServerUC,
		removeServerUC: removeServerUC,
		metrics:        metrics,
	}
	if err := dispatcher.Register(master.MsgHeartbeat, handler); err != nil {
		return Handler{}, err
	}
	return handler, nil
}

func (h Handler) Handle(
	ctx context.Context,
	connAddr *net.UDPAddr,
	payload []byte,
) ([]byte, error) {
	instanceID, rest, err := reporter.ParseInstanceID(payload)
	if err != nil {
		return nil, err
	}

	fields, err := parseHeartbeatParams(rest)
	if err != nil || len(fields) == 0 {
		return nil, fmt.Errorf("invalid heartbeat payload: %w", err)
	}

	svrAddr, queryPort, err := parseAddrFromHeartbeatParams(connAddr, fields)
	if err != nil {
		return nil, err
	}

	// remove the server from the list on statechanged=2
	if statechanged, ok := fields["statechanged"]; ok && statechanged == "2" {
		return h.removeServer(ctx, svrAddr, instanceID)
	}

	// add the reported server to the list
	return h.reportServer(ctx, connAddr, svrAddr, queryPort, instanceID, fields)
}

func (h Handler) reportServer(
	ctx context.Context,
	connAddr *net.UDPAddr,
	svrAddr addr.Addr,
	queryPort int,
	instanceID string,
	fields map[string]string,
) ([]byte, error) {
	req := reportserver.NewRequest(svrAddr, queryPort, instanceID, fields)
	if err := h.reportServerUC.Execute(ctx, req); err != nil {
		return nil, err
	}

	// prepare the packed client address to be used in the response
	clientAddr := make([]byte, 7)
	copy(clientAddr[1:5], connAddr.IP.To4()) // the first byte is supposed to be null byte, so leave it zero value
	binary.BigEndian.PutUint16(clientAddr[5:7], uint16(connAddr.Port))
	// the last 28th byte remains zero, because the response payload is supposed to be null-terminated
	resp := make([]byte, 28)
	copy(resp[:3], []byte{0xfe, 0xfd, 0x01})   // initial bytes, 3 of them
	copy(resp[3:7], instanceID)                // instance id (4 bytes)
	copy(resp[7:13], master.ResponseChallenge) // challenge, we keep it constant, 6 bytes
	hex.Encode(resp[13:27], clientAddr)        // hex-encoded client address, 14 bytes

	return resp, nil
}

func (h Handler) removeServer(
	ctx context.Context,
	svrAddr addr.Addr,
	instanceID string,
) ([]byte, error) {
	req := removeserver.NewRequest(instanceID, svrAddr)
	if err := h.removeServerUC.Execute(ctx, req); err != nil {
		return nil, err
	}
	h.metrics.ReporterRemovals.Inc()
	// because the server is exiting, it does not expect a response from us
	return nil, nil
}

func parseHeartbeatParams(payload []byte) (map[string]string, error) {
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
			continue
		}
		// there should be another c string in the slice after the field name, which is the field's value
		// Throw an error if that's not the case
		if len(unparsed) == 0 || unparsed[0] == 0x00 {
			return nil, fmt.Errorf("missing value for field %s", name)
		}
		valueBin, unparsed = binutils.ConsumeCString(unparsed)
		value := string(bytes.ToValidUTF8(valueBin, []byte{'?'}))
		fields[name] = value
	}
	return fields, nil
}

func parseAddrFromHeartbeatParams(
	connAddr *net.UDPAddr,
	fields map[string]string,
) (addr.Addr, int, error) {
	gamePort, ok1 := parseNumericField(fields, "hostport")
	queryPort, ok2 := parseNumericField(fields, "localport")
	if !ok1 || !ok2 {
		return addr.Blank, -1, fmt.Errorf("missing hostport or localport field")
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
