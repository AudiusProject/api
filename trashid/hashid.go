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

type TrashId int

func (num TrashId) MarshalJSON() ([]byte, error) {
	hid, err := EncodeHashId(int(num))
	return []byte(`"` + hid + `"`), err
}

func (num *TrashId) UnmarshalJSON(data []byte) error {
	idStr := strings.Trim(string(data), `"`)
	id, err := DecodeHashId(idStr)
	if err != nil {
		return err
	}
	*num = TrashId(id)
	return nil
}
