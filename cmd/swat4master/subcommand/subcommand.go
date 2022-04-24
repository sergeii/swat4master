package subcommand

import (
	"sync"

	"github.com/sergeii/swat4master/cmd/swat4master/config"
)

type GroupContext struct {
	Cfg  *config.Config
	wg   *sync.WaitGroup
	rg   *sync.WaitGroup
	Exit chan struct{}
}

func NewGroupContext(cfg *config.Config, size int) *GroupContext {
	exitCh := make(chan struct{}, size)
	rg := &sync.WaitGroup{}
	rg.Add(size)
	wg := &sync.WaitGroup{}
	wg.Add(size)
	gc := &GroupContext{
		Cfg:  cfg,
		wg:   wg,
		rg:   rg,
		Exit: exitCh,
	}
	return gc
}

func (gc *GroupContext) Ready() {
	gc.rg.Done()
}

func (gc *GroupContext) WaitReady() {
	gc.rg.Wait()
}

func (gc *GroupContext) Quit() {
	gc.wg.Done()
	gc.Exit <- struct{}{}
}

func (gc *GroupContext) WaitQuit() {
	gc.wg.Wait()
}
