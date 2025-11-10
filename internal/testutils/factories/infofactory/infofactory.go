package infofactory

import (
	"maps"

	"github.com/sergeii/swat4master/internal/core/entities/details"
	"github.com/sergeii/swat4master/pkg/slice"
)

type F map[string]string

type BuildOption func(map[string]string)

func WithFields(extra F) BuildOption {
	return func(fields map[string]string) {
		maps.Copy(fields, extra)
	}
}

func Build(opts ...BuildOption) details.Info {
	fields := map[string]string{
		"hostname": slice.RandomChoice([]string{
			"Swat4 Server",
			"Awesome Server",
			"Another Swat4 Server",
			"Pro Server",
		}),
		"hostport": slice.RandomChoice([]string{
			"10480",
			"10580",
		}),
		"mapname":     slice.RandomChoice([]string{"A-Bomb Nightclub", "Food Wall Restaurant", "-EXP- FunTime Amusements"}),
		"gamever":     slice.RandomChoice([]string{"1.0", "1.1"}),
		"gamevariant": slice.RandomChoice([]string{"SWAT 4", "SEF", "SWAT 4X"}),
		"gametype":    slice.RandomChoice([]string{"VIP Escort", "Rapid Deployment", "Barricaded Suspects", "CO-OP"}),
		"numplayers":  "0",
		"maxplayers":  "16",
	}

	for _, opt := range opts {
		opt(fields)
	}

	return details.MustNewInfoFromParams(fields)
}
