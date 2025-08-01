package logging

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/tracelog"
	"go.uber.org/zap"
)

type ReadableSQLLogger struct{ logger *zap.Logger }

func NewReadableSQLLogger(logger *zap.Logger) *ReadableSQLLogger {
	return &ReadableSQLLogger{logger}
}

func (l *ReadableSQLLogger) Log(_ context.Context, level tracelog.LogLevel, msg string, data map[string]interface{}) {
	if msg != "Query" || data["sql"] == nil {
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

	fmt.Printf("Executed SQL (%v)\n%s\n", data["time"].(time.Duration), sql)
}

func format(v interface{}) string {
	switch val := v.(type) {
	case nil:
		return "NULL"
	case string:
		return "'" + strings.ReplaceAll(val, "'", "''") + "'"
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
