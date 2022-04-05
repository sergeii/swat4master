package testutils

import (
	"context"
	"net"

	"github.com/sergeii/swat4master/internal/api/master/reporter"
)

func SendHeartbeat(
	service *reporter.MasterReporterService,
	instanceID []byte,
	getParamsFunc func() map[string]string,
	getAddrFunc func() (net.IP, int),
) ([]byte, error) {
	ip, port := getAddrFunc()
	return service.DispatchRequest(
		context.TODO(),
		PackHeartbeatRequest(instanceID, getParamsFunc()),
		&net.UDPAddr{IP: ip, Port: port},
	)
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
