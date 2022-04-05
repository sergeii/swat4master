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
