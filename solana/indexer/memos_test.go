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
	parsed, err := parsePurchaseMemo([]byte("track:1:2:3:download"))
	assert.NoError(t, err)
	assert.Equal(t, expected, parsed)

	// errors
	_, err = parsePurchaseMemo([]byte("not:purchase"))
	assert.EqualError(t, err, "not a purchase memo")
	_, err = parsePurchaseMemo([]byte("track:foo:2:3:download"))
	assert.EqualError(t, err, "failed to parse contentId: strconv.Atoi: parsing \"foo\": invalid syntax")
	_, err = parsePurchaseMemo([]byte("track:1:foo:3:download"))
	assert.EqualError(t, err, "failed to parse validAfterBlocknumber: strconv.Atoi: parsing \"foo\": invalid syntax")
	_, err = parsePurchaseMemo([]byte("track:1:2:foo:download"))
	assert.EqualError(t, err, "failed to parse buyerUserId: strconv.Atoi: parsing \"foo\": invalid syntax")
}

func TestParseLocationMemo(t *testing.T) {
	// happy case
	expected := parsedLocationMemo{
		City:    "Minneapolis",
		Region:  "MN",
		Country: "USA",
	}
	parsed, err := parseLocationMemo([]byte(`geo:{"city":"Minneapolis","region":"MN","country":"USA"}`))
	assert.NoError(t, err)
	assert.Equal(t, expected, parsed)

	// errors
	_, err = parseLocationMemo([]byte(`geo:{"city":"Minneapolis","region":"MN","country":"USA}`))
	assert.Error(t, err)
	_, err = parseLocationMemo([]byte(`{"city":"Minneapolis","region":"MN","country":"USA"}`))
	assert.Error(t, err)
}
