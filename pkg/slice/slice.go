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
	idx := rand.Intn(len(s)) // nolint:gosec
	return s[idx]
}

func Reverse[T any](s []T) []T {
	i := 0
	j := len(s) - 1
	for i < j {
		s[i], s[j] = s[j], s[i]
		i++
		j--
	}
	return s
}
