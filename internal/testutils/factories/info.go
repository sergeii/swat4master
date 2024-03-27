package factories

import (
	"github.com/sergeii/swat4master/internal/core/entities/details"
	"github.com/sergeii/swat4master/pkg/slice"
)

type BuildInfoOption func(map[string]string)

func WithFields(extra map[string]string) BuildInfoOption {
	return func(fields map[string]string) {
		for k, v := range extra {
			fields[k] = v
		}
	}
}

func BuildInfo(opts ...BuildInfoOption) details.Info {
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
	}

	for _, opt := range opts {
		opt(fields)
	}

	return details.MustNewInfoFromParams(fields)
}
