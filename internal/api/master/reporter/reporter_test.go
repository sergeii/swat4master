package reporter_test

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeii/swat4master/internal/aggregate"
	"github.com/sergeii/swat4master/internal/api/master/reporter"
	"github.com/sergeii/swat4master/internal/server"
	"github.com/sergeii/swat4master/internal/server/memory"
	"github.com/sergeii/swat4master/internal/testutils"
)

func TestReporter_DispatchAvailableRequest_OK(t *testing.T) {
	service, _ := reporter.NewService(reporter.WithMemoryServerRepositiory())
	resp, msgT, err := service.DispatchRequest(context.TODO(), []byte{0x09}, &net.UDPAddr{})
	assert.NoError(t, err)
	assert.Equal(t, reporter.MasterMsgAvailable, msgT)
	assert.Equal(t, resp, []byte{0xfe, 0xfd, 0x09, 0x00, 0x00, 0x00, 0x00})
}

func TestReporter_DispatchChallengeRequest_OK(t *testing.T) {
	service, _ := reporter.NewService(reporter.WithMemoryServerRepositiory())
	resp, msgT, err := service.DispatchRequest(context.TODO(), []byte{0x01, 0xfe, 0xed, 0xf0, 0x0d}, &net.UDPAddr{})
	assert.NoError(t, err)
	assert.Equal(t, reporter.MasterMsgChallenge, msgT)
	assert.Equal(t, resp, []byte{0xfe, 0xfd, 0x0a, 0xfe, 0xed, 0xf0, 0x0d})
}

func TestReporter_DispatchChallengeRequest_InvalidInstanceID(t *testing.T) {
	tests := []struct {
		name     string
		payload  []byte
		wantResp []byte
		wantErr  error
	}{
		{
			name:     "positive case",
			payload:  []byte{0x01, 0xfe, 0xed, 0xf0, 0x0d},
			wantResp: []byte{0xfe, 0xfd, 0x0a, 0xfe, 0xed, 0xf0, 0x0d},
		},
		{
			name:    "insufficient payload length #1",
			payload: []byte{0x01},
			wantErr: reporter.ErrInvalidChallengeRequest,
		},
		{
			name:    "insufficient payload length #2",
			payload: []byte{0x01, 0xfe, 0xed},
			wantErr: reporter.ErrInvalidChallengeRequest,
		},
		{
			name:    "invalid instance id",
			payload: []byte{0x01, 0x00, 0x00, 0x00, 0x00},
			wantErr: reporter.ErrInvalidChallengeRequest,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, _ := reporter.NewService(reporter.WithMemoryServerRepositiory())
			resp, _, err := service.DispatchRequest(context.TODO(), tt.payload, &net.UDPAddr{})
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			} else {
				require.Equal(t, tt.wantResp, resp)
			}
		})
	}
}

func TestReporter_DispatchHeartbeatRequest_OK(t *testing.T) {
	service, _ := reporter.NewService(reporter.WithMemoryServerRepositiory())
	resp, msgT, err := service.DispatchRequest(
		context.TODO(),
		testutils.PackHeartbeatRequest([]byte{0xfe, 0xed, 0xf0, 0x0d}, testutils.GenServerParams()),
		&net.UDPAddr{IP: net.ParseIP("1.1.1.1"), Port: 10481},
	)
	assert.NoError(t, err)
	assert.Equal(t, reporter.MasterMsgHeartbeat, msgT)
	assert.Equal(t, resp[:3], []byte{0xfe, 0xfd, 0x01})
	assert.Equal(t, resp[3:7], []byte{0xfe, 0xed, 0xf0, 0x0d})
	assert.Equal(t, resp[7:13], []byte{0x44, 0x3d, 0x73, 0x7e, 0x6a, 0x59})

	addr := make([]byte, 7)
	hex.Decode(addr, resp[13:27]) // nolint: errcheck
	assert.Equal(t, addr[0], uint8(0x00))
	assert.Equal(t, "1.1.1.1", net.IPv4(addr[1], addr[2], addr[3], addr[4]).String())
	assert.Equal(t, 10481, int(binary.BigEndian.Uint16(addr[5:7])))
	assert.Equal(t, uint8(0x00), resp[27])
}

