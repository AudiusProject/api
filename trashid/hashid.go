package trashid

import (
	"errors"
	"strconv"
	"strings"

	"github.com/speps/go-hashids/v2"
)

var hashIdUtil *hashids.HashID

func init() {
	hd := hashids.NewData()
	hd.Salt = "azowernasdfoia"
	hd.MinLength = 5
	hashIdUtil, _ = hashids.NewWithData(hd)
}

func DecodeHashId(id string) (int, error) {
	if val, err := strconv.Atoi(id); err == nil {
		return val, nil
	}
	ids, err := hashIdUtil.DecodeWithError(id)
	if err != nil {
		return 0, err
	}
	if len(ids) == 0 {
		return 0, errors.New("invalid hashid")
	}
	return ids[0], err
}

func EncodeHashId(id int) (string, error) {
	return hashIdUtil.Encode([]int{id})
}

func StringEncode(id string) string {
	if num, err := strconv.Atoi(id); err == nil {
		if result, err := hashIdUtil.Encode([]int{num}); err == nil {
			return result
		}
	}
	return id
}

func MustEncodeHashID(id int) string {
	enc, err := EncodeHashId(id)
	if err != nil {
		panic(err)
	}
	return enc
}

func MustDecodeHashID(id string) int {
	val, err := DecodeHashId(id)
	if err != nil {
		panic(err)
	}
	return val
}

// IntId accepts a hash ID or raw int on input (JSON unmarshal) but
// always marshals back as a plain integer. Use this for fields that are
// part of chain metadata where the indexer expects numeric IDs.
type IntId int

func (num IntId) MarshalJSON() ([]byte, error) {
	return []byte(strconv.Itoa(int(num))), nil
}

func (num *IntId) UnmarshalJSON(data []byte) error {
	if data[0] == '"' {
		idStr := strings.Trim(string(data), `"`)
		id, err := DecodeHashId(idStr)
		if err != nil {
			return err
		}
		*num = IntId(id)
		return nil
	}
	val, err := strconv.Atoi(string(data))
	*num = IntId(val)
	return err
}

// type alias for int that will do hashid on the way out the door
type HashId int

func (num HashId) MarshalJSON() ([]byte, error) {
	hid, err := EncodeHashId(int(num))
	return []byte(`"` + hid + `"`), err
}

func (num *HashId) UnmarshalJSON(data []byte) error {
	// if we have a string, it should be hashid encoded
	// so we decode to a number
	if data[0] == '"' {
		idStr := strings.Trim(string(data), `"`)
		id, err := DecodeHashId(idStr)
		if err != nil {
			return err
		}
		*num = HashId(id)
		return nil
	}
	// if not a string parse to int
	val, err := strconv.Atoi(string(data))
	*num = HashId(val)
	return err
}
