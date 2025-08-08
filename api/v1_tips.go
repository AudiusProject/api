package api

import (
	"bridgerton.audius.co/api/dbv1"
	"bridgerton.audius.co/trashid"
	"github.com/gofiber/fiber/v2"
)

type GetTipsQueryParams struct {
	ReceiverMinFollowers int              `query:"receiver_min_followers" default:"0" validate:"min=0"`
	ReceiverIsVerified   bool             `query:"receiver_is_verified" default:"false"`
	CurrentUserFollows   *string          `query:"current_user_follows" default:"" validate:"omitempty,oneof=sender receiver sender_or_receiver"`
	UniqueBy             *string          `query:"unique_by" default:"" validate:"omitempty,oneof=sender receiver"`
	Limit                int              `query:"limit" default:"10" validate:"min=1,max=100"`
	Offset               int              `query:"offset" default:"0" validate:"min=0"`
	MinSlot              *int             `query:"min_slot" default:"0" validate:"omitempty,min=0"`
	MaxSlot              *int             `query:"max_slot" default:"0" validate:"omitempty,min=0"`
	ExcludeRecipients    []trashid.HashId `query:"exclude_recipients" default:"" validate:"omitempty,min=1,max=100"`
}

func (app *ApiServer) v1Tips(c *fiber.Ctx) error {
	var params = GetTipsQueryParams{}
	if err := app.ParseAndValidateQueryParams(c, &params); err != nil {
		return err
	}
	txSignatures := queryMulti(c, "tx_signature")

	tips, err := app.queries.FullTips(c.Context(), dbv1.GetTipsParams{
		MyId:                 app.getMyId(c),
		UserId:               app.getMyId(c),
		Limit:                params.Limit,
		Offset:               params.Offset,
		ReceiverMinFollowers: params.ReceiverMinFollowers,
		ReceiverIsVerified:   params.ReceiverIsVerified,
		CurrentUserFollows:   params.CurrentUserFollows,
		UniqueBy:             params.UniqueBy,
		MinSlot:              params.MinSlot,
		MaxSlot:              params.MaxSlot,
		TxSignatures:         txSignatures,
	})
	if err != nil {
		return err
	}

	return v1TipsResponse(c, tips)
}