func TestReporter_DispatchHeartbeatRequest_ServerIsAddedAndUpdated(t *testing.T) {
	repo := memory.New()
	instanceID := []byte{0xfe, 0xed, 0xf0, 0x0d}
	service, _ := reporter.NewService(reporter.WithServerRepository(repo))
	paramsBefore := testutils.GenExtraServerParams(map[string]string{
		"gametype":   "VIP Escort",
		"mapname":    "A-Bomb Nightclub",
		"numplayers": "16",
		"hostport":   "10580",
		"localport":  "10584",
	})
	resp, _, err := service.DispatchRequest(
		context.TODO(),
		testutils.PackHeartbeatRequest(instanceID, paramsBefore),
		&net.UDPAddr{IP: net.ParseIP("55.55.55.55"), Port: 22712}, // server is behind nat
	)
	assert.NoError(t, err)
	assert.Equal(t, resp[:3], []byte{0xfe, 0xfd, 0x01})
	svr, _ := repo.GetByInstanceID(string(instanceID))
	svrParams := svr.GetReportedParams()
	assert.Equal(t, "16", svrParams["numplayers"])
	assert.Equal(t, "VIP Escort", svrParams["gametype"])
	assert.Equal(t, "A-Bomb Nightclub", svrParams["mapname"])
	assert.Equal(t, "55.55.55.55", svr.GetDottedIP())
	assert.Equal(t, 10580, svr.GetGamePort())
	assert.Equal(t, 10584, svr.GetQueryPort())

	paramsAfter := testutils.GenExtraServerParams(map[string]string{
		"gametype":   "VIP Escort",
		"mapname":    "Food Wall Restaurant",
		"numplayers": "15",
		"hostport":   "10480",
	})
	resp, _, _ = service.DispatchRequest(
		context.TODO(),
		testutils.PackHeartbeatRequest(instanceID, paramsAfter),
		&net.UDPAddr{IP: net.ParseIP("1.1.1.1"), Port: 10481},
	)
	svr, _ = repo.GetByInstanceID(string(instanceID))
	svrParams = svr.GetReportedParams()
	assert.Equal(t, "15", svrParams["numplayers"])
	assert.Equal(t, "VIP Escort", svrParams["gametype"])
	assert.Equal(t, "Food Wall Restaurant", svrParams["mapname"])
	assert.Equal(t, "1.1.1.1", svr.GetDottedIP())
}

func TestReporter_DispatchHeartbeatRequest_HandleServerBehindNAT(t *testing.T) {
	before := time.Now()
	repo := memory.New()
	instanceID := []byte{0xfe, 0xed, 0xf0, 0x0d}
	service, _ := reporter.NewService(reporter.WithServerRepository(repo))
	paramsBefore := testutils.GenExtraServerParams(map[string]string{
		"gametype":   "VIP Escort",
		"mapname":    "A-Bomb Nightclub",
		"numplayers": "16",
		"hostport":   "10480",
		"localport":  "10484",
	})
	resp, err := testutils.SendHeartbeat(
		service, instanceID,
		testutils.WithServerParams(paramsBefore),
		// server is behind nat, connection port is different from the query port
		testutils.WithCustomAddr("1.1.1.1", 22712),
	)
	assert.NoError(t, err)
	assert.Equal(t, resp[:3], []byte{0xfe, 0xfd, 0x01})
	svr, _ := repo.GetByInstanceID(string(instanceID))
	svrParams := svr.GetReportedParams()
	assert.Equal(t, "1.1.1.1", svr.GetDottedIP())
	assert.Equal(t, "16", svrParams["numplayers"])
	assert.Equal(t, "10484", svrParams["localport"])
	assert.Equal(t, "10480", svrParams["hostport"])
	assert.Equal(t, 10480, svr.GetGamePort())
	assert.Equal(t, 10484, svr.GetQueryPort())

	paramsAfter := testutils.GenExtraServerParams(map[string]string{
		"gametype":   "VIP Escort",
		"mapname":    "Food Wall Restaurant",
		"numplayers": "15",
		"hostport":   "10480",
		"localport":  "10484",
	})
	testutils.SendHeartbeat( // nolint: errcheck
		service, instanceID,
		testutils.WithServerParams(paramsAfter),
		testutils.WithCustomAddr("1.1.1.1", 37122),
	)
	svr, _ = repo.GetByInstanceID(string(instanceID))
	svrParams = svr.GetReportedParams()
	assert.Equal(t, "1.1.1.1", svr.GetDottedIP())
	assert.Equal(t, "15", svrParams["numplayers"])
	assert.Equal(t, "10484", svrParams["localport"])
	assert.Equal(t, "10480", svrParams["hostport"])
	assert.Equal(t, 10480, svr.GetGamePort())
	assert.Equal(t, 10484, svr.GetQueryPort())

	servers, _ := repo.GetReportedSince(before)
	assert.Len(t, servers, 1)
}

