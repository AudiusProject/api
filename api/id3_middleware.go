package api

import (
	"api.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
)

// id3Middleware reads the optional `?id3=true` query param and stashes the
// resulting bool on the request context so dbv1.TracksKeyed can decide
// whether to embed ID3 tag query params on generated stream/preview URLs.
//
// ID3 tags are opt-in: when omitted, validator nodes can redirect straight to
// presigned blob-storage URLs; when requested, the validator must proxy the
// bytes so it can prepend the ID3 tag.
func (app *ApiServer) id3Middleware(c *fiber.Ctx) error {
	if c.QueryBool("id3") {
		c.Locals(dbv1.IncludeID3TagsCtxKey, true)
	}
	return c.Next()
}
