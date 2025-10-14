package common

import (
	"fmt"
	"time"
)

func WithRetries(f func() error, maxRetries int, interval time.Duration) error {
	err := f()
	retries := 0
	for err != nil && retries < maxRetries {
		time.Sleep(interval)
		err = f()
		retries++
	}
	if err != nil {
		return fmt.Errorf("retry failed: %w", err)
	}
	return nil
}

func WithRetriesResult[T any](f func() (T, error), maxRetries int, interval time.Duration) (T, error) {
	result, err := f()
	retries := 0
	for err != nil && retries < maxRetries {
		time.Sleep(interval)
		result, err = f()
		retries++
	}
	if err != nil {
		var zero T
		return zero, fmt.Errorf("retry failed: %w", err)
	}
	return result, nil
}
