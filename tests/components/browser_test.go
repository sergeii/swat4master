package components_test

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"

	"github.com/sergeii/swat4master/cmd/swat4master/application"
	"github.com/sergeii/swat4master/cmd/swat4master/components/browser"
	ds "github.com/sergeii/swat4master/internal/core/entities/discovery/status"
	"github.com/sergeii/swat4master/internal/core/repositories"
	"github.com/sergeii/swat4master/internal/metrics"
	"github.com/sergeii/swat4master/internal/settings"
	tu "github.com/sergeii/swat4master/internal/testutils"
	"github.com/sergeii/swat4master/internal/testutils/factories/serverfactory"
	"github.com/sergeii/swat4master/pkg/binutils"
	gscrypt "github.com/sergeii/swat4master/pkg/gamespy/crypt"
	"github.com/sergeii/swat4master/pkg/random"
	"github.com/sergeii/swat4master/tests/testapp"
)

func makeAppWithBrowser(extra ...fx.Option) (*fx.App, func()) {
	fxopts := []fx.Option{
		fx.Provide(testapp.NoLogging),
		fx.Provide(testapp.ProvideSettings),
		fx.Provide(testapp.ProvidePersistence),
		application.Module,
		fx.Decorate(func(settings settings.Settings) settings.Settings {
			settings.ServerLiveness = time.Hour
			return settings
		}),
		fx.Supply(browser.Config{
			ListenAddr:    "localhost:13382",
			ClientTimeout: time.Millisecond * 100,
		}),
		browser.Module,
		fx.NopLogger,
		fx.Invoke(func(_ *browser.Component) {}),
	}
	fxopts = append(fxopts, extra...)
	app := fx.New(fxopts...)
	return app, func() {
		app.Stop(context.TODO()) // nolint: errcheck
	}
}

func TestBrowser_Filters(t *testing.T) {
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

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var serverRepo repositories.ServerRepository

			ctx := context.TODO()
			app, cancel := makeAppWithBrowser(
				fx.Populate(&serverRepo),
			)
			defer cancel()
			tu.MustNoErr(app.Start(ctx))

			serverfactory.Create(
				ctx,
				serverRepo,
				serverfactory.WithAddress("1.1.1.1", 10480),
				serverfactory.WithQueryPort(10481),
				serverfactory.WithDiscoveryStatus(ds.Master|ds.Info),
				serverfactory.WithInfo(map[string]string{
					"hostname":   "Swat4 Server",
					"gamever":    "1.1",
					"gametype":   "VIP Escort",
					"hostport":   "10480",
					"localport":  "10481",
					"numplayers": "16",
					"maxplayers": "16",
				}),
				serverfactory.WithRefreshedAt(time.Now()),
			)

			serverfactory.Create(
				ctx,
				serverRepo,
				serverfactory.WithAddress("2.2.2.2", 10480),
				serverfactory.WithQueryPort(10481),
				serverfactory.WithDiscoveryStatus(ds.Master|ds.Info),
				serverfactory.WithInfo(map[string]string{
					"hostname":   "Another Swat4 Server",
					"gamever":    "1.0",
					"gametype":   "VIP Escort",
					"hostport":   "10480",
					"localport":  "10481",
					"numplayers": "0",
					"maxplayers": "16",
				}),
				serverfactory.WithRefreshedAt(time.Now()),
			)

			serverfactory.Create(
				ctx,
				serverRepo,
				serverfactory.WithAddress("3.3.3.3", 10580),
				serverfactory.WithQueryPort(10584),
				serverfactory.WithDiscoveryStatus(ds.Master|ds.Info),
				serverfactory.WithInfo(map[string]string{
					"hostname":   "New Swat4 Server",
					"gamever":    "1.0",
					"gametype":   "Barricaded Suspects",
					"hostport":   "10580",
					"localport":  "10584",
					"numplayers": "12",
					"maxplayers": "12",
				}),
				serverfactory.WithRefreshedAt(time.Now()),
			)

			// Server is outdated
			serverfactory.Create(
				ctx,
				serverRepo,
				serverfactory.WithRandomAddress(),
				serverfactory.WithDiscoveryStatus(ds.Master|ds.Info|ds.Details),
				serverfactory.WithInfo(map[string]string{
					"hostname":   "VIP Server",
					"gamever":    "1.1",
					"gametype":   "VIP Escort",
					"hostport":   "10480",
					"localport":  "10481",
					"numplayers": "0",
					"maxplayers": "16",
				}),
				serverfactory.WithRefreshedAt(time.Now().Add(-time.Minute*61)),
			)

			// Server is active but has no master status
			serverfactory.Create(
				ctx,
				serverRepo,
				serverfactory.WithRandomAddress(),
				serverfactory.WithDiscoveryStatus(ds.Info|ds.Details),
				serverfactory.WithInfo(map[string]string{
					"hostname":   "Some Swat4 Server",
					"gamever":    "1.1",
					"gametype":   "VIP Escort",
					"hostport":   "10480",
					"localport":  "10481",
					"numplayers": "10",
					"maxplayers": "16",
				}),
				serverfactory.WithRefreshedAt(time.Now()),
			)

			resp := tu.SendBrowserRequest("localhost:13382", tt.filters)
			filteredServers := tu.UnpackServerList(resp)

			serverNames := make([]string, 0, len(filteredServers))
			for _, svr := range filteredServers {
				serverNames = append(serverNames, svr["hostname"])
			}
			assert.Len(t, filteredServers, len(tt.servers))
			assert.ElementsMatch(t, tt.servers, serverNames)
		})
	}
}

