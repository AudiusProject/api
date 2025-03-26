package api

import (
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHome(t *testing.T) {
	// Setup
	as := NewApiServer()
	req := httptest.NewRequest("GET", "/hello/asdf", nil)
	rec := httptest.NewRecorder()
	c := as.NewContext(req, rec)

	c.SetPath("/hello/:name")
	c.SetParamNames("name")
	c.SetParamValues("asdf")

	err := as.SayHello(c)
	assert.NoError(t, err)
	assert.Equal(t, 200, rec.Code)
	assert.Equal(t, "hello asdf", rec.Body.String())
}
