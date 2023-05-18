package timing

import (
	"github.com/benbjohnson/clock"
)

func Provide() clock.Clock {
	return clock.New()
}
