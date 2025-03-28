package api

import (
	"io"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHome(t *testing.T) {
	app := NewApiServer()
	req := httptest.NewRequest("GET", "/hello/asdf", nil)

	res, err := app.Test(req, -1)
	assert.NoError(t, err)
	assert.Equal(t, 200, res.StatusCode)
	got, _ := io.ReadAll(res.Body)
	assert.Equal(t, "hello asdf", string(got))
}
