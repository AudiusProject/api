package trashid

import (
	"bytes"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// this will match strings like:
// "id": 123
// "some_id": 123
// "specifier": "123"
//
// but will not match:
// "specifier": "123abc"
//
// specifier can contain a number quoted as a string.
// this is because notifications specifier can contain ints (which should be hash encoded)
// or other values like 180b1ca2 or 36d46f28:202529
var re = regexp.MustCompile(`"(?P<key>\w+_id|id|specifier)"\s*:\s*(?P<val>"\d+"|\d+)`)
var skipKeys = [][]byte{
	[]byte(`special_id`),
}

func HashifyJson(jsonBytes []byte) []byte {
	return re.ReplaceAllFunc(jsonBytes, func(match []byte) []byte {
		submatches := re.FindSubmatchIndex(match)
		if len(submatches) < 6 {
			return match
		}

		// Extract key and value from match using named groups
		key := match[submatches[2]:submatches[3]]
		for _, skipKey := range skipKeys {
			if bytes.Equal(key, skipKey) {
				return match
			}
		}

		val := string(match[submatches[4]:submatches[5]])
		val = strings.Trim(val, `"`)
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
