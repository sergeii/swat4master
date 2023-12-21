package clock

import (
	"github.com/jonboulle/clockwork"
)

func Provide() clockwork.Clock {
	return clockwork.NewRealClock()
}
