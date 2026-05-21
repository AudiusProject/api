package api

import (
	"encoding/json"

	"api.audius.co/trashid"
	"github.com/gofiber/fiber/v2"
)

// postV1ChallengesSignal accepts client-reported challenge signals
// (mobile installs, referrals, one-shot grants). Inserts a row into
// challenge_signals which the IndexChallengesJob processors then consume.
//
// Authn: requireAuthMiddleware + requireWriteScope (existing pattern).
//
// For user-reported signals (`mobile_install`, `referral`), the
// authed user must equal the target — you can only report your own
// install or your own referrer-association.
//
// For admin-issued signals (`one_shot`), this endpoint requires the
// authed wallet to be in a trusted-admin allowlist. For now we accept
// any authenticated request and rely on the catalog row being inactive
// when needed; tightening to an admin allowlist is a follow-up.
func (app *ApiServer) postV1ChallengesSignal(c *fiber.Ctx) error {
	type body struct {
		Type        string          `json:"type"`
		UserID      string          `json:"user_id"`
		Extra       json.RawMessage `json:"extra"`
		ClientNonce string          `json:"client_nonce"`
	}
	var b body
	if err := c.BodyParser(&b); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid body: "+err.Error())
	}

	// Validate type against the enum we'll insert as.
	switch b.Type {
	case "mobile_install", "one_shot", "referral":
	default:
		return fiber.NewError(fiber.StatusBadRequest, "unsupported signal type")
	}

	targetUserID, err := trashid.DecodeHashId(b.UserID)
	if err != nil || targetUserID == 0 {
		return fiber.NewError(fiber.StatusBadRequest, "invalid user_id")
	}

	authedUserID := app.getMyId(c)

	// For user-reported signals, target must equal the authed user — a
	// user can't report someone else's mobile install or claim someone
	// else's referral. one_shot may target arbitrary users (admin).
	if b.Type != "one_shot" && int32(targetUserID) != authedUserID {
		return fiber.NewError(fiber.StatusForbidden, "cannot report a signal for another user")
	}

	// Default extra to empty JSON object so the inserted column is valid jsonb.
	extra := b.Extra
	if len(extra) == 0 {
		extra = json.RawMessage("{}")
	}

	source := "client"
	if b.Type == "one_shot" {
		source = "admin"
	}

	var nonce any
	if b.ClientNonce != "" {
		nonce = b.ClientNonce
	}

	if app.writePool == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "write pool not configured")
	}
	_, err = app.writePool.Exec(c.Context(), `
		INSERT INTO challenge_signals (type, user_id, extra, source, client_nonce)
		VALUES ($1::challenge_signal_type, $2, $3::jsonb, $4, $5)
		ON CONFLICT (type, user_id, client_nonce)
		  WHERE client_nonce IS NOT NULL
		  DO NOTHING
	`, b.Type, targetUserID, string(extra), source, nonce)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to record signal: "+err.Error())
	}

	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{"status": "accepted"})
}
