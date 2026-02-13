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

type GetCommentsParams struct {
	SortMethod string `query:"sort_method" default:"newest" validate:"oneof=top timestamp newest"`
	Limit      int    `query:"limit" default:"10" validate:"min=1,max=100"`
	Offset     int    `query:"offset" default:"0" validate:"min=0"`
}

type CreateCommentRequest struct {
	EntityType string `json:"entityType" validate:"required,oneof=Track"`
	EntityId   string `json:"entityId" validate:"required"`
	Body       string `json:"body" validate:"required"`
	CommentId  *int   `json:"commentId,omitempty"`
	ParentId   *int   `json:"parentId,omitempty"`
}

type UpdateCommentRequest struct {
	TrackId string `json:"trackId"`
	Body    string `json:"body"`
}

type ReactCommentRequest struct {
	TrackId string `json:"trackId"`
	IsLiked bool   `json:"isLiked"`
}

type PinCommentRequest struct {
	TrackId string `json:"trackId"`
	IsPin   bool   `json:"isPin"`
}

func (app *ApiServer) queryFullComments(
	c *fiber.Ctx,
	sql string,
	args pgx.NamedArgs,
	includeUnlisted bool,
) error {
	var params = GetCommentsParams{}
	if err := app.ParseAndValidateQueryParams(c, &params); err != nil {
		return err
	}

	// sort
	switch params.SortMethod {
	case "top":
		sql += ` ORDER BY (SELECT count(*) FROM comment_reactions WHERE comment_id = comments.comment_id) DESC, comments.created_at DESC `
	case "timestamp":
		sql += ` ORDER BY comments.created_at ASC `
	default:
		sql += ` ORDER BY comments.created_at DESC `
	}

	// pagination
	sql += `
	LIMIT @limit
	OFFSET @offset
	`

	args["limit"] = params.Limit
	args["offset"] = params.Offset

	rows, err := app.pool.Query(c.Context(), sql, args)
	if err != nil {
		return err
	}

	commentIds, err := pgx.CollectRows(rows, pgx.RowTo[int32])
	if err != nil {
		return err
	}

	comments, err := app.queries.FullComments(c.Context(), dbv1.GetCommentsParams{
		Ids:             commentIds,
		MyID:            app.getMyId(c),
		IncludeUnlisted: includeUnlisted,
	})
	if err != nil {
		return err
	}

	// related
	userIds := []int32{}
	trackIds := []int32{}
	for _, c := range comments {
		userIds = append(userIds, int32(c.UserId))
		trackIds = append(trackIds, int32(c.EntityId))
		for _, m := range c.Mentions {
			userIds = append(userIds, int32(m.UserId))
		}
	}
	related, err := app.queries.Parallel(c.Context(), dbv1.ParallelParams{
		UserIds:  userIds,
		TrackIds: trackIds,
		MyID:     app.getMyId(c),
	})
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"data": comments,
		"related": fiber.Map{
			"users":  related.UserList(),
			"tracks": related.TrackList(),
		},
	})
}

func (app *ApiServer) postV1Comment(c *fiber.Ctx) error {
	userID := app.getMyId(c)

	var req CreateCommentRequest
	if err := app.ParseAndValidateBody(c, &req); err != nil {
		return err
	}

	signer, err := app.getApiSigner(c)
	if err != nil {
		return err
	}

	nonce := time.Now().UnixNano()
	commentID := 0
	if req.CommentId != nil {
		commentID = *req.CommentId
	} else {
		// Generate random comment ID if not provided
		commentID = int(nonce % 1000000)
	}

	// Build metadata
	metadataMap := map[string]interface{}{
		"entity_type": req.EntityType,
		"entity_id":   req.EntityId,
		"body":        req.Body,
	}
	if req.ParentId != nil {
		metadataMap["parent_id"] = *req.ParentId
	}

	metadataObj := map[string]interface{}{
		"cid":  "",
		"data": metadataMap,
	}
	metadataBytes, _ := json.Marshal(metadataObj)

	manageEntityTx := &corev1.ManageEntityLegacy{
		Signer:     common.HexToAddress(signer.Address).String(),
		UserId:     int64(userID),
		EntityId:   int64(commentID),
		Action:     indexer.Action_Create,
		EntityType: indexer.Entity_Comment,
		Nonce:      strconv.FormatInt(nonce, 10),
		Metadata:   string(metadataBytes),
	}

	response, err := app.sendTransactionWithSigner(manageEntityTx, signer.PrivateKey)
	if err != nil {
		app.logger.Error("Failed to send comment create transaction", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create comment",
		})
	}

	encodedCommentID, _ := trashid.EncodeHashId(commentID)
	return c.JSON(fiber.Map{
		"success":          true,
		"transaction_hash": response.Msg.GetTransaction().GetHash(),
		"comment_id":       encodedCommentID,
	})
}

