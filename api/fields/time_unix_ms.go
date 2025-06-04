package fields

import (
	"fmt"
	"time"
)

type TimeUnixMs time.Time

func (t TimeUnixMs) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("%d", time.Time(t).Unix())), nil
}

func (t *TimeUnixMs) Scan(value interface{}) error {
	if v, ok := value.(time.Time); ok {
		*t = TimeUnixMs(v)
		return nil
	}
	return fmt.Errorf("cannot convert %v to UnixTimeMs", value)
}
