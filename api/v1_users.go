package api

import (
	"encoding/json"
	"strconv"
	"time"

	"api.audius.co/api/dbv1"
	"api.audius.co/indexer"
	"api.audius.co/trashid"
	corev1 "github.com/OpenAudio/go-openaudio/pkg/api/core/v1"
	"github.com/ethereum/go-ethereum/common"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

// PlaylistLibraryItem represents an item in the user's playlist library
// PlaylistLibraryItem can be either a folder or a playlist identifier
type PlaylistLibraryItem map[string]interface{}

type PlaylistLibrary struct {
	Contents []PlaylistLibraryItem `json:"contents" validate:"required"`
}

type Events struct {
	Referrer     *trashid.HashId `json:"referrer,omitempty" validate:"omitempty,min=1"`
	IsMobileUser *bool           `json:"is_mobile_user,omitempty"`
}

type CreateUserRequest struct {
	UserId              *trashid.HashId  `json:"user_id,omitempty" validate:"omitempty,min=1"`
	Handle              string           `json:"handle" validate:"required,min=1"`
	Wallet              string           `json:"wallet" validate:"required"`
	Name                *string          `json:"name,omitempty" validate:"omitempty,min=1"`
	Bio                 *string          `json:"bio,omitempty" validate:"omitempty,max=256"`
	Location            *string          `json:"location,omitempty"`
	Website             *string          `json:"website,omitempty" validate:"omitempty,url"`
	Donation            *string          `json:"donation,omitempty"`
	TwitterHandle       *string          `json:"twitter_handle,omitempty"`
	InstagramHandle     *string          `json:"instagram_handle,omitempty"`
	TiktokHandle        *string          `json:"tiktok_handle,omitempty"`
	ProfilePicture      *string          `json:"profile_picture,omitempty"`
	ProfilePictureSizes *string          `json:"profile_picture_sizes,omitempty"`
	CoverPhoto          *string          `json:"cover_photo,omitempty"`
	CoverPhotoSizes     *string          `json:"cover_photo_sizes,omitempty"`
	ProfileType         *string          `json:"profile_type,omitempty" validate:"omitempty,oneof=label"`
	AllowAiAttribution  *bool            `json:"allow_ai_attribution,omitempty"`
	SplUsdcPayoutWallet *string          `json:"spl_usdc_payout_wallet,omitempty"`
	PlaylistLibrary     *PlaylistLibrary `json:"playlist_library,omitempty" validate:"omitempty"`
	Events              *Events          `json:"events,omitempty"`
}

type UpdateUserRequest struct {
	Handle              *string          `json:"handle,omitempty" validate:"omitempty,min=1"`
	Name                *string          `json:"name,omitempty" validate:"omitempty,min=1"`
	Bio                 *string          `json:"bio,omitempty" validate:"omitempty,max=256"`
	Location            *string          `json:"location,omitempty"`
	Website             *string          `json:"website,omitempty" validate:"omitempty,url"`
	Donation            *string          `json:"donation,omitempty"`
	TwitterHandle       *string          `json:"twitter_handle,omitempty"`
	InstagramHandle     *string          `json:"instagram_handle,omitempty"`
	TiktokHandle        *string          `json:"tiktok_handle,omitempty"`
	ProfilePicture      *string          `json:"profile_picture,omitempty"`
	ProfilePictureSizes *string          `json:"profile_picture_sizes,omitempty"`
	CoverPhoto          *string          `json:"cover_photo,omitempty"`
	CoverPhotoSizes     *string          `json:"cover_photo_sizes,omitempty"`
	ProfileType         *string          `json:"profile_type,omitempty" validate:"omitempty,oneof=label"`
	IsDeactivated       *bool            `json:"is_deactivated,omitempty"`
	ArtistPickTrackId   *trashid.HashId  `json:"artist_pick_track_id,omitempty" validate:"omitempty,min=1"`
	AllowAiAttribution  *bool            `json:"allow_ai_attribution,omitempty"`
	PlaylistLibrary     *PlaylistLibrary `json:"playlist_library,omitempty" validate:"omitempty"`
	SplUsdcPayoutWallet *string          `json:"spl_usdc_payout_wallet,omitempty"`
	CoinFlairMint       *string          `json:"coin_flair_mint,omitempty"`
	Events              *Events          `json:"events,omitempty"`
}

// v1Users is a handler that retrieves full user data
func (app *ApiServer) v1Users(c *fiber.Ctx) error {
	myId := app.getMyId(c)
	ids := decodeIdList(c)

	if len(ids) == 0 {
		return fiber.NewError(fiber.StatusBadRequest, "no user ids provided")
	}

	users, err := app.queries.Users(c.Context(), dbv1.GetUsersParams{
		MyID: myId,
		Ids:  ids,
	})
	if err != nil {
		return err
	}

	return v1UsersResponse(c, users)
}

func (app *ApiServer) postV1Users(c *fiber.Ctx) error {
	// Parse and validate request body
	var req CreateUserRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body: " + err.Error(),
		})
	}

	// Validate struct tags
	if err := app.requestValidator.Validate(&req); err != nil {
		return err
	}

	// Determine user ID
	var userID int
	if req.UserId != nil {
		userID = int(*req.UserId)
	} else {
		// Generate unclaimed user ID if not provided
		generatedID, err := app.generateUnclaimedId(c.Context(), "users", "user_id", 3_000_000, 999_999_999)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to generate user ID: " + err.Error(),
			})
		}
		userID = generatedID
	}

	// Convert struct to map for metadata
	metadataBytes, err := json.Marshal(req)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Failed to serialize metadata",
		})
	}

	var metadata map[string]interface{}
	if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Failed to process metadata",
		})
	}

	// Remove nil values from metadata
	for key, value := range metadata {
		if value == nil {
			delete(metadata, key)
		}
	}

	// Add user_id to metadata
	metadata["user_id"] = userID

	signer, err := app.getApiSigner(c)
	if err != nil {
		return err
	}

	nonce := time.Now().UnixNano()

	// Build metadata JSON with cid and data fields
	metadataJSON := map[string]interface{}{
		"cid":  "",
		"data": metadata,
	}
	finalMetadataBytes, _ := json.Marshal(metadataJSON)

	manageEntityTx := &corev1.ManageEntityLegacy{
		Signer:     common.HexToAddress(signer.Address).String(),
		UserId:     int64(userID),
		EntityId:   int64(userID),
		Action:     indexer.Action_Create,
		EntityType: indexer.Entity_User,
		Nonce:      strconv.FormatInt(nonce, 10),
		Metadata:   string(finalMetadataBytes),
	}

	response, err := app.sendTransactionWithSigner(manageEntityTx, signer.PrivateKey)
	if err != nil {
		app.logger.Error("Failed to send user create transaction", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create user",
		})
	}

	encodedUserID, _ := trashid.EncodeHashId(userID)
	return c.JSON(fiber.Map{
		"success":          true,
		"transaction_hash": response.Msg.GetTransaction().GetHash(),
		"user_id":          encodedUserID,
	})
}

