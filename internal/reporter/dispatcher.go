package reporter

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/rs/zerolog"

	"github.com/sergeii/swat4master/internal/core/entities/master"
	"github.com/sergeii/swat4master/internal/metrics"
)

type Dispatcher struct {
	metrics  *metrics.Collector
	clock    clockwork.Clock
	logger   *zerolog.Logger
	handlers map[master.Msg]Handler
	mutex    sync.Mutex
}

func NewDispatcher(
	metrics *metrics.Collector,
	clock clockwork.Clock,
	logger *zerolog.Logger,
) *Dispatcher {
	return &Dispatcher{
		metrics:  metrics,
		clock:    clock,
		logger:   logger,
		handlers: make(map[master.Msg]Handler),
	}
}

func (d *Dispatcher) Register(msgType master.Msg, handler Handler) error {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	if another, exists := d.handlers[msgType]; exists {
		return fmt.Errorf("handler '%v' has already been registered for msg type '%s'", another, msgType)
	}
	d.handlers[msgType] = handler
	return nil
}

func (d *Dispatcher) Handle(
	ctx context.Context,
	conn *net.UDPConn,
	addr *net.UDPAddr,
	payload []byte,
) {
	reqStarted := d.clock.Now()

	d.logger.Debug().
		Str("type", fmt.Sprintf("0x%02x", payload[0])).Stringer("src", addr).Int("len", len(payload)).
		Msg("Received request")

	d.metrics.ReporterReceived.Add(float64(len(payload)))

	resp, reqType, err := d.dispatch(ctx, payload, addr)
	if err != nil {
		d.metrics.ReporterErrors.WithLabelValues(reqType.String()).Inc()
		d.logger.Error().
			Err(err).
			Stringer("src", addr).Stringer("type", reqType).Int("len", len(payload)).
			Msg("Failed to dispatch request")
		return
	}

	// responses are optional for some request types, such as keepalive requests
	if resp != nil {
		d.logger.Debug().
			Stringer("dst", addr).Int("len", len(resp)).
			Msg("Sending response")
		if _, err = conn.WriteToUDP(resp, addr); err != nil {
			d.logger.Error().
				Err(err).Stringer("dst", addr).Int("len", len(resp)).
				Msg("Failed to send response")
		} else {
			// only account the size of the response if we were able to actually push it through the socket
			d.metrics.ReporterSent.Add(float64(len(resp)))
		}
	}

	d.metrics.ReporterRequests.WithLabelValues(reqType.String()).Inc()
	d.metrics.ReporterDurations.
		WithLabelValues(reqType.String()).
		Observe(time.Since(reqStarted).Seconds())
}

func (d *Dispatcher) dispatch(
	ctx context.Context,
	payload []byte,
	addr *net.UDPAddr,
) ([]byte, master.Msg, error) {
	reqType := master.Msg(payload[0])
	handler, err := d.selectHandler(reqType)
	if err != nil {
		return nil, master.Msg(0), err
	}
	resp, err := handler.Handle(ctx, addr, payload)
	return resp, reqType, err
}

func (d *Dispatcher) selectHandler(msgType master.Msg) (Handler, error) {
	if handler, ok := d.handlers[msgType]; ok {
		return handler, nil
	}
	return nil, fmt.Errorf("no associated handler for message type '%s'", msgType)
}

func ParseInstanceID(payload []byte) ([]byte, []byte, error) {
	if len(payload) < 5 {
		return nil, nil, fmt.Errorf("invalid payload length %d", len(payload))
	}
	id := make([]byte, 4)
	copy(id, payload[1:5])
	return id, payload[5:], nil
}