func TestBrowser_ParseResponse(t *testing.T) {
	var repo repositories.ServerRepository

	ctx := context.TODO()
	app, cancel := makeAppWithBrowser(fx.Populate(&repo))
	defer cancel()
	tu.MustNoErr(app.Start(ctx))

	serverfactory.Create(
		ctx,
		repo,
		serverfactory.WithAddress("20.20.20.20", 10580),
		serverfactory.WithQueryPort(10581),
		serverfactory.WithDiscoveryStatus(ds.Master),
		serverfactory.WithInfo(map[string]string{
			"hostname":    "Swat4 Server",
			"hostport":    "10580",
			"mapname":     "A-Bomb Nightclub",
			"gamever":     "1.1",
			"gamevariant": "SWAT 4",
			"gametype":    "VIP Escort",
		}),
		serverfactory.WithRefreshedAt(time.Now()),
	)

	serverfactory.Create(
		ctx,
		repo,
		serverfactory.WithAddress("30.30.30.30", 10480),
		serverfactory.WithQueryPort(10481),
		serverfactory.WithDiscoveryStatus(ds.Master),
		serverfactory.WithInfo(map[string]string{
			"hostname":    "Another Swat4 Server",
			"hostport":    "10480",
			"mapname":     "A-Bomb Nightclub",
			"gamever":     "1.0",
			"gamevariant": "SWAT 4",
			"gametype":    "Barricaded Suspects",
		}),
		serverfactory.WithRefreshedAt(time.Now()),
	)

	resp := tu.SendBrowserRequest("localhost:13382", "")

	reqIP := net.IPv4(resp[0], resp[1], resp[2], resp[3])
	// reqPort := int(binary.BigEndian.Uint16(resp[4:6]))
	assert.Equal(t, "127.0.0.1", reqIP.String())
	// assert.Equal(t, 10481, reqPort)

	fieldCount := int(resp[6])
	assert.Equal(t, uint8(0), resp[7])
	assert.Equal(t, 9, fieldCount)
	fields := make([]string, 0, fieldCount)
	unparsed := resp[8:]
	for range fieldCount {
		field, rem := binutils.ConsumeCString(unparsed)
		assert.True(t, len(field) > 0)
		assert.Equal(t, uint8(0), rem[0])
		// consume extra null byte at the end of the field
		unparsed = rem[1:]
		fields = append(fields, string(field))
	}

	list := make(map[string]map[string]string)
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
		list[key] = params
	}

	assert.Len(t, list, 2)
	assert.Contains(t, list, "30.30.30.30:10481")
	assert.Contains(t, list, "20.20.20.20:10581")
	assert.Equal(t,
		[]string{
			"hostname", "maxplayers", "gametype",
			"gamevariant", "mapname", "hostport",
			"password", "gamever", "statsenabled",
		},
		fields,
	)
	// each server has the fields listed
	for _, svr := range list {
		for _, f := range fields {
			svrField, ok := svr[f]
			assert.True(t, ok)
			assert.True(t, svrField != "")
		}
	}

	// the remaining bytes
	assert.Equal(t, []byte{0x00, 0xff, 0xff, 0xff, 0xff}, unparsed)
}

