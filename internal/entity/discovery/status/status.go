package status

import (
	"fmt"
)

type DiscoveryStatus int

const NoStatus = DiscoveryStatus(0)

const (
	New DiscoveryStatus = 1 << iota
	Master
	Info
	Details
	DetailsRetry
	NoDetails
	Port
	PortRetry
	NoPort
)

func (ds DiscoveryStatus) HasStatus() bool {
	return ds != NoStatus
}

func (ds DiscoveryStatus) String() string {
	switch ds { // nolint: exhaustive
	case New:
		return "new"
	case Master:
		return "master"
	case Info:
		return "info"
	case Details:
		return "details"
	case DetailsRetry:
		return "details_retry"
	case NoDetails:
		return "no_details"
	case Port:
		return "port"
	case PortRetry:
		return "port_retry"
	case NoPort:
		return "no_port"
	}
	return fmt.Sprintf("%d", ds)
}