func (app *ApiServer) putV1Comment(c *fiber.Ctx) error {
	userID := app.getMyId(c)
	commentID, err := trashid.DecodeHashId(c.Params("commentId"))
	if err != nil {
		return err
	}

	var req UpdateCommentRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	signer, err := app.getApiSigner(c)
	if err != nil {
		return err
	}

	nonce := time.Now().UnixNano()

	metadataObj := map[string]interface{}{
		"cid": "",
		"data": map[string]interface{}{
			"entity_id": req.TrackId,
			"body":      req.Body,
		},
	}
	metadataBytes, _ := json.Marshal(metadataObj)

	manageEntityTx := &corev1.ManageEntityLegacy{
		Signer:     common.HexToAddress(signer.Address).String(),
		UserId:     int64(userID),
		EntityId:   int64(commentID),
		Action:     indexer.Action_Update,
		EntityType: indexer.Entity_Comment,
		Nonce:      strconv.FormatInt(nonce, 10),
		Metadata:   string(metadataBytes),
	}

	response, err := app.sendTransactionWithSigner(manageEntityTx, signer.PrivateKey)
	if err != nil {
		app.logger.Error("Failed to send comment update transaction", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to update comment",
		})
	}

	return c.JSON(fiber.Map{
		"success":          true,
		"transaction_hash": response.Msg.GetTransaction().GetHash(),
	})
}

func (app *ApiServer) deleteV1Comment(c *fiber.Ctx) error {
	userID := app.getMyId(c)
	commentID, err := trashid.DecodeHashId(c.Params("commentId"))
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
		EntityId:   int64(commentID),
		Action:     indexer.Action_Delete,
		EntityType: indexer.Entity_Comment,
		Nonce:      strconv.FormatInt(nonce, 10),
		Metadata:   "",
	}

	response, err := app.sendTransactionWithSigner(manageEntityTx, signer.PrivateKey)
	if err != nil {
		app.logger.Error("Failed to send comment delete transaction", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to delete comment",
		})
	}

	return c.JSON(fiber.Map{
		"success":          true,
		"transaction_hash": response.Msg.GetTransaction().GetHash(),
	})
}

func (app *ApiServer) postV1CommentReact(c *fiber.Ctx) error {
	userID := app.getMyId(c)
	commentID, err := trashid.DecodeHashId(c.Params("commentId"))
	if err != nil {
		return err
	}

	var req ReactCommentRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	signer, err := app.getApiSigner(c)
	if err != nil {
		return err
	}

	nonce := time.Now().UnixNano()

	action := indexer.Action_React
	if !req.IsLiked {
		action = indexer.Action_Unreact
	}

	metadataObj := map[string]interface{}{
		"cid": "",
		"data": map[string]interface{}{
			"entity_id":   req.TrackId,
			"entity_type": indexer.Entity_Track,
		},
	}
	metadataBytes, _ := json.Marshal(metadataObj)

	manageEntityTx := &corev1.ManageEntityLegacy{
		Signer:     common.HexToAddress(signer.Address).String(),
		UserId:     int64(userID),
		EntityId:   int64(commentID),
		Action:     action,
		EntityType: indexer.Entity_Comment,
		Nonce:      strconv.FormatInt(nonce, 10),
		Metadata:   string(metadataBytes),
	}

	response, err := app.sendTransactionWithSigner(manageEntityTx, signer.PrivateKey)
	if err != nil {
		app.logger.Error("Failed to send comment react transaction", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to react to comment",
		})
	}

	return c.JSON(fiber.Map{
		"success":          true,
		"transaction_hash": response.Msg.GetTransaction().GetHash(),
	})
}