func TestBrowser_ValidateRequest(t *testing.T) {
	tests := []struct {
		name             string
		fields           []string
		filters          string
		options          []byte
		getChallengeFunc func() []byte
		getLengthFunc    func([]byte) int
		wantResp         bool
	}{
		{
			name: "positive case",
			fields: []string{
				"hostname", "maxplayers", "gametype",
				"gamevariant", "mapname", "hostport",
				"password", "gamever", "statsenabled",
			},
			filters:          "gametype='VIP Escort' and gamever='1.1'",
			options:          []byte{0x00, 0x00, 0x00, 0x00},
			getChallengeFunc: tu.GenBrowserChallenge8,
			getLengthFunc:    tu.CalcReqLength,
			wantResp:         true,
		},
		{
			name: "positive case - filters are optional",
			fields: []string{
				"hostname", "maxplayers", "gametype",
				"gamevariant", "mapname", "hostport",
				"password", "gamever", "statsenabled",
			},
			filters:          "",
			options:          []byte{0x00, 0x00, 0x00, 0x00},
			getChallengeFunc: tu.GenBrowserChallenge8,
			getLengthFunc:    tu.CalcReqLength,
			wantResp:         true,
		},
		{
			name:             "positive case - list type 1 is accepted",
			fields:           []string{"hostname"},
			filters:          "",
			options:          []byte{0x00, 0x00, 0x00, 0x01},
			getChallengeFunc: tu.GenBrowserChallenge8,
			getLengthFunc:    tu.CalcReqLength,
			wantResp:         true,
		},
		{
			name: "broken filters are skipped",
			fields: []string{
				"hostname", "maxplayers", "gametype",
				"gamevariant", "mapname", "hostport",
				"password", "gamever", "statsenabled",
			},
			filters:          "gametype='VIP Escort' and gamever='1.1",
			options:          []byte{0x00, 0x00, 0x00, 0x00},
			getChallengeFunc: tu.GenBrowserChallenge8,
			getLengthFunc:    tu.CalcReqLength,
			wantResp:         true,
		},
		{
			name:             "no fields specified",
			fields:           []string{},
			filters:          "gametype='VIP Escort' and gamever='1.1'",
			options:          []byte{0x00, 0x00, 0x00, 0x00},
			getChallengeFunc: tu.GenBrowserChallenge8,
			getLengthFunc:    tu.CalcReqLength,
			wantResp:         false,
		},
		{
			name: "invalid challenge length",
			fields: []string{
				"hostname", "maxplayers", "gametype",
				"gamevariant", "mapname", "hostport",
				"password", "gamever", "statsenabled",
			},
			filters: "",
			options: []byte{0x00, 0x00, 0x00, 0x00},
			getChallengeFunc: func() []byte {
				return tu.GenBrowserChallenge(7)
			},
			getLengthFunc: tu.CalcReqLength,
			wantResp:      false,
		},
		{
			name:             "declared length exceeds the bounds",
			fields:           []string{"hostname"},
			filters:          "",
			options:          []byte{0x00, 0x00, 0x00, 0x00},
			getChallengeFunc: tu.GenBrowserChallenge8,
			getLengthFunc:    tu.WithBrowserChallengeLength(400),
			wantResp:         false,
		},
		{
			name:             "declared length is low",
			fields:           []string{"hostname"},
			filters:          "",
			options:          []byte{0x00, 0x00, 0x00, 0x00},
			getChallengeFunc: tu.GenBrowserChallenge8,
			getLengthFunc:    tu.WithBrowserChallengeLength(30),
			wantResp:         false,
		},
		{
			name:             "declared length is zero",
			fields:           []string{"hostname"},
			filters:          "",
			options:          []byte{0x00, 0x00, 0x00, 0x00},
			getChallengeFunc: tu.GenBrowserChallenge8,
			getLengthFunc:    tu.WithBrowserChallengeLength(0),
			wantResp:         false,
		},
		{
			name:             "invalid list type option",
			fields:           []string{"hostname"},
			filters:          "",
			options:          []byte{0x00, 0x00, 0x00, 0x20},
			getChallengeFunc: tu.GenBrowserChallenge8,
			getLengthFunc:    tu.CalcReqLength,
			wantResp:         false,
		},
		{
			name:             "invalid option mask length",
			fields:           []string{"hostname"},
			filters:          "",
			options:          []byte{0x00, 0x00, 0x00, 0x00, 0x00},
			getChallengeFunc: tu.GenBrowserChallenge8,
			getLengthFunc:    tu.CalcReqLength,
			wantResp:         false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gameKey [6]byte
			var clientChallenge [8]byte
			var serverRepo repositories.ServerRepository
			var collector *metrics.Collector

			ctx := context.TODO()
			app, cancel := makeAppWithBrowser(
				fx.Populate(&serverRepo, &collector),
			)
			defer cancel()
			tu.MustNoErr(app.Start(ctx))

			serverfactory.Create(
				ctx,
				serverRepo,
				serverfactory.WithAddress("1.1.1.1", 10480),
				serverfactory.WithQueryPort(10481),
				serverfactory.WithDiscoveryStatus(ds.Master|ds.Info),
				serverfactory.WithInfo(map[string]string{
					"hostname":   "Swat4 Server",
					"gamever":    "1.1",
					"gametype":   "VIP Escort",
					"hostport":   "10480",
					"localport":  "10481",
					"numplayers": "16",
					"maxplayers": "16",
				}),
				serverfactory.WithRefreshedAt(time.Now()),
			)

			challenge := tt.getChallengeFunc()
			copy(gameKey[:], "tG3j8c")
			copy(clientChallenge[:], challenge)
			payload := tu.PackBrowserRequest(
				tt.fields,
				tt.filters,
				tt.options,
				func() []byte {
					return challenge
				},
				tt.getLengthFunc,
			)

			client := tu.NewTCPClient("localhost:13382", 2048, time.Millisecond*10)
			defer client.Close()
			respEnc, err := client.Send(payload)

			metricErrors := testutil.ToFloat64(collector.BrowserErrors)
			metricSent := testutil.ToFloat64(collector.BrowserSent)

			if tt.wantResp {
				require.NoError(t, err)
				resp := gscrypt.Decrypt(gameKey, clientChallenge, respEnc)
				servers := tu.UnpackServerList(resp)
				assert.Len(t, servers, 1)
				assert.True(t, metricSent > 0)
				assert.Equal(t, float64(0), metricErrors)
			} else {
				assert.ErrorIs(t, err, io.EOF)
				assert.Equal(t, float64(0), metricSent)
				assert.Equal(t, float64(1), metricErrors)
			}
		})
	}
}

