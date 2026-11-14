package api

import "github.com/gofiber/fiber/v2"

func isTrendingReward(id string) bool { return id == "tt" || id == "tut" }

// Keep retired routes explicit for older clients, before any code consumption,
// pool creation, signing or Solana transaction submission.
func programmableRewardsDisabled(c *fiber.Ctx) error {
	return fiber.NewError(fiber.StatusGone, "Programmable rewards are no longer supported")
}
