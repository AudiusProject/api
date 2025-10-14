package common

import (
	"fmt"
	"testing"
	"time"

	"github.com/test-go/testify/assert"
)

func TestRetryUtil(t *testing.T) {
	attempts := 0
	err := WithRetries(func() error {
		attempts++
		if attempts < 3 {
			return fmt.Errorf("fail")
		}
		return nil
	}, 3, time.Millisecond*10)
	assert.NoError(t, err)
	assert.Equal(t, 3, attempts)
}

func TestRetryUtil_Error(t *testing.T) {
	attempts := 0
	err := WithRetries(func() error {
		attempts++
		return fmt.Errorf("fail")
	}, 3, time.Millisecond*10)
	assert.Error(t, err)
	assert.Equal(t, 4, attempts) // initial attempt + 3 retries
}

func TestRetryUtilResult(t *testing.T) {
	attempts := 0
	result, err := WithRetriesResult(func() (string, error) {
		attempts++
		if attempts < 3 {
			return "", fmt.Errorf("fail")
		}
		return "success", nil
	}, 3, time.Millisecond*10)
	assert.NoError(t, err)
	assert.Equal(t, "success", result)
	assert.Equal(t, 3, attempts)
}

func TestRetryUtilResult_Error(t *testing.T) {
	attempts := 0
	result, err := WithRetriesResult(func() (string, error) {
		attempts++
		return "", fmt.Errorf("fail")
	}, 3, time.Millisecond*10)
	assert.Error(t, err)
	assert.Equal(t, "", result)
	assert.Equal(t, 4, attempts) // initial attempt + 3 retries
}
