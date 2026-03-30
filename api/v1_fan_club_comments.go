package api

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"api.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

// fanClubFeedMergedSQL returns rows: kind ('text_post' | 'track'), comment_id, track_id, created_at.
const fanClubFeedMergedSQL = `
WITH 
coin AS (
	SELECT mint, user_id AS owner_id
	FROM artist_coins
	WHERE mint = @mint
),

muted_by_karma AS (
	SELECT muted_user_id
	FROM muted_users
	JOIN aggregate_user ON muted_users.user_id = aggregate_user.user_id
	WHERE muted_users.is_delete = false
	GROUP BY muted_user_id
	HAVING SUM(aggregate_user.follower_count) >= @karmaCommentCountThreshold
),

low_abuse_score AS (
	SELECT user_id
	FROM aggregate_user
	WHERE score < 0
),

deactivated_users AS (
	SELECT user_id
	FROM users
	WHERE is_deactivated = true
),

high_karma_reporters AS (
	SELECT comment_reports.comment_id
	FROM comment_reports
	JOIN aggregate_user ON comment_reports.user_id = aggregate_user.user_id
	WHERE comment_reports.is_delete = false
	GROUP BY comment_reports.comment_id
	HAVING SUM(aggregate_user.follower_count) >= @karmaCommentCountThreshold
),

eligible_comments AS (
	SELECT
		'text_post'::text AS kind,
		comments.comment_id AS comment_id,
		NULL::integer AS track_id,
		comments.created_at
	FROM comments
	LEFT JOIN coin ON comments.entity_id = coin.owner_id
	LEFT JOIN comment_threads USING (comment_id)
	LEFT JOIN comment_reports ON comments.comment_id = comment_reports.comment_id
	LEFT JOIN muted_users ON (
		muted_users.muted_user_id = comments.user_id
		AND (
			muted_users.user_id = @myId
			OR muted_users.user_id = coin.owner_id
			OR muted_users.muted_user_id IN (SELECT muted_user_id FROM muted_by_karma)
		)
		AND @myId != comments.user_id
	)
	LEFT JOIN low_abuse_score ON (
		low_abuse_score.user_id = comments.user_id
		AND @myId != comments.user_id
		AND coin.owner_id != comments.user_id
	)
	LEFT JOIN deactivated_users ON (
		deactivated_users.user_id = comments.user_id
		AND @myId != comments.user_id
		AND coin.owner_id != comments.user_id
	)
	WHERE comments.entity_id = coin.owner_id
		AND NOT EXISTS (
			SELECT 1 FROM comment_threads ct WHERE ct.comment_id = comments.comment_id
		)
		AND comments.entity_type = 'Coin'
		AND comments.is_delete = false
		AND (
			comment_reports.comment_id IS NULL
			OR (
				comment_reports.user_id != COALESCE(@myId, 0)
				AND comment_reports.user_id != coin.owner_id
				AND comments.comment_id NOT IN (SELECT hkr.comment_id FROM high_karma_reporters hkr)
			)
			OR comment_reports.is_delete = true
		)
		AND (
			muted_users.muted_user_id IS NULL
			OR muted_users.is_delete = true
		)
		AND low_abuse_score.user_id IS NULL
		AND deactivated_users.user_id IS NULL
),

eligible_tracks AS (
	SELECT
		'track'::text AS kind,
		NULL::integer AS comment_id,
		tracks.track_id AS track_id,
		tracks.created_at
	FROM tracks
	INNER JOIN coin ON tracks.owner_id = coin.owner_id
	WHERE tracks.is_current = true
		AND tracks.is_delete = false
		AND tracks.is_unlisted = false
		AND tracks.is_stream_gated = true
		AND tracks.stream_conditions->'token_gate'->>'token_mint' = @mint
)

SELECT kind, comment_id, track_id, created_at FROM (
	SELECT * FROM eligible_comments
	UNION ALL
	SELECT * FROM eligible_tracks
) AS merged
`

// viewerCanRevealFanClubText is true for the artist, Audius users holding >= 1 whole token,
// or a verified third-party Solana wallet with enough raw balance for that mint.
func (app *ApiServer) viewerCanRevealFanClubText(
	ctx context.Context,
	artistUserID int32,
	mint string,
	myID int32,
	solanaWallet string,
) (bool, error) {
	if myID != 0 && myID == artistUserID {
		return true, nil
	}

	var audiusOK bool
	err := app.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM sol_user_balances sub
			JOIN artist_coins ac ON ac.mint = sub.mint
			WHERE sub.user_id = $1
			  AND ac.user_id = $2
			  AND ac.mint = $3
			  AND sub.balance >= power(10::numeric, ac.decimals)::bigint
		)
	`, myID, artistUserID, mint).Scan(&audiusOK)
	if err != nil {
		return false, err
	}
	if audiusOK {
		return true, nil
	}

	if solanaWallet == "" {
		return false, nil
	}

	var walletOK bool
	err = app.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(st.balance), 0) >= power(10::numeric, ac.decimals)::bigint
		FROM artist_coins ac
		LEFT JOIN sol_token_account_balances st ON st.owner = $2 AND st.mint = ac.mint
		WHERE ac.mint = $1
		GROUP BY ac.decimals
	`, mint, solanaWallet).Scan(&walletOK)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return walletOK, nil
}

func fanClubRedactCommentMessageMap(m map[string]any) {
	m["message"] = nil
	reps, ok := m["replies"].([]any)
	if !ok || reps == nil {
		return
	}
	for _, r := range reps {
		rm, ok := r.(map[string]any)
		if !ok {
			continue
		}
		fanClubRedactCommentMessageMap(rm)
	}
}

