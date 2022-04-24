package main_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/sergeii/swat4master/cmd/swat4master/config"
	"github.com/sergeii/swat4master/cmd/swat4master/logging"
)

func TestConfigureLogging(t *testing.T) {
	tests := []struct {
		name    string
		cfg     config.Config
		wantErr error
	}{
		{
			"positive case - console/info",
			config.Config{LogOutput: "console", LogLevel: "info"},
			nil,
		},
		{
			"positive case - json/error",
			config.Config{LogOutput: "json", LogLevel: "error"},
			nil,
		},
		{
			"positive case - stdout/panic",
			config.Config{LogOutput: "stdout", LogLevel: "panic"},
			nil,
		},
		{
			"positive case - stderr/warn",
			config.Config{LogOutput: "stderr", LogLevel: "warn"},
			nil,
		},
		{
			"logging level should be lowercase",
			config.Config{LogOutput: "stdout", LogLevel: "INFO"},
			logging.ErrInvalidLogLevel,
		},
		{
			"invalid logging level",
			config.Config{LogOutput: "stdout", LogLevel: "critical"},
			logging.ErrInvalidLogLevel,
		},
		{
			"invalid logging output",
			config.Config{LogOutput: "text", LogLevel: "warn"},
			logging.ErrInvalidLogOutput,
		},
		{
			"invalid logging output and level",
			config.Config{LogOutput: "out", LogLevel: "DEBUG"},
			logging.ErrInvalidLogLevel,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := logging.ConfigureLogging(&tt.cfg)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, got)
			}
		})
	}
}
