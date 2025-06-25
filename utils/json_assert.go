package utils

import (
	"fmt"
	"testing"

	"github.com/test-go/testify/require"
	"github.com/tidwall/gjson"
)

func JsonAssert(t *testing.T, body []byte, expectations map[string]any) {
	for path, expectation := range expectations {
		var actual any
		switch v := expectation.(type) {
		case string:
			actual = gjson.GetBytes(body, path).String()
		case bool:
			actual = gjson.GetBytes(body, path).Bool()
		case float64:
			actual = gjson.GetBytes(body, path).Float()
		case int:
			actual = int(gjson.GetBytes(body, path).Int())
		default:
			t.Errorf("unsupported type for expectation: %T", v)
		}
		msg := fmt.Sprintf("Expected %s to be %v got %v", path, expectation, actual)
		require.Equal(t, expectation, actual, msg)
	}
}
