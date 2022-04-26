package logging

import (
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/sergeii/swat4master/cmd/swat4master/config"
	"github.com/sergeii/swat4master/pkg/logutils"
)

var (
	ErrInvalidLogOutput = errors.New("unknown logging output format")
	ErrInvalidLogLevel  = errors.New("unknown logging level")
)

func ConfigureLogging(cfg *config.Config) (zerolog.Logger, error) {
	var output io.Writer
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnixMicro
	zerolog.DurationFieldUnit = time.Second
	zerolog.CallerMarshalFunc = logutils.ShortCallerFormatter

	lvl, err := zerolog.ParseLevel(cfg.LogLevel)
	if err != nil {
		return zerolog.Logger{}, ErrInvalidLogLevel
	}
	zerolog.SetGlobalLevel(lvl)
	fmt.Fprintf(os.Stderr, "Global logging level is set to %s\n", zerolog.GlobalLevel())

	switch cfg.LogOutput {
	case "console":
		output = zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339}
	case "stdout":
		output = zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339, NoColor: true}
	case "stderr":
		output = zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339, NoColor: true}
	case "json":
		output = nil
	default:
		return zerolog.Logger{}, ErrInvalidLogOutput
	}

	logger := log.With().Caller().Logger()
	if output != nil {
		logger = logger.Output(output)
	}
	return logger, nil
}
