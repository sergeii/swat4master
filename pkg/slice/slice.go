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

func First[T any](slice []T) T {
	if len(slice) == 0 {
		panic("empty slice")
	}
	return slice[0]
}

// Intersection computes the intersection of multiple slices of a generic comparable type
// and returns a new slice containing the common elements between all input slices
func Intersection[T comparable](slices ...[]T) []T {
	if len(slices) == 0 {
		return nil
	}

	counter := make(map[T]int)
	want := len(slices)

	for _, slice := range slices {
		seen := make(map[T]struct{}, len(slice))
		for _, value := range slice {
			// the value has already been seen in the current slice
			if _, ok := seen[value]; ok {
				continue
			}
			counter[value]++
			seen[value] = struct{}{}
		}
	}

	result := make([]T, 0)
	for key, count := range counter {
		if count == want {
			result = append(result, key)
		}
	}

	return result
}

// Difference computes the difference between the first slice and the other slices.
// It returns elements in the first slice that are not present in any of the other slices.
func Difference[T comparable](base []T, others ...[]T) []T {
	if len(base) == 0 {
		return nil
	}

	seen := make(map[T]struct{})
	for _, slice := range others {
		for _, value := range slice {
			seen[value] = struct{}{}
		}
	}

	result := make([]T, 0)
	for _, value := range base {
		if _, exists := seen[value]; !exists {
			result = append(result, value)
		}
	}

	return result
}
