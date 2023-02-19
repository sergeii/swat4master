package persistence

import (
	"github.com/sergeii/swat4master/internal/persistence"
	"github.com/sergeii/swat4master/internal/persistence/memory"
)

func Provide() persistence.Repositories {
	return memory.New()
}
