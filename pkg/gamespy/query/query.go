package query

import (
	"errors"
	"strings"

	"github.com/rs/zerolog/log"

	"github.com/sergeii/swat4master/pkg/gamespy/query/filter"
)

var ErrQueryHasNoFilters = errors.New("provided query contains no valid filters")

type Query struct {
	filters []filter.Filter
}

func New(query string) (*Query, error) {
	rawFilters := strings.Split(query, " and ")
	filters := make([]filter.Filter, 0, len(rawFilters))
	for _, raw := range rawFilters {
		parsed, err := filter.ParseFilter(raw)
		if err != nil {
			log.Warn().Err(err).Str("filter", raw).Msg("Unable to parse filter")
			return nil, err
		}
		filters = append(filters, parsed)
	}
	if len(filters) == 0 {
		return nil, ErrQueryHasNoFilters
	}
	return &Query{filters}, nil
}

func (q *Query) Match(fieldset map[string]string) bool {
	for _, f := range q.filters {
		ok, err := f.Match(fieldset)
		if err != nil {
			log.Warn().Err(err).Stringer("filter", f).Msg("Unable to apply filter")
			return false
		}
		if !ok {
			return false
		}
	}
	return true
}
