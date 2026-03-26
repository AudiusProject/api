package api

import (
	"crypto/subtle"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

const notificationCampaignOpenMetricsHeader = "X-Notification-Campaign-Metrics-Secret"

// POST /v1/users/:userId/notifications/campaigns/:campaignId/open
// Records a first-party push open for an internal notification campaign id (e.g. Supabase announcement / engagement send UUID).
func (app *ApiServer) v1NotificationCampaignPushOpen(c *fiber.Ctx) error {
	if app.writePool == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "write database unavailable")
	}

	campaignID, err := uuid.Parse(c.Params("campaignId"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid campaignId (expected UUID)")
	}

	userID := app.getUserId(c)
	if userID <= 0 {
		return fiber.NewError(fiber.StatusBadRequest, "invalid userId")
	}

	ctx := c.Context()
	cmd, err := app.writePool.Exec(ctx, `
INSERT INTO notification_campaign_push_open (campaign_id, user_id, opened_at)
VALUES ($1, $2, now())
ON CONFLICT (campaign_id, user_id) DO NOTHING
`, campaignID, userID)
	if err != nil {
		return err
	}

	firstOpen := cmd.RowsAffected() > 0
	return c.JSON(fiber.Map{
		"data": fiber.Map{
			"first_open": firstOpen,
		},
	})
}

// GET /v1/notifications/campaigns/:campaignId/opens
// Server-to-server: returns distinct opener count for metrics sync (protected by shared secret).
func (app *ApiServer) v1NotificationCampaignPushOpenMetrics(c *fiber.Ctx) error {
	secret := app.config.NotificationCampaignOpenMetricsSecret
	if secret == "" {
		return fiber.NewError(fiber.StatusNotFound, "not found")
	}

	got := c.Get(notificationCampaignOpenMetricsHeader)
	if !constantTimeStringEqual(got, secret) {
		return fiber.NewError(fiber.StatusUnauthorized, "unauthorized")
	}

	campaignID, err := uuid.Parse(c.Params("campaignId"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid campaignId (expected UUID)")
	}

	ctx := c.Context()
	var count int64
	err = app.pool.QueryRow(ctx, `
SELECT COUNT(*)::bigint
FROM notification_campaign_push_open
WHERE campaign_id = $1
`, campaignID).Scan(&count)
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"data": fiber.Map{
			"unique_opens": count,
		},
	})
}

func constantTimeStringEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
