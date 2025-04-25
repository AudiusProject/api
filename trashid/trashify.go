package trashid

import (
	"bytes"
	"fmt"
	"regexp"
	"strconv"
)

var re = regexp.MustCompile(`"(?P<key>\w+_id|id|specifier)"\s*:\s*(?P<val>\d+)`)
var skipKeys = [][]byte{
	[]byte(`special_id`),
}

func Trashify(jsonBytes []byte) []byte {
	return re.ReplaceAllFunc(jsonBytes, func(match []byte) []byte {
		submatches := re.FindSubmatchIndex(match)
		if submatches == nil || len(submatches) < 6 {
			return match
		}

		// Extract key and value from match using named groups
		key := match[submatches[2]:submatches[3]]
		for _, skipKey := range skipKeys {
			if bytes.Equal(key, skipKey) {
				return match
			}
		}

		val := match[submatches[4]:submatches[5]]
		num, err := strconv.Atoi(string(val))
		if err != nil {
			return match
		}

		// Replace with hex string
		hashed, err := EncodeHashId(num)
		if err != nil {
			return match
		}
		return []byte(fmt.Sprintf(`"%s": "%s"`, key, hashed))
	})
}
