package status

import (
	"fmt"
	"math"
	"strings"
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

func (ds DiscoveryStatus) BitString() string {
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

func (ds DiscoveryStatus) String() string {
	var bit DiscoveryStatus

	maxBits := int(math.Log2(float64(ds))) + 1 // we also use 1(New)
	bits := make([]string, 0, maxBits)

	for bit = 1; bit <= ds; bit <<= 1 {
		if ds&bit == bit {
			bits = append(bits, bit.BitString())
		}
	}

	return strings.Join(bits, "|")
}
