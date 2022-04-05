package binutils_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/sergeii/swat4master/pkg/binutils"
)

func TestConsumeCString(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		wantStr []byte
		wantRem []byte
	}{
		{
			name:    "no null",
			data:    []byte{0xfe, 0xed, 0xf0, 0x0d},
			wantStr: []byte{0xfe, 0xed, 0xf0, 0x0d},
			wantRem: nil,
		},
		{
			name:    "null at the end",
			data:    []byte{0xfe, 0xed, 0xf0, 0x0d, 0x00},
			wantStr: []byte{0xfe, 0xed, 0xf0, 0x0d},
			wantRem: []byte{},
		},
		{
			name:    "null at the beginning",
			data:    []byte{0x00, 0xfe, 0xed, 0xf0, 0x0d},
			wantStr: []byte{},
			wantRem: []byte{0xfe, 0xed, 0xf0, 0x0d},
		},
		{
			name:    "null in the middle",
			data:    []byte{0xfe, 0xed, 0x00, 0xf0, 0x0d},
			wantStr: []byte{0xfe, 0xed},
			wantRem: []byte{0xf0, 0x0d},
		},
		{
			name:    "multiple nulls",
			data:    []byte{0x00, 0x00d, 0x00, 0x00, 0x00},
			wantStr: []byte{},
			wantRem: []byte{0x00d, 0x00, 0x00, 0x00},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// make a copy of the test slice, so we don't mutate it
			data := make([]byte, len(tt.data))
			copy(data, tt.data)
			str, rem := binutils.ConsumeCString(data)
			if tt.wantRem == nil {
				assert.Nil(t, rem)
				assert.Equal(t, tt.data, str)
			} else {
				assert.Equal(t, tt.wantStr, str)
				assert.Equal(t, tt.wantRem, rem)
			}
		})
	}
}
