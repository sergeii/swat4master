package application

import (
	"github.com/sergeii/swat4master/internal/core/instances"
	"github.com/sergeii/swat4master/internal/core/probes"
	"github.com/sergeii/swat4master/internal/core/servers"
	"github.com/sergeii/swat4master/internal/services/discovery/finding"
	"github.com/sergeii/swat4master/internal/services/monitoring"
	"github.com/sergeii/swat4master/internal/services/probe"
	"github.com/sergeii/swat4master/internal/services/server"
)

type App struct {
	Servers   servers.Repository
	Instances instances.Repository
	Probes    probes.Repository

	ServerService  *server.Service
	ProbeService   *probe.Service
	FindingService *finding.Service
	MetricService  *monitoring.MetricService
}

func NewApp(
	serversRepo servers.Repository,
	instancesRepo instances.Repository,
	probesRepo probes.Repository,
	serverService *server.Service,
	probeService *probe.Service,
	findingService *finding.Service,
	metrics *monitoring.MetricService,
) *App {
	// svrs := []struct {
	// 	ip    string
	// 	port  int
	// 	query int
	// }{
	// 	{"5.9.50.58", 12480, 12481},
	// 	{"5.9.53.214", 5480, 5481},
	// 	{"193.85.73.35", 10480, 10481},
	// 	{"5.9.50.39", 6480, 6481},
	// 	{"5.9.50.39", 8480, 8481},
	// 	{"5.9.53.214", 8480, 8481},
	// 	{"5.9.53.214", 9480, 9481},
	// 	{"185.15.73.207", 11480, 11481},
	// 	{"5.9.50.58", 6480, 6481},
	// 	{"5.9.50.39", 9480, 9481},
	// 	{"64.187.238.45", 6480, 6481},
	// 	{"64.187.238.44", 9480, 9481},
	// 	{"64.187.238.44", 6480, 6481},
	// 	{"116.203.36.143", 10480, 10481},
	// 	{"195.201.217.123", 40480, 40481},
	// 	{"213.239.209.233", 10480, 10481},
	// 	{"213.239.209.233", 10780, 10781},
	// 	{"213.239.209.233", 10580, 10581},
	// 	{"213.239.209.233", 10880, 10881},
	// 	{"213.239.209.233", 11880, 11881},
	// 	{"195.201.217.123", 30480, 30481},
	// 	{"213.239.209.233", 11780, 11781},
	// 	{"64.187.238.45", 5480, 5481},
	// 	{"64.187.238.42", 16480, 16481},
	// 	{"93.177.67.105", 10480, 10481},
	// 	{"104.238.220.173", 10480, 10481},
	// 	{"176.9.38.43", 10480, 10481},
	// 	{"5.161.64.223", 10480, 10481},
	// 	{"5.9.50.39", 17480, 17481},
	// 	{"192.158.239.15", 10480, 10481},
	// 	{"43.138.39.121", 10520, 10521},
	// 	{"5.161.64.223", 21480, 21481},
	// 	{"5.161.143.239", 10480, 10481},
	// 	{"5.161.143.239", 20480, 20481},
	// 	{"5.161.143.239", 30480, 30481},
	// 	{"5.161.64.223", 31480, 31481},
	// 	{"31.43.157.18", 10480, 10481},
	// 	{"31.43.157.18", 10490, 10491},
	// 	{"66.66.16.162", 10480, 10481},
	// 	{"185.107.96.213", 10480, 10481},
	// 	{"192.169.89.82", 10490, 10491},
	// 	{"176.42.9.171", 11480, 11481},
	// 	{"107.5.44.246", 10485, 10486},
	// 	{"209.89.222.161", 10480, 10481},
	// 	{"209.89.222.161", 10680, 10681},
	// 	{"209.89.222.161", 10780, 10781},
	// 	{"209.89.222.161", 10580, 10581},
	// 	{"209.89.222.161", 11680, 11681},
	// 	{"209.89.222.161", 11480, 11481},
	// 	{"209.89.222.161", 11580, 11581},
	// 	{"148.251.174.202", 10480, 10481},
	// }
	// for _, svr := range svrs {
	// 	s, _ := servers.New(net.ParseIP(svr.ip), svr.port, svr.query)
	// 	serversRepo.AddOrUpdate(context.TODO(), s)
	// }
	return &App{
		Servers:        serversRepo,
		Instances:      instancesRepo,
		Probes:         probesRepo,
		ServerService:  serverService,
		ProbeService:   probeService,
		FindingService: findingService,
		MetricService:  metrics,
	}
}