func TestReporter_DispatchHeartbeatRequest_ServerIsUpdatedWithNewInstanceID(t *testing.T) {
	repo := memory.New()
	oldInstanceID := []byte{0xfe, 0xed, 0xf0, 0x0d}
	service, _ := reporter.NewService(reporter.WithServerRepository(repo))
	params := testutils.GenExtraServerParams(map[string]string{
		"gametype":   "VIP Escort",
		"mapname":    "A-Bomb Nightclub",
		"numplayers": "16",
		"hostport":   "10480",
	})
	resp, _, err := service.DispatchRequest(
		context.TODO(),
		testutils.PackHeartbeatRequest(oldInstanceID, params),
		&net.UDPAddr{IP: net.ParseIP("1.1.1.1"), Port: 10481},
	)
	assert.NoError(t, err)
	assert.Equal(t, resp[:3], []byte{0xfe, 0xfd, 0x01})
	svr, _ := repo.GetByInstanceID(string(oldInstanceID))
	svrParams := svr.GetReportedParams()
	assert.Equal(t, "16", svrParams["numplayers"])
	assert.Equal(t, "VIP Escort", svrParams["gametype"])
	assert.Equal(t, "A-Bomb Nightclub", svrParams["mapname"])
	assert.Equal(t, "1.1.1.1", svr.GetDottedIP())

	newParams := testutils.GenExtraServerParams(map[string]string{
		"gametype":   "Barricaded Suspects",
		"mapname":    "Food Wall Restaurant",
		"numplayers": "15",
		"hostport":   "10480",
	})
	newInstanceID := []byte{0xde, 0xad, 0xbe, 0xef}
	resp, _, _ = service.DispatchRequest(
		context.TODO(),
		testutils.PackHeartbeatRequest(newInstanceID, newParams),
		&net.UDPAddr{IP: net.ParseIP("1.1.1.1"), Port: 10481},
	)
	svr, _ = repo.GetByInstanceID(string(newInstanceID))
	svrParams = svr.GetReportedParams()
	assert.Equal(t, "15", svrParams["numplayers"])
	assert.Equal(t, "Barricaded Suspects", svrParams["gametype"])
	assert.Equal(t, "Food Wall Restaurant", svrParams["mapname"])
	assert.Equal(t, "1.1.1.1", svr.GetDottedIP())

	// at the same time the server is no longer accessible by the former instance key
	svr, getErr := repo.GetByInstanceID(string(oldInstanceID))
	assert.ErrorIs(t, getErr, server.ErrServerNotFound)
	assert.Nil(t, svr)
}

