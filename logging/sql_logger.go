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

type ReadableSQLLogger struct {
	logger      *zap.SugaredLogger
	minLogLevel tracelog.LogLevel
}

func NewReadableSQLLogger(minLevel tracelog.LogLevel) *ReadableSQLLogger {
	enc := zapcore.NewConsoleEncoder(zapcore.EncoderConfig{
		MessageKey: "M", EncodeLevel: zapcore.CapitalColorLevelEncoder,
	})
	core := zapcore.NewCore(enc, zapcore.AddSync(os.Stdout), zapcore.DebugLevel)
	return &ReadableSQLLogger{logger: zap.New(core).Sugar(), minLogLevel: minLevel}
}

func (l *ReadableSQLLogger) Log(_ context.Context, level tracelog.LogLevel, msg string, data map[string]interface{}) {
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
	l.logger.Infof("\n\033[1m--- Executed SQL (%v) ---\033[0m\n%s\n", data["time"].(time.Duration), colorizeSQL(sql))
}

func format(v interface{}) string {
	switch val := v.(type) {
	case nil:
		return "\033[3;90mNULL\033[0m"
	case string:
		return "\033[32m'" + strings.ReplaceAll(val, "'", "''") + "'\033[0m"
	case []interface{}:
		var parts []string
		for _, e := range val {
			parts = append(parts, format(e))
		}
		return "ARRAY[" + strings.Join(parts, ", ") + "]"
	default:
		return fmt.Sprintf("%v", val)
	}
}

// SQL formatting regexes
var (
	sqlKeywords  = regexp.MustCompile(`(?i)\b(SELECT|FROM|WHERE|JOIN|LEFT|RIGHT|INNER|OUTER|FULL|ON|AS|AND|OR|CASE|WHEN|THEN|ELSE|END|GROUP BY|ORDER BY|LIMIT|OFFSET|INSERT INTO|VALUES|UPDATE|SET|DELETE|DISTINCT|HAVING|UNION|ALL)\b`)
	sqlFunctions = regexp.MustCompile(`(?i)\b(EXTRACT|NOW|COALESCE|ISNULL|TO_TIMESTAMP|COUNT|MAX|MIN|AVG|SUM|ARRAY|JSON_AGG|JSON_BUILD_OBJECT)\b`)
)

func colorizeSQL(sql string) string {
	sql = sqlKeywords.ReplaceAllStringFunc(sql, func(m string) string { return "\033[1;36m" + m + "\033[0m" })
	sql = sqlFunctions.ReplaceAllStringFunc(sql, func(m string) string { return "\033[33m" + m + "\033[0m" })
	return sql
}
