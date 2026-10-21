package testutils

import (
	"maps"
	"slices"
)

func MapKeys[K comparable, V any](m map[K]V) []K {
	return slices.AppendSeq(make([]K, 0, len(m)), maps.Keys(m))
}
