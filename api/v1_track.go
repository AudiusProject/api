package api

import (
	"encoding/json"
	"math"
	"strconv"
	"time"

	"api.audius.co/api/dbv1"
	"api.audius.co/indexer"
	"api.audius.co/trashid"
	corev1 "github.com/OpenAudio/go-openaudio/pkg/api/core/v1"
	"github.com/ethereum/go-ethereum/common"
	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

// Nested type definitions for track metadata

type RemixParent struct {
	ParentTrackId trashid.HashId `json:"parent_track_id" validate:"required,min=1"`
}

type RemixOf struct {
	Tracks []RemixParent `json:"tracks" validate:"required,min=1,dive"`
}

type StemOf struct {
	Category      string         `json:"category" validate:"required,oneof=INSTRUMENTAL LEAD_VOCALS MELODIC_LEAD PAD SNARE KICK HIHAT PERCUSSION SAMPLE BACKING_VOX BASS OTHER"`
	ParentTrackId trashid.HashId `json:"parent_track_id" validate:"required,min=1"`
}

type FieldVisibility struct {
	Mood      *bool `json:"mood,omitempty"`
	Tags      *bool `json:"tags,omitempty"`
	Genre     *bool `json:"genre,omitempty"`
	Share     *bool `json:"share,omitempty"`
	PlayCount *bool `json:"play_count,omitempty"`
	Remixes   *bool `json:"remixes,omitempty"`
}

type NFTCollection struct {
	Chain        string  `json:"chain" validate:"required,oneof=eth sol"`
	Standard     *string `json:"standard,omitempty" validate:"omitempty,oneof=ERC721 ERC1155"`
	Address      string  `json:"address" validate:"required"`
	Name         string  `json:"name" validate:"required"`
	Slug         *string `json:"slug,omitempty"`
	ImageUrl     *string `json:"image_url,omitempty"`
	ExternalLink *string `json:"external_link,omitempty"`
}

type CollectibleGatedConditions struct {
	NftCollection *NFTCollection `json:"nft_collection,omitempty" validate:"omitempty"`
}

type FollowGatedConditions struct {
	FollowUserId int `json:"follow_user_id" validate:"required,min=1"`
}

type TipGatedConditions struct {
	TipUserId int `json:"tip_user_id" validate:"required,min=1"`
}

type TokenGate struct {
	TokenMint   string `json:"token_mint" validate:"required"`
	TokenAmount int    `json:"token_amount" validate:"required,min=1"`
}

type TokenGatedConditions struct {
	TokenGate TokenGate `json:"token_gate" validate:"required"`
}

type PurchaseSplit struct {
	UserId     int     `json:"user_id" validate:"required,min=1"`
	Percentage float64 `json:"percentage" validate:"required,min=0,max=100"`
}

type USDCPurchase struct {
	Price  float64         `json:"price" validate:"required,min=0"`
	Splits []PurchaseSplit `json:"splits" validate:"required,dive"`
}

type USDCPurchaseConditions struct {
	UsdcPurchase USDCPurchase `json:"usdc_purchase" validate:"required"`
}

// AccessConditions can be one of: CollectibleGatedConditions, FollowGatedConditions,
// TipGatedConditions, TokenGatedConditions, or USDCPurchaseConditions.
// In Go, we use a flexible approach where the JSON contains the discriminating field.
type AccessConditions struct {
	// Exactly one of these should be populated
	NftCollection *NFTCollection `json:"nft_collection,omitempty" validate:"omitempty"`
	FollowUserId  *int           `json:"follow_user_id,omitempty" validate:"omitempty,min=1"`
	TipUserId     *int           `json:"tip_user_id,omitempty" validate:"omitempty,min=1"`
	TokenGate     *TokenGate     `json:"token_gate,omitempty" validate:"omitempty"`
	UsdcPurchase  *USDCPurchase  `json:"usdc_purchase,omitempty" validate:"omitempty"`
}

type DDEXResourceContributor struct {
	Name           string   `json:"name" validate:"required,min=1"`
	Roles          []string `json:"roles" validate:"required,min=1,dive,min=1"`
	SequenceNumber *int     `json:"sequence_number,omitempty" validate:"omitempty,min=0"`
}

type DDEXCopyright struct {
	Year string `json:"year" validate:"required,len=4"`
	Text string `json:"text" validate:"required,min=1"`
}

type DDEXRightsController struct {
	Name               string   `json:"name" validate:"required,min=1"`
	Roles              []string `json:"roles" validate:"required,min=1,dive,min=1"`
	RightsShareUnknown *string  `json:"rights_share_unknown,omitempty"`
}

type CreateTrackRequest struct {
	TrackId                      *trashid.HashId            `json:"track_id,omitempty" validate:"omitempty,min=1"`
	Title                        string                     `json:"title" validate:"required,min=1"`
	Genre                        string                     `json:"genre" validate:"required,oneof='Electronic' 'Rock' 'Metal' 'Alternative' 'Hip-Hop/Rap' 'Experimental' 'Punk' 'Folk' 'Pop' 'Ambient' 'Soundtrack' 'World' 'Jazz' 'Acoustic' 'Funk' 'R&B/Soul' 'Devotional' 'Classical' 'Reggae' 'Podcasts' 'Country' 'Spoken Word' 'Comedy' 'Blues' 'Kids' 'Audiobooks' 'Latin' 'Lo-Fi' 'Hyperpop' 'Dancehall' 'Techno' 'Trap' 'House' 'Tech House' 'Deep House' 'Disco' 'Electro' 'Jungle' 'Progressive House' 'Hardstyle' 'Glitch Hop' 'Trance' 'Future Bass' 'Future House' 'Tropical House' 'Downtempo' 'Drum & Bass' 'Dubstep' 'Jersey Club' 'Vaporwave' 'Moombahton'"`
	Description                  *string                    `json:"description,omitempty" validate:"omitempty,max=1000"`
	Mood                         *string                    `json:"mood,omitempty" validate:"omitempty,oneof='Peaceful' 'Romantic' 'Sentimental' 'Tender' 'Easygoing' 'Yearning' 'Sophisticated' 'Sensual' 'Cool' 'Gritty' 'Melancholy' 'Serious' 'Brooding' 'Fiery' 'Defiant' 'Aggressive' 'Rowdy' 'Excited' 'Energizing' 'Empowering' 'Stirring' 'Upbeat' 'Other'"`
	Tags                         *string                    `json:"tags,omitempty"`
	License                      *string                    `json:"license,omitempty"`
	Isrc                         *string                    `json:"isrc,omitempty"`
	Iswc                         *string                    `json:"iswc,omitempty"`
	ReleaseDate                  *string                    `json:"release_date,omitempty"`
	TrackCid                     string                     `json:"track_cid" validate:"required"`
	CoverArtCid                  *string                    `json:"cover_art_cid,omitempty"`
	PreviewCid                   *string                    `json:"preview_cid,omitempty"`
	PreviewStartSeconds          *float64                   `json:"preview_start_seconds,omitempty" validate:"omitempty,min=0"`
	Duration                     *float64                   `json:"duration,omitempty" validate:"omitempty,min=0"`
	IsDownloadable               *bool                      `json:"is_downloadable,omitempty"`
	IsUnlisted                   *bool                      `json:"is_unlisted,omitempty"`
	FieldVisibility              *FieldVisibility           `json:"field_visibility,omitempty" validate:"omitempty"`
	RemixOf                      *RemixOf                   `json:"remix_of,omitempty" validate:"omitempty"`
	StemOf                       *StemOf                    `json:"stem_of,omitempty" validate:"omitempty"`
	DownloadConditions           *AccessConditions          `json:"download_conditions,omitempty" validate:"omitempty"`
	StreamConditions             *AccessConditions          `json:"stream_conditions,omitempty" validate:"omitempty"`
	IsStreamGated                *bool                      `json:"is_stream_gated,omitempty"`
	IsDownloadGated              *bool                      `json:"is_download_gated,omitempty"`
	AiAttributionUserId          *trashid.HashId            `json:"ai_attribution_user_id,omitempty" validate:"omitempty,min=0"`
	AllowedApiKeys               *[]string                  `json:"allowed_api_keys,omitempty"`
	PlacementHosts               *string                    `json:"placement_hosts,omitempty"`
	DdexApp                      *string                    `json:"ddex_app,omitempty"`
	DdexReleaseIds               *map[string]string         `json:"ddex_release_ids,omitempty"`
	Artists                      *[]DDEXResourceContributor `json:"artists,omitempty" validate:"omitempty,dive"`
	ResourceContributors         *[]DDEXResourceContributor `json:"resource_contributors,omitempty" validate:"omitempty,dive"`
	IndirectResourceContributors *[]DDEXResourceContributor `json:"indirect_resource_contributors,omitempty" validate:"omitempty,dive"`
	CopyrightLine                *DDEXCopyright             `json:"copyright_line,omitempty" validate:"omitempty"`
	ProducerCopyrightLine        *DDEXCopyright             `json:"producer_copyright_line,omitempty" validate:"omitempty"`
	ParentalWarningType          *string                    `json:"parental_warning_type,omitempty"`
	OrigFileCid                  *string                    `json:"orig_file_cid,omitempty"`
	OrigFilename                 *string                    `json:"orig_filename,omitempty"`
	IsOriginalAvailable          *bool                      `json:"is_original_available,omitempty"`
	AudioUploadId                *string                    `json:"audio_upload_id,omitempty"`
}

type UpdateTrackRequest struct {
	Title                        *string                    `json:"title,omitempty" validate:"omitempty,min=1"`
	Description                  *string                    `json:"description,omitempty" validate:"omitempty,max=1000"`
	Genre                        *string                    `json:"genre,omitempty" validate:"omitempty,oneof='Electronic' 'Rock' 'Metal' 'Alternative' 'Hip-Hop/Rap' 'Experimental' 'Punk' 'Folk' 'Pop' 'Ambient' 'Soundtrack' 'World' 'Jazz' 'Acoustic' 'Funk' 'R&B/Soul' 'Devotional' 'Classical' 'Reggae' 'Podcasts' 'Country' 'Spoken Word' 'Comedy' 'Blues' 'Kids' 'Audiobooks' 'Latin' 'Lo-Fi' 'Hyperpop' 'Dancehall' 'Techno' 'Trap' 'House' 'Tech House' 'Deep House' 'Disco' 'Electro' 'Jungle' 'Progressive House' 'Hardstyle' 'Glitch Hop' 'Trance' 'Future Bass' 'Future House' 'Tropical House' 'Downtempo' 'Drum & Bass' 'Dubstep' 'Jersey Club' 'Vaporwave' 'Moombahton'"`
	Mood                         *string                    `json:"mood,omitempty" validate:"omitempty,oneof='Peaceful' 'Romantic' 'Sentimental' 'Tender' 'Easygoing' 'Yearning' 'Sophisticated' 'Sensual' 'Cool' 'Gritty' 'Melancholy' 'Serious' 'Brooding' 'Fiery' 'Defiant' 'Aggressive' 'Rowdy' 'Excited' 'Energizing' 'Empowering' 'Stirring' 'Upbeat' 'Other'"`
	Tags                         *string                    `json:"tags,omitempty"`
	License                      *string                    `json:"license,omitempty"`
	Isrc                         *string                    `json:"isrc,omitempty"`
	Iswc                         *string                    `json:"iswc,omitempty"`
	ReleaseDate                  *string                    `json:"release_date,omitempty"`
	Artwork                      *map[string]interface{}    `json:"artwork,omitempty"`
	TrackCid                     *string                    `json:"track_cid,omitempty"`
	CoverArtCid                  *string                    `json:"cover_art_cid,omitempty"`
	PreviewCid                   *string                    `json:"preview_cid,omitempty"`
	PreviewStartSeconds          *float64                   `json:"preview_start_seconds,omitempty" validate:"omitempty,min=0"`
	IsDownloadable               *bool                      `json:"is_downloadable,omitempty"`
	IsUnlisted                   *bool                      `json:"is_unlisted,omitempty"`
	FieldVisibility              *FieldVisibility           `json:"field_visibility,omitempty" validate:"omitempty"`
	RemixOf                      *RemixOf                   `json:"remix_of,omitempty" validate:"omitempty"`
	StemOf                       *StemOf                    `json:"stem_of,omitempty" validate:"omitempty"`
	DownloadConditions           *AccessConditions          `json:"download_conditions,omitempty" validate:"omitempty"`
	StreamConditions             *AccessConditions          `json:"stream_conditions,omitempty" validate:"omitempty"`
	IsStreamGated                *bool                      `json:"is_stream_gated,omitempty"`
	IsDownloadGated              *bool                      `json:"is_download_gated,omitempty"`
	AiAttributionUserId          *trashid.HashId            `json:"ai_attribution_user_id,omitempty" validate:"omitempty,min=0"`
	AllowedApiKeys               *[]string                  `json:"allowed_api_keys,omitempty"`
	PlacementHosts               *string                    `json:"placement_hosts,omitempty"`
	DdexApp                      *string                    `json:"ddex_app,omitempty"`
	DdexReleaseIds               *map[string]string         `json:"ddex_release_ids,omitempty"`
	Artists                      *[]DDEXResourceContributor `json:"artists,omitempty" validate:"omitempty,dive"`
	ResourceContributors         *[]DDEXResourceContributor `json:"resource_contributors,omitempty" validate:"omitempty,dive"`
	IndirectResourceContributors *[]DDEXResourceContributor `json:"indirect_resource_contributors,omitempty" validate:"omitempty,dive"`
	CopyrightLine                *DDEXCopyright             `json:"copyright_line,omitempty" validate:"omitempty"`
	ProducerCopyrightLine        *DDEXCopyright             `json:"producer_copyright_line,omitempty" validate:"omitempty"`
	ParentalWarningType          *string                    `json:"parental_warning_type,omitempty"`
	AudioUploadId                *string                    `json:"audio_upload_id,omitempty"`
}

func (app *ApiServer) v1Track(c *fiber.Ctx) error {
	myId := app.getMyId(c)
	trackId := c.Locals("trackId").(int)

	tracks, err := app.queries.Tracks(c.Context(), dbv1.TracksParams{
		GetTracksParams: dbv1.GetTracksParams{
			MyID: myId,
			Ids:  []int32{int32(trackId)},
		},
	})
	if err != nil {
		return err
	}

	if len(tracks) == 0 {
		return fiber.NewError(fiber.StatusNotFound, "track not found")
	}

	track := tracks[0]

	return v1TrackResponse(c, track)
}

func (app *ApiServer) postV1Tracks(c *fiber.Ctx) error {
	userID := app.getMyId(c)

	// Parse and validate request body
	var req CreateTrackRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body: " + err.Error(),
		})
	}

	// Validate struct tags
	if err := app.requestValidator.Validate(&req); err != nil {
		return err
	}

	// Determine track ID
	var trackID int
	if req.TrackId != nil {
		trackID = int(*req.TrackId)
	} else {
		// Generate unclaimed track ID if not provided
		generatedID, err := app.generateUnclaimedId(c.Context(), "tracks", "track_id", 2_000_000, math.MaxInt32)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to generate track ID: " + err.Error(),
			})
		}
		trackID = generatedID
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

	if metadata["owner_id"] == nil {
		metadata["owner_id"] = userID
	}

	// Remove nil values from metadata
	for key, value := range metadata {
		if value == nil {
			delete(metadata, key)
		}
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
		EntityId:   int64(trackID),
		Action:     indexer.Action_Create,
		EntityType: indexer.Entity_Track,
		Nonce:      strconv.FormatInt(nonce, 10),
		Metadata:   string(finalMetadataBytes),
	}

	response, err := app.sendTransactionWithSigner(manageEntityTx, signer.PrivateKey)
	if err != nil {
		app.logger.Error("Failed to send track create transaction", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create track",
		})
	}

	encodedTrackID, _ := trashid.EncodeHashId(trackID)
	return c.JSON(fiber.Map{
		"transaction_hash": response.Msg.GetTransaction().GetHash(),
		"block_hash":       response.Msg.GetTransaction().GetBlockHash(),
		"block_number":     response.Msg.GetTransaction().GetHeight(),
		"track_id":         encodedTrackID,
	})
}

