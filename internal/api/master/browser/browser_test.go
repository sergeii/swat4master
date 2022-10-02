package browser_test

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeii/swat4master/internal/api/master/browser"
	"github.com/sergeii/swat4master/internal/api/master/reporter"
	"github.com/sergeii/swat4master/internal/server/memory"
	"github.com/sergeii/swat4master/internal/testutils"
	"github.com/sergeii/swat4master/pkg/binutils"
	"github.com/sergeii/swat4master/pkg/gamespy/browsing"
	"github.com/sergeii/swat4master/pkg/random"
)

func TestMasterBrowserService_HandleRequest_Parse(t *testing.T) {
	tests := []struct {
		name    string
		payload []byte
		wantErr error
	}{
		{
			name: "positive case",
			payload: testutils.PackBrowserRequest(
				[]string{
					"hostname", "maxplayers", "gametype",
					"gamevariant", "mapname", "hostport",
					"password", "gamever", "statsenabled",
				},
				"gametype='VIP Escort' and gamever='1.1'",
				[]byte{0x00, 0x00, 0x00, 0x00},
				testutils.GenBrowserChallenge8,
				testutils.CalcReqLength,
			),
		},
		{
			name: "positive case - filters are optional",
			payload: testutils.PackBrowserRequest(
				[]string{
					"hostname", "maxplayers", "gametype",
					"gamevariant", "mapname", "hostport",
					"password", "gamever", "statsenabled",
				},
				"",
				[]byte{0x00, 0x00, 0x00, 0x00},
				testutils.GenBrowserChallenge8,
				testutils.CalcReqLength,
			),
		},
		{
			name: "positive case - list type 1 is accepted",
			payload: testutils.PackBrowserRequest(
				[]string{"hostname"},
				"",
				[]byte{0x00, 0x00, 0x00, 0x01},
				testutils.GenBrowserChallenge8,
				testutils.CalcReqLength,
			),
		},
		{
			name: "broker filters",
			payload: testutils.PackBrowserRequest(
				[]string{
					"hostname", "maxplayers", "gametype",
					"gamevariant", "mapname", "hostport",
					"password", "gamever", "statsenabled",
				},
				"gametype='VIP Escort' and gamever='1.1",
				[]byte{0x00, 0x00, 0x00, 0x00},
				testutils.GenBrowserChallenge8,
				testutils.CalcReqLength,
			),
		},
		{
			name: "no fields specified",
			payload: testutils.PackBrowserRequest(
				[]string{},
				"gametype='VIP Escort' and gamever='1.1'",
				[]byte{0x00, 0x00, 0x00, 0x00},
				testutils.GenBrowserChallenge8,
				testutils.CalcReqLength,
			),
			wantErr: browsing.ErrNoFieldsRequested,
		},
		{
			name: "invalid challenge length",
			payload: testutils.PackBrowserRequest(
				[]string{
					"hostname", "maxplayers", "gametype",
					"gamevariant", "mapname", "hostport",
					"password", "gamever", "statsenabled",
				},
				"",
				[]byte{0x00, 0x00, 0x00, 0x00},
				func() []byte {
					return testutils.GenBrowserChallenge(7)
				},
				testutils.CalcReqLength,
			),
			wantErr: browsing.ErrInvalidRequestFormat,
		},
		{
			name:    "junk payload",
			payload: random.RandBytes(200),
			wantErr: browsing.ErrInvalidRequestFormat,
		},
		{
			name:    "empty payload",
			payload: []byte{},
			wantErr: browsing.ErrInvalidRequestFormat,
		},
		{
			name:    "null payload",
			payload: []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
			wantErr: browsing.ErrInvalidRequestFormat,
		},
		{
			name: "incomplete payload",
			payload: testutils.PackBrowserRequest(
				[]string{"hostname"},
				"",
				[]byte{0x00, 0x00, 0x00, 0x00},
				testutils.GenBrowserChallenge8,
				testutils.CalcReqLength,
			)[:30],
			wantErr: browsing.ErrInvalidRequestFormat,
		},
		{
			name: "declared length exceeds the bounds",
			payload: testutils.PackBrowserRequest(
				[]string{"hostname"},
				"",
				[]byte{0x00, 0x00, 0x00, 0x00},
				testutils.GenBrowserChallenge8,
				testutils.WithBrowserChallengeLength(400),
			),
			wantErr: browsing.ErrInvalidRequestFormat,
		},
		{
			name: "declared length is low",
			payload: testutils.PackBrowserRequest(
				[]string{"hostname"},
				"",
				[]byte{0x00, 0x00, 0x00, 0x00},
				testutils.GenBrowserChallenge8,
				testutils.WithBrowserChallengeLength(30),
			),
			wantErr: browsing.ErrInvalidRequestFormat,
		},
		{
			name: "declared length is zero",
			payload: testutils.PackBrowserRequest(
				[]string{"hostname"},
				"",
				[]byte{0x00, 0x00, 0x00, 0x00},
				testutils.GenBrowserChallenge8,
				testutils.WithBrowserChallengeLength(0),
			),
			wantErr: browsing.ErrInvalidRequestFormat,
		},
		{
			name: "invalid list type option",
			payload: testutils.PackBrowserRequest(
				[]string{"hostname"},
				"",
				[]byte{0x00, 0x00, 0x00, 0x20},
				testutils.GenBrowserChallenge8,
				testutils.CalcReqLength,
			),
			wantErr: browsing.ErrInvalidRequestFormat,
		},
		{
			name: "invalid option mask length",
			payload: testutils.PackBrowserRequest(
				[]string{"hostname"},
				"",
				[]byte{0x00, 0x00, 0x00, 0x00, 0x00},
				testutils.GenBrowserChallenge8,
				testutils.CalcReqLength,
			),
			wantErr: browsing.ErrInvalidRequestFormat,
		},
	}

	repo := memory.New()
	reportingAPI, _ := reporter.NewService(reporter.WithServerRepository(repo))
	browsingAPI, _ := browser.NewService(browser.WithServerRepository(repo), browser.WithLivenessDuration(time.Hour))
	// prepare the servers
	testutils.SendHeartbeat( // nolint: errcheck
		reportingAPI,
		[]byte{0xf0, 0xf0, 0xf0, 0x0d},
		testutils.WithExtraServerParams(map[string]string{
			"hostname":   "Swat4 Server",
			"gamever":    "1.1",
			"gametype":   "VIP Escort",
			"hostport":   "10480",
			"localport":  "10481",
			"numplayers": "16",
			"maxplayers": "16",
		}),
		testutils.WithRandomAddr(),
	)
	testutils.SendHeartbeat( // nolint: errcheck
		reportingAPI,
		[]byte{0xbe, 0xef, 0xbe, 0xef},
		testutils.WithServerParams(nil),
		testutils.WithRandomAddr(),
	)
	testutils.SendHeartbeat( // nolint: errcheck
		reportingAPI,
		[]byte{0xca, 0xfe, 0xca, 0xfe},
		testutils.WithServerParams(map[string]string{}),
		testutils.WithRandomAddr(),
	)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := browsingAPI.HandleRequest(
				context.TODO(),
				&net.TCPAddr{IP: net.ParseIP("1.1.1.1"), Port: 1337},
				tt.payload,
			)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				assert.Nil(t, resp)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestMasterBrowserService_HandleRequest_ParseResponse(t *testing.T) {
	repo := memory.New()
	reportingAPI, _ := reporter.NewService(reporter.WithServerRepository(repo))
	browsingAPI, _ := browser.NewService(
		browser.WithServerRepository(repo),
		browser.WithLivenessDuration(time.Millisecond*10),
	)

	_, err := testutils.SendHeartbeat(
		reportingAPI,
		[]byte{0xf0, 0xf0, 0xf0, 0x0d},
		testutils.WithExtraServerParams(map[string]string{
			"hostname":  "Swat4 Server",
			"gametype":  "VIP Escort",
			"localport": "10581",
			"hostport":  "10580",
		}),
		testutils.WithCustomAddr("20.20.20.20", 18231), // server is behind nat
	)
	require.NoError(t, err)
	_, err = testutils.SendHeartbeat(
		reportingAPI,
		[]byte{0xbe, 0xef, 0xbe, 0xef},
		testutils.WithExtraServerParams(map[string]string{
			"hostname":  "Another Swat4 Server",
			"gamever":   "1.0",
			"localport": "10481",
			"hostport":  "10480",
		}),
		testutils.WithCustomAddr("30.30.30.30", 10481),
	)
	require.NoError(t, err)

	resp, err := testutils.SendBrowserRequest(browsingAPI, "", testutils.StandardAddr)
	require.NoError(t, err)

	reqIP := net.IPv4(resp[0], resp[1], resp[2], resp[3])
	reqPort := int(binary.BigEndian.Uint16(resp[4:6]))
	assert.Equal(t, "1.1.1.1", reqIP.String())
	assert.Equal(t, 10481, reqPort)

	fieldCount := int(resp[6])
	assert.Equal(t, uint8(0), resp[7])
	assert.Equal(t, 9, fieldCount)
	fields := make([]string, 0, fieldCount)
	unparsed := resp[8:]
	for i := 0; i < fieldCount; i++ {
		field, rem := binutils.ConsumeCString(unparsed)
		assert.True(t, len(field) > 0)
		assert.Equal(t, uint8(0), rem[0])
		// consume extra null byte at the end of the field
		unparsed = rem[1:]
		fields = append(fields, string(field))
	}

	servers := make(map[string]map[string]string)
	for unparsed[0] == 0x51 {
		serverIP := net.IPv4(unparsed[1], unparsed[2], unparsed[3], unparsed[4])
		serverPort := binary.BigEndian.Uint16(unparsed[5:7])
		params := make(map[string]string)
		unparsed = unparsed[7:]
		for i := range fields {
			assert.Equal(t, uint8(0xff), unparsed[0])
			unparsed = unparsed[1:] // skip leading 0xff
			fieldValue, rem := binutils.ConsumeCString(unparsed)
			assert.True(t, len(fieldValue) > 0)
			params[fields[i]] = string(fieldValue)
			unparsed = rem
		}
		key := fmt.Sprintf("%s:%d", serverIP, serverPort)
		servers[key] = params
	}

	assert.Len(t, servers, 2)
	assert.Contains(t, servers, "30.30.30.30:10481")
	assert.Contains(t, servers, "20.20.20.20:10581")
	assert.Equal(t,
		[]string{
			"hostname", "maxplayers", "gametype",
			"gamevariant", "mapname", "hostport",
			"password", "gamever", "statsenabled",
		},
		fields,
	)
	// each server has the fields listed
	for _, svr := range servers {
		for _, f := range fields {
			svrField, ok := svr[f]
			assert.True(t, ok)
			assert.True(t, svrField != "")
		}
	}

	// the remaining bytes
	assert.Equal(t, []byte{0x00, 0xff, 0xff, 0xff, 0xff}, unparsed)
}

func TestMasterBrowserService_HandleRequest_ServerList(t *testing.T) {
	repo := memory.New()
	reportingAPI, _ := reporter.NewService(reporter.WithServerRepository(repo))
	browsingAPI, _ := browser.NewService(
		browser.WithServerRepository(repo),
		browser.WithLivenessDuration(time.Millisecond*10),
	)

	for _, f := range []string{"", "gametype='VIP Escort'"} {
		resp, err := testutils.SendBrowserRequest(browsingAPI, f, testutils.WithRandomAddr())
		require.NoError(t, err)
		assert.Len(t, testutils.UnpackServerList(resp), 0)
	}

	testutils.SendHeartbeat( // nolint: errcheck
		reportingAPI,
		[]byte{0xf0, 0xf0, 0xf0, 0x0d},
		testutils.WithExtraServerParams(map[string]string{"hostname": "Swat4 Server", "gametype": "VIP Escort"}),
		testutils.WithRandomAddr(),
	)
	for _, f := range []string{"", "gametype='VIP Escort'"} {
		resp, err := testutils.SendBrowserRequest(browsingAPI, f, testutils.WithRandomAddr())
		require.NoError(t, err)
		servers := testutils.UnpackServerList(resp)
		assert.Len(t, servers, 1)
		assert.Equal(t, "Swat4 Server", servers[0]["hostname"])
	}

	time.Sleep(time.Millisecond * 20)
	testutils.SendHeartbeat( // nolint: errcheck
		reportingAPI,
		[]byte{0xbe, 0xef, 0xbe, 0xef},
		testutils.WithExtraServerParams(map[string]string{"hostname": "Another Swat4 Server", "gamever": "1.0"}),
		testutils.WithRandomAddr(),
	)
	testutils.SendHeartbeat( // nolint: errcheck
		reportingAPI,
		[]byte{0xca, 0xfe, 0xca, 0xfe},
		testutils.WithExtraServerParams(map[string]string{"hostname": "New Swat4 Server", "gamever": "1.1"}),
		testutils.WithRandomAddr(),
	)
	// only 2 recent servers are available
	resp, err := testutils.SendBrowserRequest(browsingAPI, "", testutils.WithRandomAddr())
	require.NoError(t, err)
	servers := testutils.UnpackServerList(resp)
	assert.Len(t, servers, 2)
	assert.ElementsMatch(t,
		[]string{"Another Swat4 Server", "New Swat4 Server"},
		[]string{servers[0]["hostname"], servers[1]["hostname"]},
	)

	// just 1 server meets the criteria of gamever=1.1
	resp, err = testutils.SendBrowserRequest(browsingAPI, "gamever='1.1'", testutils.WithRandomAddr())
	require.NoError(t, err)
	servers = testutils.UnpackServerList(resp)
	assert.Len(t, servers, 1)
	assert.Equal(t, "New Swat4 Server", servers[0]["hostname"])

	// just 1 server meets the criteria of gamever=1.2
	resp, err = testutils.SendBrowserRequest(browsingAPI, "gamever='1.2'", testutils.WithRandomAddr())
	require.NoError(t, err)
	assert.Len(t, testutils.UnpackServerList(resp), 0)
}

func TestMasterBrowserService_HandleRequest_Filters(t *testing.T) {
	tests := []struct {
		name    string
		filters string
		servers []string
	}{
		{
			name:    "no filters",
			filters: "",
			servers: []string{"Swat4 Server", "Another Swat4 Server", "New Swat4 Server"},
		},
		{
			name:    "vip escort",
			filters: "gametype='VIP Escort'",
			servers: []string{"Swat4 Server", "Another Swat4 Server"},
		},
		{
			name:    "1.0",
			filters: "gamever='1.0'",
			servers: []string{"Another Swat4 Server", "New Swat4 Server"},
		},
		{
			name:    "1.1",
			filters: "gamever='1.1'",
			servers: []string{"Swat4 Server"},
		},
		{
			name:    "no servers matching gamever",
			filters: "gamever='1.2'",
			servers: []string{},
		},
		{
			name:    "vip escort 1.0",
			filters: "gametype='VIP Escort' and gamever='1.0'",
			servers: []string{"Another Swat4 Server"},
		},
		{
			name:    "no servers matching gamever and gametype",
			filters: "gametype='Barricaded Suspects' and gamever='1.1'",
			servers: []string{},
		},
		{
			name:    "no servers matching gametype",
			filters: "gametype='Rapid Deployment'",
			servers: []string{},
		},
		{
			name:    "exclude full servers",
			filters: "numplayers!=maxplayers",
			servers: []string{"Another Swat4 Server"},
		},
		{
			name:    "exclude full servers 1.1",
			filters: "numplayers!=maxplayers and gamever='1.1'",
			servers: []string{},
		},
		{
			name:    "exclude empty servers",
			filters: "numplayers>0",
			servers: []string{"Swat4 Server", "New Swat4 Server"},
		},
		{
			name:    "exclude empty and full servers",
			filters: "numplayers>0 and numplayers!=maxplayers",
			servers: []string{},
		},
		{
			name:    "filter by hostport is allowed",
			filters: "hostport=10480",
			servers: []string{"Swat4 Server", "Another Swat4 Server"},
		},
		{
			name:    "filter by localport is not allowed",
			filters: "localport=10481",
			servers: []string{"Swat4 Server", "New Swat4 Server", "Another Swat4 Server"},
		},
		{
			name:    "filter by localport is not allowed #2",
			filters: "localport=10584",
			servers: []string{"Swat4 Server", "New Swat4 Server", "Another Swat4 Server"},
		},
		{
			name:    "filter by localport is not allowed #3",
			filters: "hostport=localport",
			servers: []string{"Swat4 Server", "New Swat4 Server", "Another Swat4 Server"},
		},
	}

	repo := memory.New()
	reportingAPI, _ := reporter.NewService(reporter.WithServerRepository(repo))
	browsingAPI, _ := browser.NewService(browser.WithServerRepository(repo), browser.WithLivenessDuration(time.Hour))
	// prepare the servers
	testutils.SendHeartbeat( // nolint: errcheck
		reportingAPI,
		[]byte{0xf0, 0xf0, 0xf0, 0x0d},
		testutils.WithExtraServerParams(map[string]string{
			"hostname":   "Swat4 Server",
			"gamever":    "1.1",
			"gametype":   "VIP Escort",
			"hostport":   "10480",
			"localport":  "10481",
			"numplayers": "16",
			"maxplayers": "16",
		}),
		testutils.WithCustomAddr("1.1.1.1", 10481),
	)
	testutils.SendHeartbeat( // nolint: errcheck
		reportingAPI,
		[]byte{0xbe, 0xef, 0xbe, 0xef},
		testutils.WithExtraServerParams(map[string]string{
			"hostname":   "Another Swat4 Server",
			"gamever":    "1.0",
			"gametype":   "VIP Escort",
			"hostport":   "10480",
			"localport":  "10481",
			"numplayers": "0",
			"maxplayers": "16",
		}),
		testutils.WithCustomAddr("2.2.2.2", 10481),
	)
	testutils.SendHeartbeat( // nolint: errcheck
		reportingAPI,
		[]byte{0xbe, 0xef, 0xbe, 0xef},
		testutils.WithExtraServerParams(map[string]string{
			"hostname":   "New Swat4 Server",
			"gamever":    "1.0",
			"gametype":   "Barricaded Suspects",
			"hostport":   "10580",
			"localport":  "10584",
			"numplayers": "12",
			"maxplayers": "12",
		}),
		testutils.WithCustomAddr("3.3.3.3", 17221),
	)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := testutils.SendBrowserRequest(browsingAPI, tt.filters, testutils.WithRandomAddr())
			require.NoError(t, err)
			servers := testutils.UnpackServerList(resp)
			serverNames := make([]string, 0, len(servers))
			for _, svr := range servers {
				serverNames = append(serverNames, svr["hostname"])
			}
			assert.Len(t, servers, len(tt.servers))
			assert.ElementsMatch(t, tt.servers, serverNames)
		})
	}
}
