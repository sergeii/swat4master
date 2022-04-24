package running

import (
	"context"
	"sync"

	"github.com/sergeii/swat4master/cmd/swat4master/config"
	"github.com/sergeii/swat4master/internal/application"
)

type Runner struct {
	app  *application.App
	cfg  config.Config
	wg   *sync.WaitGroup
	rg   *sync.WaitGroup
	Exit chan struct{}
}

func NewRunner(app *application.App, cfg config.Config) *Runner {
	exitCh := make(chan struct{})
	rg := &sync.WaitGroup{}
	wg := &sync.WaitGroup{}
	sc := &Runner{
		app:  app,
		cfg:  cfg,
		wg:   wg,
		rg:   rg,
		Exit: exitCh,
	}
	return sc
}

func (rnr *Runner) Add(
	runnable func(context.Context, *Runner, *application.App, config.Config),
	ctx context.Context, // nolint: revive
) {
	go runnable(ctx, rnr, rnr.app, rnr.cfg)
	rnr.wg.Add(1)
	rnr.rg.Add(1)
}

func (rnr *Runner) Ready() {
	rnr.rg.Done()
}

func (rnr *Runner) WaitReady() {
	rnr.rg.Wait()
}

func (rnr *Runner) Quit(exiting <-chan struct{}) {
	rnr.wg.Done()
	select {
	// an exit message is only read once for the first exiting service
	// effectively making successive sends blocked
	case rnr.Exit <- struct{}{}:
	case <-exiting:
	}
}

func (rnr *Runner) WaitQuit() {
	rnr.wg.Wait()
}
