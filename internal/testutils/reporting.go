package testutils

import (
	"context"
	"net"

	"github.com/sergeii/swat4master/internal/services/master/reporting"
	"github.com/sergeii/swat4master/pkg/slice"
)

func GenServerParams() map[string]string {
	return map[string]string{
		"localip0":     "192.168.10.72",
		"localip1":     "1.1.1.1",
		"localport":    "10481",
		"gamename":     slice.RandomChoice([]string{"swat4", "swat4xp1"}),
		"hostname":     "Swat4 Server",
		"numplayers":   slice.RandomChoice([]string{"0", "1", "10", "16"}),
		"maxplayers":   "16",
		"gametype":     slice.RandomChoice([]string{"VIP Escort", "Rapid Deployment", "Barricaded Suspects", "CO-OP"}),
		"gamevariant":  slice.RandomChoice([]string{"SWAT 4", "SEF", "SWAT 4X"}),
		"mapname":      slice.RandomChoice([]string{"A-Bomb Nightclub", "Food Wall Restaurant", "-EXP- FunTime Amusements"}),
		"hostport":     "10480",
		"password":     slice.RandomChoice([]string{"0", "1"}),
		"statsenabled": slice.RandomChoice([]string{"0", "1"}),
		"gamever":      slice.RandomChoice([]string{"1.0", "1.1"}),
	}
}

func GenExtraServerParams(extra map[string]string) map[string]string {
	params := GenServerParams()
	for k, v := range extra {
		params[k] = v
	}
	return params
}

func WithServerParams(params map[string]string) func() map[string]string {
	return func() map[string]string {
		return params
	}
}

func WithExtraServerParams(extra map[string]string) func() map[string]string {
	return func() map[string]string {
		return GenExtraServerParams(extra)
	}
}

func SendHeartbeat(
	service *reporting.Service,
	instanceID []byte,
	getParamsFunc func() map[string]string,
	getAddrFunc func() (net.IP, int),
) ([]byte, error) {
	ip, port := getAddrFunc()
	resp, _, err := service.DispatchRequest(
		context.TODO(),
		PackHeartbeatRequest(instanceID, getParamsFunc()),
		&net.UDPAddr{IP: ip, Port: port},
	)
	return resp, err
}

func PackHeartbeatRequest(instanceID []byte, params map[string]string) []byte {
	req := make([]byte, 0)
	req = append(req, 0x03)
	req = append(req, instanceID...)
	for field, value := range params {
		req = append(req, []byte(field)...)
		req = append(req, 0x00)
		if value != "" {
			req = append(req, []byte(value)...)
			req = append(req, 0x00)
		}
	}
	req = append(req, 0x00, 0x00, 0x00)
	for _, field := range []string{"player_", "score_", "ping_"} {
		req = append(req, []byte(field)...)
		req = append(req, 0x00)
	}
	return append(req, 0x00, 0x00, 0x00, 0x00)
}