func (app *ApiServer) putV1Track(c *fiber.Ctx) error {
	userID := app.getMyId(c)
	trackID, err := trashid.DecodeHashId(c.Params("trackId"))
	if err != nil {
		return err
	}

	// Parse and validate request body
	var req UpdateTrackRequest
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
		EntityId:   int64(trackID),
		Action:     indexer.Action_Update,
		EntityType: indexer.Entity_Track,
		Nonce:      strconv.FormatInt(nonce, 10),
		Metadata:   string(finalMetadataBytes),
	}

	response, err := app.sendTransactionWithSigner(manageEntityTx, signer.PrivateKey)
	if err != nil {
		app.logger.Error("Failed to send track update transaction", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to update track",
		})
	}

	return c.JSON(fiber.Map{
		"transaction_hash": response.Msg.GetTransaction().GetHash(),
		"block_hash":       response.Msg.GetTransaction().GetBlockHash(),
		"block_number":     response.Msg.GetTransaction().GetHeight(),
	})
}

func (app *ApiServer) deleteV1Track(c *fiber.Ctx) error {
	userID := app.getMyId(c)
	trackID, err := trashid.DecodeHashId(c.Params("trackId"))
	if err != nil {
		return err
	}

	signer, err := app.getApiSigner(c)
	if err != nil {
		return err
	}

	nonce := time.Now().UnixNano()

	manageEntityTx := &corev1.ManageEntityLegacy{
		Signer:     common.HexToAddress(signer.Address).String(),
		UserId:     int64(userID),
		EntityId:   int64(trackID),
		Action:     indexer.Action_Delete,
		EntityType: indexer.Entity_Track,
		Nonce:      strconv.FormatInt(nonce, 10),
		Metadata:   "",
	}

	response, err := app.sendTransactionWithSigner(manageEntityTx, signer.PrivateKey)
	if err != nil {
		app.logger.Error("Failed to send track delete transaction", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to delete track",
		})
	}

	return c.JSON(fiber.Map{
		"success":          true,
		"transaction_hash": response.Msg.GetTransaction().GetHash(),
	})
}