func fanClubCommentForFeedJSON(co dbv1.FullComment, reveal bool) (any, error) {
	if reveal {
		return co, nil
	}
	b, err := json.Marshal(co)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	fanClubRedactCommentMessageMap(m)
	return m, nil
}

func (app *ApiServer) v1FanClubFeed(c *fiber.Ctx) error {
	mint := c.Query("mint")
	if mint == "" {
		return fiber.NewError(fiber.StatusBadRequest, "mint query parameter is required")
	}

	var artistUserID int32
	err := app.pool.QueryRow(c.Context(), `
		SELECT user_id FROM artist_coins WHERE mint = $1 LIMIT 1
	`, mint).Scan(&artistUserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fiber.NewError(fiber.StatusNotFound, "unknown artist coin mint")
		}
		return err
	}

	var params GetCommentsParams
	if err := app.ParseAndValidateQueryParams(c, &params); err != nil {
		return err
	}

	myID := app.getMyId(c)
	solWallet, _ := c.Context().Value(SolanaWalletCtxKey).(string)

	revealText, err := app.viewerCanRevealFanClubText(c.Context(), artistUserID, mint, myID, solWallet)
	if err != nil {
		return err
	}

	args := pgx.NamedArgs{
		"myId":                       myID,
		"mint":                       mint,
		"karmaCommentCountThreshold": karmaCommentCountThreshold,
		"limit":                      params.Limit,
		"offset":                     params.Offset,
	}

	orderBy := `merged.created_at DESC`
	switch params.SortMethod {
	case "timestamp":
		orderBy = `merged.created_at ASC`
	case "top":
		orderBy = `(SELECT COUNT(*) FROM comment_reactions cr WHERE cr.comment_id = merged.comment_id) DESC NULLS LAST, merged.created_at DESC`
	}

	sql := fanClubFeedMergedSQL + ` ORDER BY ` + orderBy + `
LIMIT @limit
OFFSET @offset
`

	type feedRow struct {
		Kind      string
		CommentID *int32
		TrackID   *int32
		CreatedAt time.Time
	}

	rows, err := app.pool.Query(c.Context(), sql, args)
	if err != nil {
		return err
	}
	defer rows.Close()

	var feedRows []feedRow
	for rows.Next() {
		var r feedRow
		if err := rows.Scan(&r.Kind, &r.CommentID, &r.TrackID, &r.CreatedAt); err != nil {
			return err
		}
		feedRows = append(feedRows, r)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	commentIDs := make([]int32, 0)
	trackIDs := make([]int32, 0)
	for _, r := range feedRows {
		switch r.Kind {
		case "text_post":
			if r.CommentID != nil {
				commentIDs = append(commentIDs, *r.CommentID)
			}
		case "track":
			if r.TrackID != nil {
				trackIDs = append(trackIDs, *r.TrackID)
			}
		}
	}

	comments, err := app.queries.FullComments(c.Context(), dbv1.GetCommentsParams{
		Ids:             commentIDs,
		MyID:            myID,
		IncludeUnlisted: true,
	})
	if err != nil {
		return err
	}
	commentByID := make(map[int32]dbv1.FullComment, len(comments))
	for _, co := range comments {
		commentByID[int32(co.Id)] = co
	}

	parallel, err := app.queries.Parallel(c.Context(), dbv1.ParallelParams{
		UserIds:      nil,
		TrackIds:     trackIDs,
		MyID:         myID,
		AuthedWallet: app.tryGetAuthedWallet(c),
	})
	if err != nil {
		return err
	}
	trackMap := parallel.TrackMap
	if trackMap == nil {
		trackMap = map[int32]dbv1.Track{}
	}

	userIDs := []int32{}
	relatedTrackIDs := append([]int32(nil), trackIDs...)
	for _, co := range comments {
		userIDs = append(userIDs, int32(co.UserId))
		if co.EntityType == "Track" {
			relatedTrackIDs = append(relatedTrackIDs, int32(co.EntityId))
		}
		if co.EntityType == "Coin" {
			userIDs = append(userIDs, int32(co.EntityId))
		}
		for _, m := range co.Mentions {
			userIDs = append(userIDs, int32(m.UserId))
		}
	}
	for _, tid := range trackIDs {
		if t, ok := trackMap[tid]; ok {
			userIDs = append(userIDs, int32(t.UserID))
		}
	}

	related, err := app.queries.Parallel(c.Context(), dbv1.ParallelParams{
		UserIds:      userIDs,
		TrackIds:     relatedTrackIDs,
		MyID:         myID,
		AuthedWallet: app.tryGetAuthedWallet(c),
	})
	if err != nil {
		return err
	}

	items := make([]fiber.Map, 0, len(feedRows))
	for _, r := range feedRows {
		switch r.Kind {
		case "text_post":
			if r.CommentID == nil {
				continue
			}
			co, ok := commentByID[*r.CommentID]
			if !ok {
				continue
			}
			commentJSON, err := fanClubCommentForFeedJSON(co, revealText)
			if err != nil {
				return err
			}
			items = append(items, fiber.Map{
				"item_type": "text_post",
				"comment":   commentJSON,
			})
		case "track":
			if r.TrackID == nil {
				continue
			}
			tr, ok := trackMap[*r.TrackID]
			if !ok {
				continue
			}
			items = append(items, fiber.Map{
				"item_type": "track",
				"track":     tr,
			})
		}
	}

	return c.JSON(fiber.Map{
		"data": items,
		"related": fiber.Map{
			"users":  related.UserList(),
			"tracks": related.TrackList(),
		},
	})
}
