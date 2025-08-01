package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"
)

type LogEntry struct {
	Level    string      `json:"level"`
	Msg      string      `json:"msg"`
	SQL      string      `json:"sql,omitempty"`
	Args     interface{} `json:"args,omitempty"`
	Duration string      `json:"duration,omitempty"`
	Time     interface{} `json:"time,omitempty"` // Can be string or number
	Service  string      `json:"service,omitempty"`
}

func main() {
	scanner := bufio.NewScanner(os.Stdin)

	for scanner.Scan() {
		line := scanner.Text()

		// Try to parse as JSON first
		var entry LogEntry
		if err := json.Unmarshal([]byte(line), &entry); err == nil {
			// If it's a Query log entry, format it nicely
			if entry.Msg == "Query" && entry.SQL != "" {
				formatSQLQuery(entry)
				continue
			}
		} else {
			// Try to extract SQL query using regex if JSON parsing fails
			if formatSQLWithRegex(line) {
				continue
			}
		}

		// Pass through non-SQL lines
		fmt.Println(line)
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Error reading input: %v\n", err)
		os.Exit(1)
	}
}

func formatSQLQuery(entry LogEntry) {
	// Unescape the SQL string - handle both literal \n and actual newlines
	sql := entry.SQL
	sql = strings.ReplaceAll(sql, "\\n", "\n")
	sql = strings.ReplaceAll(sql, "\\t", "\t")

	// Parse duration if available (try both duration and time fields)
	var durationStr string
	if entry.Duration != "" {
		if dur, err := time.ParseDuration(entry.Duration); err == nil {
			durationStr = dur.String()
		}
	} else if entry.Time != nil {
		// The time field is in seconds, convert to duration
		if dur, err := time.ParseDuration(fmt.Sprintf("%vs", entry.Time)); err == nil {
			durationStr = dur.String()
		}
	}

	if durationStr != "" {
		fmt.Printf("\n\033[1m--- Executed SQL (%s) ---\033[0m\n", durationStr)
	} else {
		fmt.Printf("\n\033[1m--- Executed SQL ---\033[0m\n")
	}

	// Substitute variables if args are available
	if entry.Args != nil {
		sql = substituteVariables(sql, entry.Args)
	}

	// Format and colorize the SQL
	fmt.Println(colorizeSQL(sql))

	fmt.Println()
}

func substituteVariables(sql string, args interface{}) string {
	// Handle different types of args
	switch v := args.(type) {
	case []interface{}:
		// Simple array of arguments
		for i, arg := range v {
			placeholder := fmt.Sprintf("$%d", i+1)
			formattedArg := formatArg(arg)
			sql = strings.ReplaceAll(sql, placeholder, formattedArg)
		}
	case map[string]interface{}:
		// Named arguments
		for key, value := range v {
			placeholder := fmt.Sprintf("@%s", key)
			formattedArg := formatArg(value)
			sql = strings.ReplaceAll(sql, placeholder, formattedArg)
		}
	}
	return sql
}

func formatArg(arg interface{}) string {
	switch v := arg.(type) {
	case nil:
		return "\033[3;90mNULL\033[0m"
	case string:
		return fmt.Sprintf("\033[32m'%s'\033[0m", strings.ReplaceAll(v, "'", "''"))
	case []interface{}:
		// Handle arrays
		parts := make([]string, len(v))
		for i, item := range v {
			parts[i] = formatArg(item)
		}
		return fmt.Sprintf("ARRAY[%s]", strings.Join(parts, ", "))
	case []string:
		// Handle string arrays
		parts := make([]string, len(v))
		for i, item := range v {
			parts[i] = formatArg(item)
		}
		return fmt.Sprintf("ARRAY[%s]", strings.Join(parts, ", "))
	default:
		return fmt.Sprintf("\033[33m%v\033[0m", v)
	}
}

func formatSQLWithRegex(line string) bool {
	// For multi-line JSON, we need to read multiple lines
	// This is a simplified approach - in practice, we'd need a more robust JSON parser

	// Try to match the pattern: "msg":"Query","sql":"... followed by the SQL and closing quote
	// This handles the case where SQL contains newlines
	pattern := regexp.MustCompile(`"msg":"Query","sql":"([^"]*(?:\n[^"]*)*)"`)
	matches := pattern.FindStringSubmatch(line)
	if len(matches) > 1 {
		sql := matches[1]
		sql = strings.ReplaceAll(sql, "\\n", "\n")
		sql = strings.ReplaceAll(sql, "\\t", "\t")

		fmt.Printf("\n\033[1m--- Executed SQL ---\033[0m\n")
		fmt.Println(colorizeSQL(sql))
		fmt.Println()
		return true
	}

	// Fallback patterns for other formats
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`"msg":"Query","sql":"([^"]+)"`),
		regexp.MustCompile(`"level":"info","msg":"SQL Query","sql":"([^"]+)","args":([^,]+),"duration":([^}]+)`),
		regexp.MustCompile(`SQL Query.*sql":"([^"]+)"`),
	}

	for _, pattern := range patterns {
		matches := pattern.FindStringSubmatch(line)
		if len(matches) > 1 {
			sql := matches[1]
			sql = strings.ReplaceAll(sql, "\\n", "\n")
			sql = strings.ReplaceAll(sql, "\\t", "\t")

			fmt.Printf("\n\033[1m--- Executed SQL ---\033[0m\n")
			fmt.Println(colorizeSQL(sql))
			fmt.Println()
			return true
		}
	}

	return false
}

var (
	sqlKeywords  = regexp.MustCompile(`(?i)\b(SELECT|FROM|WHERE|JOIN|LEFT|RIGHT|INNER|OUTER|FULL|ON|AS|AND|OR|CASE|WHEN|THEN|ELSE|END|GROUP BY|ORDER BY|LIMIT|OFFSET|INSERT INTO|VALUES|UPDATE|SET|DELETE|DISTINCT|HAVING|UNION|ALL)\b`)
	sqlFunctions = regexp.MustCompile(`(?i)\b(EXTRACT|NOW|COALESCE|ISNULL|TO_TIMESTAMP|COUNT|MAX|MIN|AVG|SUM|ARRAY|JSON_AGG|JSON_BUILD_OBJECT)\b`)
)

func colorizeSQL(sql string) string {
	sql = sqlKeywords.ReplaceAllStringFunc(sql, func(m string) string {
		return "\033[1;36m" + m + "\033[0m"
	})
	sql = sqlFunctions.ReplaceAllStringFunc(sql, func(m string) string {
		return "\033[33m" + m + "\033[0m"
	})
	return sql
}
