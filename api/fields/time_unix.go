package fields

import (
	"fmt"
	"time"
)

type TimeUnix time.Time

func (t TimeUnix) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("%d", time.Time(t).Unix())), nil
}

func (t *TimeUnix) Scan(value interface{}) error {
	if v, ok := value.(time.Time); ok {
		*t = TimeUnix(v)
		return nil
	}
	return fmt.Errorf("cannot convert %v to UnixTimeMs", value)
}
