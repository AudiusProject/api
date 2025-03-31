package trashid

import (
	"errors"

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

func MustEncodeHashID(id int) string {
	enc, err := EncodeHashId(id)
	if err != nil {
		panic(err)
	}
	return enc
}
