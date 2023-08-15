package slice

import (
	"math/rand"
)

func TruncateSafe[T any](s []T, n int) []T {
	switch {
	case len(s) > n:
		return s[:n]
	default:
		return s
	}
}

func RandomChoice[T any](s []T) T {
	idx := rand.Intn(len(s)) // nolint: gosec // no need for crypto/rand here
	return s[idx]
}