func TestReporter_DispatchHeartbeatRequest_InvalidPayload(t *testing.T) {
	tests := []struct {
		name    string
		payload []byte
		wantErr error
	}{
		{
			name:    "insufficient payload length #1",
			payload: []byte{0x03},
			wantErr: reporter.ErrInvalidHeartbeatRequest,
		},
		{
			name:    "insufficient payload length #2",
			payload: []byte{0x03, 0xfe, 0xed, 0xf0, 0x0d},
			wantErr: reporter.ErrInvalidHeartbeatRequest,
		},
		{
			name:    "no fields are present",
			payload: testutils.PackHeartbeatRequest([]byte{0xfe, 0xed, 0xf0, 0x0d}, nil),
			wantErr: reporter.ErrInvalidHeartbeatRequest,
		},
		{
			name:    "invalid instance id",
			payload: testutils.PackHeartbeatRequest([]byte{0xfe, 0x00, 0x00, 0x0d}, testutils.GenServerParams()),
			wantErr: reporter.ErrInvalidHeartbeatRequest,
		},
		{
			name: "no known fields are present",
			payload: testutils.PackHeartbeatRequest(
				[]byte{0xfe, 0xed, 0xf0, 0x0d},
				map[string]string{"somefield": "Swat4 Server", "other": "1.1"},
			),
			wantErr: reporter.ErrInvalidHeartbeatRequest,
		},
		{
			name: "field has no value",
			payload: testutils.PackHeartbeatRequest(
				[]byte{0xfe, 0xed, 0xf0, 0x0d},
				map[string]string{"hostname": "Swat4 Server", "gamever": ""},
			),
			wantErr: reporter.ErrInvalidHeartbeatRequest,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, _ := reporter.NewService(reporter.WithMemoryServerRepositiory())
			resp, _, err := service.DispatchRequest(
				context.TODO(),
				tt.payload,
				&net.UDPAddr{IP: net.ParseIP("1.1.1.1"), Port: 10481},
			)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.Equal(t, resp[:3], []byte{0xfe, 0xfd, 0x01})
			}
		})
	}
}

func TestReporter_DispatchHeartbeatRequest_OnlyIPv4IsSupported(t *testing.T) {
	tests := []struct {
		name    string
		ipaddr  string
		wantErr error
	}{
		{
			name:    "positive case",
			ipaddr:  "1.1.1.1",
			wantErr: nil,
		},
		{
			name:    "IPv6 is not supported",
			ipaddr:  "2001:0db8:85a3:0000:0000:8a2e:0370:7334",
			wantErr: aggregate.ErrInvalidGameServerIP,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, _ := reporter.NewService(reporter.WithMemoryServerRepositiory())
			resp, _, err := service.DispatchRequest(
				context.TODO(),
				testutils.PackHeartbeatRequest([]byte{0xfe, 0xed, 0xf0, 0x0d}, testutils.GenServerParams()),
				&net.UDPAddr{IP: net.ParseIP(tt.ipaddr), Port: 10481},
			)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.Equal(t, resp[:3], []byte{0xfe, 0xfd, 0x01})
			}
		})
	}
}

func TestReporter_DispatchHeartbeatRequest_ServerLivenessIsRefreshed(t *testing.T) {
	repo := memory.New()
	service, _ := reporter.NewService(reporter.WithServerRepository(repo))

	// initial report
	resp, err := testutils.SendHeartbeat(
		service, []byte{0xfe, 0xed, 0xf0, 0x0d},
		testutils.GenServerParams, testutils.StandardAddr,
	)
	assert.NoError(t, err)
	assert.Equal(t, resp[:3], []byte{0xfe, 0xfd, 0x01})

	time.Sleep(time.Millisecond)
	before := time.Now()
	reportedSinceBefore, _ := repo.GetReportedSince(before)
	assert.Len(t, reportedSinceBefore, 0)

	// successive report refreshes the server
	resp, err = testutils.SendHeartbeat(
		service, []byte{0xfe, 0xed, 0xf0, 0x0d},
		testutils.GenServerParams, testutils.StandardAddr,
	)
	assert.NoError(t, err)
	assert.Equal(t, resp[:3], []byte{0xfe, 0xfd, 0x01})
	reportedSinceBefore, _ = repo.GetReportedSince(before)
	assert.Len(t, reportedSinceBefore, 1)
}

