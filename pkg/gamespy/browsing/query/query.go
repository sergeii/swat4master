package query

import (
	"errors"
	"strings"

	"github.com/rs/zerolog/log"

	"github.com/sergeii/swat4master/pkg/gamespy/browsing/query/filter"
)

var ErrQueryHasNoFilters = errors.New("provided query contains no valid filters")

type Query struct {
	filters []filter.Filter
}

var Blank Query

func New(filters []filter.Filter) (Query, error) {
	if len(filters) == 0 {
		return Blank, ErrQueryHasNoFilters
	}
	return Query{filters}, nil
}

func NewFromString(query string) (Query, error) {
	var rawFilter string
	unscanned := query
	filters := make([]filter.Filter, 0)
	for len(unscanned) > 0 {
		rawFilter, unscanned = scanFilter(unscanned)
		parsed, err := filter.ParseFilter(rawFilter)
		if err != nil {
			log.Warn().Err(err).Str("filter", rawFilter).Msg("Unable to parse filter")
			return Blank, err
		}
		filters = append(filters, parsed)
	}
	return New(filters)
}

func (q Query) Match(fields any) bool {
	for _, f := range q.filters {
		ok, err := f.Match(fields)
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

func scanFilter(s string) (string, string) {
	i := strings.Index(s, " and ")
	if i == -1 {
		return s, ""
	}
	return s[:i], s[i+5:]
}