func (app *ApiServer) deleteV1CommentReact(c *fiber.Ctx) error {
	userID := app.getMyId(c)
	commentID, err := trashid.DecodeHashId(c.Params("commentId"))
	if err != nil {
		return err
	}

	var req ReactCommentRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	signer, err := app.getApiSigner(c)
	if err != nil {
		return err
	}

	nonce := time.Now().UnixNano()

	metadataObj := map[string]interface{}{
		"cid": "",
		"data": map[string]interface{}{
			"entity_id":   req.TrackId,
			"entity_type": indexer.Entity_Track,
		},
	}
	metadataBytes, _ := json.Marshal(metadataObj)

	manageEntityTx := &corev1.ManageEntityLegacy{
		Signer:     common.HexToAddress(signer.Address).String(),
		UserId:     int64(userID),
		EntityId:   int64(commentID),
		Action:     indexer.Action_Unreact,
		EntityType: indexer.Entity_Comment,
		Nonce:      strconv.FormatInt(nonce, 10),
		Metadata:   string(metadataBytes),
	}

	response, err := app.sendTransactionWithSigner(manageEntityTx, signer.PrivateKey)
	if err != nil {
		app.logger.Error("Failed to send comment unreact transaction", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to unreact to comment",
		})
	}

	return c.JSON(fiber.Map{
		"success":          true,
		"transaction_hash": response.Msg.GetTransaction().GetHash(),
	})
}

func (app *ApiServer) postV1CommentPin(c *fiber.Ctx) error {
	userID := app.getMyId(c)
	commentID, err := trashid.DecodeHashId(c.Params("commentId"))
	if err != nil {
		return err
	}

	var req PinCommentRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	signer, err := app.getApiSigner(c)
	if err != nil {
		return err
	}

	nonce := time.Now().UnixNano()

	action := indexer.Action_Pin
	if !req.IsPin {
		action = indexer.Action_Unpin
	}

	metadataObj := map[string]interface{}{
		"cid": "",
		"data": map[string]interface{}{
			"entity_id": req.TrackId,
		},
	}
	metadataBytes, _ := json.Marshal(metadataObj)

	manageEntityTx := &corev1.ManageEntityLegacy{
		Signer:     common.HexToAddress(signer.Address).String(),
		UserId:     int64(userID),
		EntityId:   int64(commentID),
		Action:     action,
		EntityType: indexer.Entity_Comment,
		Nonce:      strconv.FormatInt(nonce, 10),
		Metadata:   string(metadataBytes),
	}

	response, err := app.sendTransactionWithSigner(manageEntityTx, signer.PrivateKey)
	if err != nil {
		app.logger.Error("Failed to send comment pin transaction", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to pin comment",
		})
	}

	return c.JSON(fiber.Map{
		"success":          true,
		"transaction_hash": response.Msg.GetTransaction().GetHash(),
	})
}

func (app *ApiServer) deleteV1CommentPin(c *fiber.Ctx) error {
	userID := app.getMyId(c)
	commentID, err := trashid.DecodeHashId(c.Params("commentId"))
	if err != nil {
		return err
	}

	var req PinCommentRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	signer, err := app.getApiSigner(c)
	if err != nil {
		return err
	}

	nonce := time.Now().UnixNano()

	metadataObj := map[string]interface{}{
		"cid": "",
		"data": map[string]interface{}{
			"entity_id": req.TrackId,
		},
	}
	metadataBytes, _ := json.Marshal(metadataObj)

	manageEntityTx := &corev1.ManageEntityLegacy{
		Signer:     common.HexToAddress(signer.Address).String(),
		UserId:     int64(userID),
		EntityId:   int64(commentID),
		Action:     indexer.Action_Unpin,
		EntityType: indexer.Entity_Comment,
		Nonce:      strconv.FormatInt(nonce, 10),
		Metadata:   string(metadataBytes),
	}

	response, err := app.sendTransactionWithSigner(manageEntityTx, signer.PrivateKey)
	if err != nil {
		app.logger.Error("Failed to send comment unpin transaction", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to unpin comment",
		})
	}

	return c.JSON(fiber.Map{
		"success":          true,
		"transaction_hash": response.Msg.GetTransaction().GetHash(),
	})
}

func (app *ApiServer) postV1CommentReport(c *fiber.Ctx) error {
	userID := app.getMyId(c)
	commentID, err := trashid.DecodeHashId(c.Params("commentId"))
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
		EntityId:   int64(commentID),
		Action:     indexer.Action_Report,
		EntityType: indexer.Entity_Comment,
		Nonce:      strconv.FormatInt(nonce, 10),
		Metadata:   "",
	}

	response, err := app.sendTransactionWithSigner(manageEntityTx, signer.PrivateKey)
	if err != nil {
		app.logger.Error("Failed to send comment report transaction", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to report comment",
		})
	}

	return c.JSON(fiber.Map{
		"success":          true,
		"transaction_hash": response.Msg.GetTransaction().GetHash(),
	})
}
