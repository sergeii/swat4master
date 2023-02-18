package logging

import (
	"errors"
	"io"
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"go.uber.org/fx"

	"github.com/sergeii/swat4master/cmd/swat4master/config"
	"github.com/sergeii/swat4master/pkg/logutils"
)

var (
	ErrInvalidLogOutput = errors.New("unknown logging output format")
	ErrInvalidLogLevel  = errors.New("unknown logging level")
)

type Result struct {
	fx.Out

	Logger   *zerolog.Logger
	LogLevel zerolog.Level
}

func Provide(cfg config.Config) (Result, error) {
	var output io.Writer
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnixMicro
	zerolog.DurationFieldUnit = time.Second
	zerolog.CallerMarshalFunc = logutils.ShortCallerFormatter

	lvl, err := zerolog.ParseLevel(cfg.LogLevel)
	if err != nil {
		return Result{}, ErrInvalidLogLevel
	}
	zerolog.SetGlobalLevel(lvl)

	switch cfg.LogOutput {
	case "console", "":
		output = zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339}
	case "stdout":
		output = zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339, NoColor: true}
	case "stderr":
		output = zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339, NoColor: true}
	case "json":
		output = nil
	default:
		return Result{}, ErrInvalidLogOutput
	}

	logger := log.With().Caller().Logger()
	if output != nil {
		logger = logger.Output(output)
	}

	result := Result{
		Logger:   &logger,
		LogLevel: lvl,
	}
	return result, nil
}

func NoGlobal() {
	log.Logger = zerolog.Nop()
}
