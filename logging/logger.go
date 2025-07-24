package logging

import (
	"context"
	"os"
	"time"

	"bridgerton.audius.co/config"
	adapter "github.com/axiomhq/axiom-go/adapters/zap"
	"github.com/axiomhq/axiom-go/axiom"
	"github.com/jackc/pgx/v5/tracelog"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// map application log level string to pgx tracelog level
func GetTraceLogLevel(logLevel string) tracelog.LogLevel {
	switch logLevel {
	case "trace", "debug":
		return tracelog.LogLevelTrace
	case "info":
		return tracelog.LogLevelInfo
	case "warn", "warning":
		return tracelog.LogLevelWarn
	case "error":
		return tracelog.LogLevelError
	default:
		return tracelog.LogLevelNone
	}
}

func NewZapLogger(config config.Config) *zap.Logger {
	// stdout core
	level, err := zapcore.ParseLevel(config.LogLevel)
	if err != nil {
		level = zapcore.InfoLevel
	}
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	consoleEncoder := zapcore.NewJSONEncoder(encoderConfig)
	stdoutCore := zapcore.NewCore(
		consoleEncoder,
		zapcore.AddSync(os.Stdout),
		level,
	)

	var core zapcore.Core = stdoutCore

	// axiom core, if token and dataset are provided
	if config.AxiomToken != "" && config.AxiomDataset != "" {
		axiomAdapter, err := adapter.New(
			adapter.SetClientOptions(
				axiom.SetAPITokenConfig(config.AxiomToken),
				axiom.SetOrganizationID("audius-Lu52"),
			),
			adapter.SetDataset(config.AxiomDataset),
			adapter.SetLevelEnabler(
				zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
					return lvl >= level
				}),
			),
		)
		if err != nil {
			panic(err)
		}

		core = zapcore.NewTee(stdoutCore, axiomAdapter)
	}

	logger := zap.New(core)
	return logger
}

func SyncOnTicks(ctx context.Context, logger *zap.Logger, tickDuration time.Duration) error {
	ticker := time.NewTicker(tickDuration)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := logger.Sync(); err != nil {
				// This specific error occurs when running in a container
				// where /dev/stdout is not a valid file descriptor.
				// We can ignore it since it doesn't affect logging to Axiom.
				// See: https://github.com/uber-go/zap/issues/328#issuecomment-284337436
				if err != os.ErrInvalid {
					logger.Error("failed to sync logger on tick", zap.Error(err))
				}
			}
		}
	}
}
