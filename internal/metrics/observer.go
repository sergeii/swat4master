package metrics

import (
	"context"
)

type Observer interface {
	Observe(ctx context.Context, metrics *Collector)
}