type GetUsersParams struct {
	Limit  int `query:"limit" default:"20" validate:"min=1,max=100"`
	Offset int `query:"offset" default:"0" validate:"min=0"`
}

// a generic responder for all the simple user lists:
// followers, followees, reposters, savers, etc.
func (app *ApiServer) queryUsers(c *fiber.Ctx, sql string, args pgx.NamedArgs) error {
	myId := app.getMyId(c)

	params := GetUsersParams{}
	if err := app.ParseAndValidateQueryParams(c, &params); err != nil {
		return err
	}

	if _, ok := args["limit"]; !ok {
		args["limit"] = params.Limit
	}
	if _, ok := args["offset"]; !ok {
		args["offset"] = params.Offset
	}

	rows, err := app.pool.Query(c.Context(), sql, args)
	if err != nil {
		return err
	}

	userIds, err := pgx.CollectRows(rows, pgx.RowTo[int32])
	if err != nil {
		return err
	}

	users, err := app.queries.Users(c.Context(), dbv1.GetUsersParams{
		MyID: myId,
		Ids:  userIds,
	})
	if err != nil {
		return err
	}

	return v1UsersResponse(c, users)
}

func (app *ApiServer) putV1User(c *fiber.Ctx) error {
	userID := app.getMyId(c)
	targetUserID, err := trashid.DecodeHashId(c.Params("userId"))
	if err != nil {
		return err
	}

	// Parse and validate request body
	var req UpdateUserRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body: " + err.Error(),
		})
	}

	// Validate struct tags
	if err := app.requestValidator.Validate(&req); err != nil {
		return err
	}

	// Convert struct to map for metadata
	metadataBytes, err := json.Marshal(req)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Failed to serialize metadata",
		})
	}

	var metadata map[string]interface{}
	if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Failed to process metadata",
		})
	}

	// Remove nil values from metadata
	for key, value := range metadata {
		if value == nil {
			delete(metadata, key)
		}
	}

	// Ensure at least one field is being updated
	if len(metadata) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "At least one field must be provided for update",
		})
	}

	signer, err := app.getApiSigner(c)
	if err != nil {
		return err
	}

	nonce := time.Now().UnixNano()

	// Build metadata JSON with cid and data fields
	metadataJSON := map[string]interface{}{
		"cid":  "",
		"data": metadata,
	}
	finalMetadataBytes, _ := json.Marshal(metadataJSON)

	manageEntityTx := &corev1.ManageEntityLegacy{
		Signer:     common.HexToAddress(signer.Address).String(),
		UserId:     int64(userID),
		EntityId:   int64(targetUserID),
		Action:     indexer.Action_Update,
		EntityType: indexer.Entity_User,
		Nonce:      strconv.FormatInt(nonce, 10),
		Metadata:   string(finalMetadataBytes),
	}

	response, err := app.sendTransactionWithSigner(manageEntityTx, signer.PrivateKey)
	if err != nil {
		app.logger.Error("Failed to send user update transaction", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to update user",
		})
	}

	return c.JSON(fiber.Map{
		"success":          true,
		"transaction_hash": response.Msg.GetTransaction().GetHash(),
	})
}
