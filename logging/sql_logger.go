package logging

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/tracelog"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type SqlLogger struct {
	logger      *zap.SugaredLogger
	minLogLevel tracelog.LogLevel
}

func NewSqlLogger(minLevel tracelog.LogLevel) *SqlLogger {
	enc := zapcore.NewConsoleEncoder(zapcore.EncoderConfig{
		MessageKey: "M", EncodeLevel: zapcore.CapitalColorLevelEncoder,
	})
	core := zapcore.NewCore(enc, zapcore.AddSync(os.Stdout), zapcore.DebugLevel)
	return &SqlLogger{logger: zap.New(core).Sugar(), minLogLevel: minLevel}
}

func (l *SqlLogger) Log(_ context.Context, level tracelog.LogLevel, msg string, data map[string]interface{}) {
	if l.minLogLevel != tracelog.LogLevelTrace || msg != "Query" || data["sql"] == nil {
		return
	}
	sql := strings.ReplaceAll(data["sql"].(string), "\\n", "\n")
	if args, ok := data["args"].([]interface{}); ok && len(args) > 0 {
		switch v := args[0].(type) {
		case map[string]interface{}:
			for k, val := range v {
				sql = strings.ReplaceAll(sql, "@"+k, format(val))
			}
		default:
			val := reflect.ValueOf(args[0])
			if val.Kind() == reflect.Map {
				for _, k := range val.MapKeys() {
					sql = strings.ReplaceAll(sql, "@"+fmt.Sprint(k), format(val.MapIndex(k).Interface()))
				}
			} else {
				for i, arg := range args {
					sql = strings.ReplaceAll(sql, fmt.Sprintf("$%d", i+1), format(arg))
				}
			}
		}
	}
	l.logger.Infof("\n\033[1m--- Executed SQL (%v) ---\033[0m\n%s\n", data["time"].(time.Duration), colorize(sql))
}

func format(v interface{}) string {
	switch val := v.(type) {
	case nil:
		return "\033[3;90mNULL\033[0m"
	case string:
		return "\033[32m'" + strings.ReplaceAll(val, "'", "''") + "'\033[0m"
	case []string:
		return "ARRAY[" + strings.Join(mapStrings(val, func(s string) string {
			return "'" + strings.ReplaceAll(s, "'", "''") + "'"
		}), ", ") + "]"
	case []interface{}:
		return "ARRAY[" + strings.Join(mapStrings(val, func(v interface{}) string {
			switch x := v.(type) {
			case nil:
				return "NULL"
			case string:
				return "'" + strings.ReplaceAll(x, "'", "''") + "'"
			default:
				return fmt.Sprintf("%v", x)
			}
		}), ", ") + "]"
	default:
		return fmt.Sprintf("%v", val)
	}
}

func mapStrings[T any](in []T, f func(T) string) []string {
	out := make([]string, len(in))
	for i, v := range in {
		out[i] = f(v)
	}
	return out
}

var (
	sqlKeywords  = regexp.MustCompile(`(?i)\b(SELECT|FROM|WHERE|JOIN|LEFT|RIGHT|INNER|OUTER|FULL|ON|AS|AND|OR|CASE|WHEN|THEN|ELSE|END|GROUP BY|ORDER BY|LIMIT|OFFSET|INSERT INTO|VALUES|UPDATE|SET|DELETE|DISTINCT|HAVING|UNION|ALL)\b`)
	sqlFunctions = regexp.MustCompile(`(?i)\b(EXTRACT|NOW|COALESCE|ISNULL|TO_TIMESTAMP|COUNT|MAX|MIN|AVG|SUM|ARRAY|JSON_AGG|JSON_BUILD_OBJECT)\b`)
)

func colorize(sql string) string {
	sql = sqlKeywords.ReplaceAllStringFunc(sql, func(m string) string { return "\033[1;36m" + m + "\033[0m" })
	sql = sqlFunctions.ReplaceAllStringFunc(sql, func(m string) string { return "\033[33m" + m + "\033[0m" })
	return sql
}
