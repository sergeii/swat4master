package persistence

import (
	"github.com/benbjohnson/clock"

	"github.com/sergeii/swat4master/internal/persistence"
	"github.com/sergeii/swat4master/internal/persistence/memory"
)

func Provide(c clock.Clock) persistence.Repositories {
	return memory.New(c)
}
