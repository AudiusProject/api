package indexer

import (
	"testing"

	"github.com/test-go/testify/assert"
)

func TestParsePurchaseMemo(t *testing.T) {
	// happy case
	expected := parsedPurchaseMemo{
		ContentType:           "track",
		ContentId:             1,
		ValidAfterBlocknumber: 2,
		BuyerUserId:           3,
		AccessType:            "download",
	}
	parsed, err := ParsePurchaseMemo([]byte("track:1:2:3:download"))
	assert.NoError(t, err)
	assert.Equal(t, expected, parsed)

	// errors
	_, err = ParsePurchaseMemo([]byte("not:purchase"))
	assert.EqualError(t, err, "not a purchase memo")
	_, err = ParsePurchaseMemo([]byte("track:foo:2:3:download"))
	assert.EqualError(t, err, "failed to parse contentId: strconv.Atoi: parsing \"foo\": invalid syntax")
	_, err = ParsePurchaseMemo([]byte("track:1:foo:3:download"))
	assert.EqualError(t, err, "failed to parse validAfterBlocknumber: strconv.Atoi: parsing \"foo\": invalid syntax")
	_, err = ParsePurchaseMemo([]byte("track:1:2:foo:download"))
	assert.EqualError(t, err, "failed to parse buyerUserId: strconv.Atoi: parsing \"foo\": invalid syntax")
}

func TestParseLocationMemo(t *testing.T) {
	// happy case
	expected := parsedLocationMemo{
		City:    "Minneapolis",
		Region:  "MN",
		Country: "USA",
	}
	parsed, err := ParseLocationMemo([]byte(`geo:{"city":"Minneapolis","region":"MN","country":"USA"}`))
	assert.NoError(t, err)
	assert.Equal(t, expected, parsed)

	// errors
	_, err = ParseLocationMemo([]byte(`geo:{"city":"Minneapolis","region":"MN","country":"USA}`))
	assert.Error(t, err)
	_, err = ParseLocationMemo([]byte(`{"city":"Minneapolis","region":"MN","country":"USA"}`))
	assert.Error(t, err)
}