func TestReporter_DispatchHeartbeatRequest_ServerIsRemoved(t *testing.T) {
	repo := memory.New()
	service, _ := reporter.NewService(reporter.WithServerRepository(repo))

	before := time.Now()
	resp, err := testutils.SendHeartbeat(
		service, []byte{0xfe, 0xed, 0xf0, 0x0d},
		testutils.GenServerParams, testutils.StandardAddr,
	)
	assert.NoError(t, err)
	assert.Equal(t, resp[:3], []byte{0xfe, 0xfd, 0x01})
	reportedSinceBefore, _ := repo.GetReportedSince(before)
	assert.Len(t, reportedSinceBefore, 1)

	// remove the server by sending param statechanged=2
	resp, err = testutils.SendHeartbeat(
		service,
		[]byte{0xfe, 0xed, 0xf0, 0x0d},
		func() map[string]string {
			return testutils.GenExtraServerParams(map[string]string{"statechanged": "2"})
		},
		testutils.StandardAddr,
	)
	assert.NoError(t, err)
	assert.Empty(t, resp) // no response
	reportedSinceBefore, _ = repo.GetReportedSince(before)
	assert.Len(t, reportedSinceBefore, 0)

	// subsequent statechanged=2 requests should produce no errors
	resp, err = testutils.SendHeartbeat(
		service,
		[]byte{0xfe, 0xed, 0xf0, 0x0d},
		func() map[string]string {
			return testutils.GenExtraServerParams(map[string]string{"statechanged": "2"})
		},
		testutils.StandardAddr,
	)
	assert.NoError(t, err)
	assert.Empty(t, resp)
	reportedSinceBefore, _ = repo.GetReportedSince(before)
	assert.Len(t, reportedSinceBefore, 0)
}

func TestReporter_DispatchHeartbeatRequest_ServerRemovalIsValidated(t *testing.T) {
	tests := []struct {
		name        string
		instanceID  []byte
		params      map[string]string
		ipaddr      string
		wantSuccess bool
	}{
		{
			name:        "positive case",
			instanceID:  []byte{0xfe, 0xed, 0xf0, 0x0d},
			params:      testutils.GenExtraServerParams(map[string]string{"statechanged": "2"}),
			ipaddr:      "1.1.1.1",
			wantSuccess: true,
		},
		{
			name:        "statechanged != 2",
			instanceID:  []byte{0xfe, 0xed, 0xf0, 0x0d},
			params:      testutils.GenExtraServerParams(map[string]string{"statechanged": "1"}),
			ipaddr:      "1.1.1.1",
			wantSuccess: false, // no error but the request is processed as a normal heartbeat message
		},
		{
			name:        "unknown server",
			instanceID:  []byte{0xde, 0xad, 0xbe, 0xef},
			params:      testutils.GenExtraServerParams(map[string]string{"statechanged": "2"}),
			ipaddr:      "2.2.2.2",
			wantSuccess: false, // no error, could be a subsequent removal request of a former server
		},
		{
			name:        "invalid instance id",
			instanceID:  []byte{0xde, 0xad, 0xbe, 0xef},
			params:      testutils.GenExtraServerParams(map[string]string{"statechanged": "2"}),
			ipaddr:      "1.1.1.1",
			wantSuccess: false,
		},
		{
			name:        "ip addr does not match",
			instanceID:  []byte{0xde, 0xad, 0xbe, 0xef},
			params:      testutils.GenExtraServerParams(map[string]string{"statechanged": "2"}),
			ipaddr:      "2.2.2.2",
			wantSuccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := time.Now()
			repo := memory.New()
			service, _ := reporter.NewService(reporter.WithServerRepository(repo))

			// initial report
			_, _, err := service.DispatchRequest(
				context.TODO(),
				testutils.PackHeartbeatRequest([]byte{0xfe, 0xed, 0xf0, 0x0d}, testutils.GenServerParams()),
				&net.UDPAddr{IP: net.ParseIP("1.1.1.1"), Port: 10481},
			)
			require.NoError(t, err)

			// removal request
			_, _, err = service.DispatchRequest(
				context.TODO(),
				testutils.PackHeartbeatRequest(tt.instanceID, tt.params),
				&net.UDPAddr{IP: net.ParseIP(tt.ipaddr), Port: 10481},
			)
			require.NoError(t, err)

			reportedSinceBefore, _ := repo.GetReportedSince(before)
			if tt.wantSuccess {
				assert.Len(t, reportedSinceBefore, 0)
			} else {
				assert.Len(t, reportedSinceBefore, 1)
			}
		})
	}
}

