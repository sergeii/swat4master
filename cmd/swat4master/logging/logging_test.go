package logging_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/sergeii/swat4master/cmd/swat4master/logging"
)

func TestProvide(t *testing.T) {
	tests := []struct {
		name    string
		cfg     logging.Config
		wantErr error
	}{
		{
			"positive case - console/info",
			logging.Config{LogOutput: "console", LogLevel: "info"},
			nil,
		},
		{
			"positive case - json/error",
			logging.Config{LogOutput: "json", LogLevel: "error"},
			nil,
		},
		{
			"positive case - stdout/panic",
			logging.Config{LogOutput: "stdout", LogLevel: "panic"},
			nil,
		},
		{
			"positive case - stderr/warn",
			logging.Config{LogOutput: "stderr", LogLevel: "warn"},
			nil,
		},
		{
			"positive case - case insensitive",
			logging.Config{LogOutput: "stdout", LogLevel: "INFO"},
			nil,
		},
		{
			"invalid logging level",
			logging.Config{LogOutput: "stdout", LogLevel: "critical"},
			logging.ErrInvalidLogLevel,
		},
		{
			"invalid logging output",
			logging.Config{LogOutput: "text", LogLevel: "warn"},
			logging.ErrInvalidLogOutput,
		},
		{
			"invalid logging output and level",
			logging.Config{LogOutput: "out", LogLevel: "debug2"},
			logging.ErrInvalidLogLevel,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := logging.Provide(tt.cfg)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
