package settings

import (
	"time"
)

type Settings struct {
	ServerLiveness time.Duration

	DiscoveryRevivalRetries int
	DiscoveryRefreshRetries int
}
