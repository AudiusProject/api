package api

import (
	"bridgerton.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
)

type GetTipsQueryParams struct {
	ReceiverMinFollowers int       `query:"receiver_min_followers" default:"0" validate:"min=0"`
	ReceiverIsVerified   bool      `query:"receiver_is_verified" default:"false"`
	CurrentUserFollows   *string   `query:"current_user_follows" default:"" validate:"omitempty,oneof=sender receiver sender_or_receiver"`
	UniqueBy             *string   `query:"unique_by" default:"" validate:"omitempty,oneof=sender receiver"`
	Limit                int       `query:"limit" default:"10" validate:"min=1,max=100"`
	Offset               int       `query:"offset" default:"0" validate:"min=0"`
	TxSignatures         *[]string `query:"tx_signatures" default:"" validate:"omitempty,min=1,max=100"`
	MinSlot              *int      `query:"min_slot" default:"0" validate:"omitempty,min=0"`
	MaxSlot              *int      `query:"max_slot" default:"0" validate:"omitempty,min=0"`
}

func (app *ApiServer) v1Tips(c *fiber.Ctx) error {
	var params = GetTipsQueryParams{}
	if err := app.ParseAndValidateQueryParams(c, &params); err != nil {
		return err
	}

	userId := app.getUserId(c)

	tips, err := app.queries.FullTips(c.Context(), dbv1.GetTipsParams{
		MyId:                 app.getMyId(c),
		UserId:               userId,
		Limit:                params.Limit,
		Offset:               params.Offset,
		ReceiverMinFollowers: params.ReceiverMinFollowers,
		ReceiverIsVerified:   params.ReceiverIsVerified,
		CurrentUserFollows:   params.CurrentUserFollows,
		UniqueBy:             params.UniqueBy,
		MinSlot:              params.MinSlot,
		MaxSlot:              params.MaxSlot,
		TxSignatures:         params.TxSignatures,
	})
	if err != nil {
		return err
	}

	return v1TipsResponse(c, tips)
}
