package slice_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/sergeii/swat4master/pkg/slice"
)

func TestTruncateSafe_Bytes(t *testing.T) {
	tests := []struct {
		name string
		s    []byte
		n    int
		want int
	}{
		{
			"equal length",
			[]byte{0xca, 0xfe, 0xca, 0xfe},
			4,
			4,
		},
		{
			"truncate to greater",
			[]byte{0xca, 0xfe},
			6,
			2,
		},
		{
			"truncate to zero",
			[]byte{0xca, 0xfe, 0xca, 0xfe},
			0,
			0,
		},
		{
			"initial zero length",
			[]byte{},
			10,
			0,
		},
		{
			"truncate to lesser",
			[]byte{0xca, 0xfe, 0xca, 0xfe},
			2,
			2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			truncated := slice.TruncateSafe(tt.s, tt.n)
			assert.Len(t, truncated, tt.want)
			assert.Equal(t, tt.s[:tt.want], truncated)
		})
	}
}

func TestTruncateSafe_Ints(t *testing.T) {
	tests := []struct {
		name string
		s    []int
		n    int
		want int
	}{
		{
			"equal length",
			[]int{1, 2, 3, 4},
			4,
			4,
		},
		{
			"truncate to greater",
			[]int{2, 3},
			6,
			2,
		},
		{
			"truncate to zero",
			[]int{1, 2, 3, 4},
			0,
			0,
		},
		{
			"initial zero length",
			[]int{},
			10,
			0,
		},
		{
			"truncate to lesser",
			[]int{1, 2, 3, 4},
			2,
			2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			truncated := slice.TruncateSafe(tt.s, tt.n)
			assert.Len(t, truncated, tt.want)
			assert.Equal(t, tt.s[:tt.want], truncated)
		})
	}
}

func TestFirst_OK(t *testing.T) {
	tests := []struct {
		name string
		s    []string
		want string
	}{
		{
			"single element",
			[]string{"a"},
			"a",
		},
		{
			"multiple elements",
			[]string{"a", "b", "c"},
			"a",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := slice.First(tt.s)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFirst_Panic(t *testing.T) {
	assert.Panics(t, func() {
		slice.First([]string{})
	})
}

func TestIntersection(t *testing.T) {
	tests := []struct {
		name string
		s    [][]string
		want []string
	}{
		{
			"common elements",
			[][]string{
				{"a", "b", "c"},
				{"b", "c", "d"},
				{"c", "b"},
			},
			[]string{"b", "c"},
		},
		{
			"one common element",
			[][]string{
				{"a", "b", "c"},
				{"b", "d"},
				{"d", "b"},
			},
			[]string{"b"},
		},
		{
			"no common elements",
			[][]string{
				{"a", "b"},
				{"c", "d"},
				{"e", "f"},
			},
			[]string{},
		},
		{
			"duplicate elements in slices",
			[][]string{
				{"a", "b", "c", "b"},
				{"b", "b", "c", "d", "d"},
				{"c", "b"},
			},
			[]string{"b", "c"},
		},
		{
			"all slices identical",
			[][]string{
				{"a", "b", "c"},
				{"a", "b", "c"},
				{"a", "b", "c"},
			},
			[]string{"a", "b", "c"},
		},
		{
			"all slices identical with duplicates",
			[][]string{
				{"a", "b", "c", "a"},
				{"a", "b", "c", "b"},
				{"a", "b", "c", "c"},
			},
			[]string{"a", "b", "c"},
		},
		{
			"all slices identical with the same element duplicated",
			[][]string{
				{"a", "a", "a"},
				{"a"},
				{"a", "a"},
			},
			[]string{"a"},
		},
		{
			"single slice",
			[][]string{
				{"a", "b", "c"},
			},
			[]string{"a", "b", "c"},
		},
		{
			"single slice with one element",
			[][]string{
				{"a"},
			},
			[]string{"a"},
		},
		{
			"single slice with no elements",
			[][]string{
				{},
			},
			[]string{},
		},
		{
			"no slices",
			[][]string{},
			[]string{},
		},
		{
			"empty slices",
			[][]string{
				{},
				{},
			},
			[]string{},
		},
		{
			"mix of empty and non-empty slices",
			[][]string{
				{"a", "b"},
				{},
				{"a", "b", "c"},
			},
			[]string{},
		},
		{
			"all slices but one are empty",
			[][]string{
				{},
				{"a", "b"},
				{},
			},
			[]string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := slice.Intersection(tt.s...)
			assert.ElementsMatch(t, tt.want, got)
		})
	}
}

func TestDifference(t *testing.T) {
	tests := []struct {
		name   string
		base   []string
		others [][]string
		want   []string
	}{
		{
			"elements in base not in others",
			[]string{"a", "b", "c"},
			[][]string{{"b", "d"}},
			[]string{"a", "c"},
		},
		{
			"all elements in base are in one of the other slices",
			[]string{"a", "b", "c"},
			[][]string{{"a", "b", "c"}},
			[]string{},
		},
		{
			"all elements in base are in all other slices",
			[]string{"a", "b", "c"},
			[][]string{{"a", "b", "c"}, {"a", "b", "c"}},
			[]string{},
		},
		{
			"base slice is empty",
			[]string{},
			[][]string{{"a", "b"}},
			[]string{},
		},
		{
			"one of the other slices is empty",
			[]string{"a", "b", "c"},
			[][]string{},
			[]string{"a", "b", "c"},
		},
		{
			"all other slices are empty",
			[]string{"a", "b", "c"},
			[][]string{{}, {}, {}},
			[]string{"a", "b", "c"},
		},
		{
			"all slices are empty",
			[]string{},
			[][]string{{}, {}, {}},
			[]string{},
		},
		{
			"no other slices",
			[]string{"a", "b", "c"},
			[][]string{},
			[]string{"a", "b", "c"},
		},
		{
			"base slice is empty and no other slices",
			[]string{},
			[][]string{},
			[]string{},
		},
		{
			"duplicate elements in base slice",
			[]string{"a", "b", "b", "c"},
			[][]string{{"b", "d"}},
			[]string{"a", "c"},
		},
		{
			"duplicate elements in other slices",
			[]string{"a", "b", "c"},
			[][]string{{"b", "b", "d"}, {"d", "d"}},
			[]string{"a", "c"},
		},
		{
			"single-element slices",
			[]string{"a"},
			[][]string{{"a"}},
			[]string{},
		},
		{
			"no overlapping elements",
			[]string{"a", "b", "c"},
			[][]string{{"x", "y", "z"}, {"1", "2", "3"}},
			[]string{"a", "b", "c"},
		},
		{
			"mixed empty and non-empty slices",
			[]string{"a", "b", "c"},
			[][]string{{}, {"b"}},
			[]string{"a", "c"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := slice.Difference(tt.base, tt.others...)
			assert.ElementsMatch(t, tt.want, got)
		})
	}
}