func TestBrowser_IgnoreInvalidPayload(t *testing.T) {
	tests := []struct {
		name    string
		payload []byte
	}{
		{
			name:    "junk payload",
			payload: random.RandBytes(200),
		},
		{
			name:    "null payload",
			payload: []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
		},
		{
			name: "incomplete payload",
			payload: tu.PackBrowserRequest(
				[]string{"hostname"},
				"",
				[]byte{0x00, 0x00, 0x00, 0x00},
				tu.GenBrowserChallenge8,
				tu.CalcReqLength,
			)[:30],
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var serverRepo repositories.ServerRepository
			var collector *metrics.Collector

			ctx := context.TODO()
			app, cancel := makeAppWithBrowser(
				fx.Populate(&serverRepo, &collector),
			)
			defer cancel()
			tu.MustNoErr(app.Start(ctx))

			serverfactory.Create(
				ctx,
				serverRepo,
				serverfactory.WithAddress("1.1.1.1", 10480),
				serverfactory.WithQueryPort(10481),
				serverfactory.WithDiscoveryStatus(ds.Master|ds.Info),
				serverfactory.WithInfo(map[string]string{
					"hostname":   "Swat4 Server",
					"gamever":    "1.1",
					"gametype":   "VIP Escort",
					"hostport":   "10480",
					"localport":  "10481",
					"numplayers": "16",
					"maxplayers": "16",
				}),
			)

			client := tu.NewTCPClient("localhost:13382", 2048, time.Millisecond*10)
			defer client.Close()
			_, err := client.Send(tt.payload)
			assert.ErrorIs(t, err, io.EOF)

			metricErrors := testutil.ToFloat64(collector.BrowserErrors)
			metricSent := testutil.ToFloat64(collector.BrowserSent)

			assert.Equal(t, float64(0), metricSent)
			assert.Equal(t, float64(1), metricErrors)
		})
	}
}
