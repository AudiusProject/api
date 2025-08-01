package logging

import (
	"context"

	"github.com/jackc/pgx/v5/tracelog"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type SqlLogger struct {
	logger      *zap.Logger
	minZapLevel zapcore.Level
}

func NewSqlLogger(logger *zap.Logger, minZapLevel zapcore.Level) *SqlLogger {
	return &SqlLogger{
		logger:      logger.WithOptions(zap.AddCallerSkip(1)),
		minZapLevel: minZapLevel,
	}
}

func (l *SqlLogger) Log(ctx context.Context, level tracelog.LogLevel, msg string, data map[string]interface{}) {
	// Only log Query statements, not Prepare statements
	if msg != "Query" {
		return
	}

	// Only log SQL queries when configured level is debug or lower
	// This means no SQL logs when level is info, warn, or error
	if l.minZapLevel > zapcore.DebugLevel {
		return
	}

	// Convert tracelog level to zap level
	var zapLevel zapcore.Level
	switch level {
	case tracelog.LogLevelTrace:
		zapLevel = zapcore.DebugLevel
	case tracelog.LogLevelDebug:
		zapLevel = zapcore.DebugLevel
	case tracelog.LogLevelInfo:
		zapLevel = zapcore.InfoLevel
	case tracelog.LogLevelWarn:
		zapLevel = zapcore.WarnLevel
	case tracelog.LogLevelError:
		zapLevel = zapcore.ErrorLevel
	default:
		zapLevel = zapcore.ErrorLevel
	}

	// Only log if the zap level meets our minimum threshold
	if zapLevel < l.minZapLevel {
		return
	}

	// Create fields from data map (like pgx-zap does)
	fields := make([]zapcore.Field, 0, len(data))
	for k, v := range data {
		fields = append(fields, zap.Any(k, v))
	}

	// Log with appropriate level
	switch zapLevel {
	case zapcore.DebugLevel:
		l.logger.Debug(msg, fields...)
	case zapcore.InfoLevel:
		l.logger.Info(msg, fields...)
	case zapcore.WarnLevel:
		l.logger.Warn(msg, fields...)
	case zapcore.ErrorLevel:
		l.logger.Error(msg, fields...)
	}
}