func TestReporter_DispatchKeepaliveRequest_RefreshesServerLiveness(t *testing.T) {
	repo := memory.New()
	service, _ := reporter.NewService(reporter.WithServerRepository(repo))
	before := time.Now()
	// initial report
	_, _, err := service.DispatchRequest(
		context.TODO(),
		testutils.PackHeartbeatRequest([]byte{0xfe, 0xed, 0xf0, 0x0d}, testutils.GenServerParams()),
		&net.UDPAddr{IP: net.ParseIP("1.1.1.1"), Port: 10481},
	)
	require.NoError(t, err)
	reportedSinceBefore, _ := repo.GetReportedSince(before)
	assert.Len(t, reportedSinceBefore, 1)

	time.Sleep(time.Millisecond)
	after := time.Now()
	reportedSinceAfter, _ := repo.GetReportedSince(after)
	assert.Len(t, reportedSinceAfter, 0)
	resp, _, _ := service.DispatchRequest(
		context.TODO(),
		[]byte{0x08, 0xfe, 0xed, 0xf0, 0x0d},
		&net.UDPAddr{IP: net.ParseIP("1.1.1.1"), Port: 10481},
	)
	assert.Empty(t, resp)
	// the server is now live again
	reportedSinceAfter, _ = repo.GetReportedSince(after)
	assert.Len(t, reportedSinceAfter, 1)
}

func TestReporter_DispatchKeepaliveRequest_Errors(t *testing.T) {
	tests := []struct {
		name    string
		payload []byte
		ipaddr  string
		wantErr error
	}{
		{
			name:    "positive case",
			payload: []byte{0x08, 0xfe, 0xed, 0xf0, 0x0d},
			ipaddr:  "1.1.1.1",
		},
		{
			name:    "unmatched ip address",
			payload: []byte{0x08, 0xfe, 0xed, 0xf0, 0x0d},
			ipaddr:  "2.2.2.2",
			wantErr: server.ErrServerNotFound,
		},
		{
			name:    "unknown instance key",
			payload: []byte{0x08, 0xde, 0xad, 0xbe, 0xef},
			ipaddr:  "1.1.1.1",
			wantErr: server.ErrServerNotFound,
		},
		{
			name:    "unacceptable payload - length",
			payload: []byte{0x08, 0xfe, 0xed},
			ipaddr:  "1.1.1.1",
			wantErr: reporter.ErrInvalidKeepaliveRequest,
		},
		{
			name:    "unacceptable payload - nulls",
			payload: []byte{0x08, 0xfe, 0x00, 0xf0, 0x0d},
			ipaddr:  "1.1.1.1",
			wantErr: reporter.ErrInvalidKeepaliveRequest,
		},
	}
	reportedInstanceID := []byte{0xfe, 0xed, 0xf0, 0x0d}
	reportedServerIP := "1.1.1.1"

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := memory.New()
			service, _ := reporter.NewService(reporter.WithServerRepository(repo))
			// initial heartbeat report
			service.DispatchRequest( // nolint: errcheck
				context.TODO(),
				testutils.PackHeartbeatRequest(reportedInstanceID, testutils.GenServerParams()),
				&net.UDPAddr{IP: net.ParseIP(reportedServerIP), Port: 10481},
			)
			// keepalive request in a while
			time.Sleep(time.Millisecond)
			since := time.Now()
			_, _, err := service.DispatchRequest(
				context.TODO(),
				tt.payload,
				&net.UDPAddr{IP: net.ParseIP(tt.ipaddr), Port: 10481},
			)
			reportedSinceAfter, _ := repo.GetReportedSince(since)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				assert.Len(t, reportedSinceAfter, 0)
			} else {
				require.NoError(t, err)
				assert.Len(t, reportedSinceAfter, 1)
			}
		})
	}
}
