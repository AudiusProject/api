package dbv1

import (
	"context"
	"errors"
	"sort"

	"github.com/RoaringBitmap/roaring"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func (q *Queries) hydrateCurrentUserRelationships(ctx context.Context, myID int32, users map[int32]User) error {
	if myID <= 0 || len(users) == 0 {
		return nil
	}

	targetIDs := make([]int32, 0, len(users))
	for id := range users {
		targetIDs = append(targetIDs, id)
	}
	sort.Slice(targetIDs, func(i, j int) bool { return targetIDs[i] < targetIDs[j] })

	myFollowees, err := q.loadFolloweesBitmap(ctx, myID)
	if err != nil {
		return err
	}

	targetFollowers, err := q.loadFollowerBitmaps(ctx, targetIDs)
	if err != nil {
		return err
	}

	subscribed, err := q.loadSubscribedTargetIDs(ctx, myID, targetIDs)
	if err != nil {
		return err
	}

	for _, id := range targetIDs {
		user := users[id]
		followers := targetFollowers[id]
		user.DoesCurrentUserFollow = myFollowees.Contains(uint32(id))
		user.DoesCurrentUserSubscribe = subscribed[id]
		user.DoesFollowCurrentUser = followers.Contains(uint32(myID))
		if id != myID {
			user.CurrentUserFolloweeFollowCount = int64(roaring.And(myFollowees, followers).GetCardinality())
		}
		users[id] = user
	}

	return nil
}

func (q *Queries) loadFolloweesBitmap(ctx context.Context, userID int32) (*roaring.Bitmap, error) {
	var data []byte
	err := q.db.QueryRow(ctx, `
		SELECT followees_bitmap
		FROM user_social_sets
		WHERE user_id = $1
	`, userID).Scan(&data)
	if err == nil {
		bitmap, err := decodeSocialBitmap(data)
		if err == nil {
			return bitmap, nil
		}
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) && !isUndefinedTable(err) {
		return nil, err
	}
	return q.loadFolloweesBitmapFromFollows(ctx, userID)
}

func (q *Queries) loadFollowerBitmaps(ctx context.Context, userIDs []int32) (map[int32]*roaring.Bitmap, error) {
	bitmaps := make(map[int32]*roaring.Bitmap, len(userIDs))
	loaded := make(map[int32]bool, len(userIDs))
	for _, userID := range userIDs {
		bitmaps[userID] = roaring.NewBitmap()
	}

	rows, err := q.db.Query(ctx, `
		SELECT user_id, followers_bitmap
		FROM user_social_sets
		WHERE user_id = ANY($1::int[])
	`, userIDs)
	if err != nil {
		if isUndefinedTable(err) {
			return q.loadFollowerBitmapsFromFollows(ctx, userIDs)
		}
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var userID int32
		var data []byte
		if err := rows.Scan(&userID, &data); err != nil {
			return nil, err
		}
		bitmap, err := decodeSocialBitmap(data)
		if err != nil {
			continue
		}
		bitmaps[userID] = bitmap
		loaded[userID] = true
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	missing := make([]int32, 0)
	for _, userID := range userIDs {
		if !loaded[userID] {
			missing = append(missing, userID)
		}
	}
	if len(missing) == 0 {
		return bitmaps, nil
	}

	rawBitmaps, err := q.loadFollowerBitmapsFromFollows(ctx, missing)
	if err != nil {
		return nil, err
	}
	for userID, bitmap := range rawBitmaps {
		bitmaps[userID] = bitmap
	}

	return bitmaps, nil
}

func (q *Queries) loadFolloweesBitmapFromFollows(ctx context.Context, userID int32) (*roaring.Bitmap, error) {
	rows, err := q.db.Query(ctx, `
		SELECT followee_user_id
		FROM follows
		WHERE follower_user_id = $1
		  AND is_delete = false
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	bitmap := roaring.NewBitmap()
	for rows.Next() {
		var followeeID int32
		if err := rows.Scan(&followeeID); err != nil {
			return nil, err
		}
		bitmap.Add(uint32(followeeID))
	}
	return bitmap, rows.Err()
}

func (q *Queries) loadFollowerBitmapsFromFollows(ctx context.Context, userIDs []int32) (map[int32]*roaring.Bitmap, error) {
	bitmaps := make(map[int32]*roaring.Bitmap, len(userIDs))
	for _, userID := range userIDs {
		bitmaps[userID] = roaring.NewBitmap()
	}

	rows, err := q.db.Query(ctx, `
		SELECT followee_user_id, follower_user_id
		FROM follows
		WHERE followee_user_id = ANY($1::int[])
		  AND is_delete = false
	`, userIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var followeeID int32
		var followerID int32
		if err := rows.Scan(&followeeID, &followerID); err != nil {
			return nil, err
		}
		if bitmap, ok := bitmaps[followeeID]; ok {
			bitmap.Add(uint32(followerID))
		}
	}
	return bitmaps, rows.Err()
}

func (q *Queries) loadSubscribedTargetIDs(ctx context.Context, myID int32, targetIDs []int32) (map[int32]bool, error) {
	rows, err := q.db.Query(ctx, `
		SELECT user_id
		FROM subscriptions
		WHERE subscriber_id = $1
		  AND user_id = ANY($2::int[])
		  AND is_delete = false
		GROUP BY user_id
	`, myID, targetIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	subscribed := make(map[int32]bool)
	for rows.Next() {
		var userID int32
		if err := rows.Scan(&userID); err != nil {
			return nil, err
		}
		subscribed[userID] = true
	}
	return subscribed, rows.Err()
}

func (q *Queries) RebuildUserSocialSet(ctx context.Context, userID int32) error {
	followees, err := q.loadFolloweesBitmapFromFollows(ctx, userID)
	if err != nil {
		return err
	}

	followerSets, err := q.loadFollowerBitmapsFromFollows(ctx, []int32{userID})
	if err != nil {
		return err
	}

	followeesData, err := encodeSocialBitmap(followees)
	if err != nil {
		return err
	}
	followersData, err := encodeSocialBitmap(followerSets[userID])
	if err != nil {
		return err
	}

	_, err = q.db.Exec(ctx, `
		INSERT INTO user_social_sets (user_id, followees_bitmap, followers_bitmap, updated_at)
		VALUES ($1, $2, $3, CURRENT_TIMESTAMP)
		ON CONFLICT (user_id) DO UPDATE SET
			followees_bitmap = EXCLUDED.followees_bitmap,
			followers_bitmap = EXCLUDED.followers_bitmap,
			updated_at = CURRENT_TIMESTAMP
	`, userID, followeesData, followersData)
	return err
}

func decodeSocialBitmap(data []byte) (*roaring.Bitmap, error) {
	bitmap := roaring.NewBitmap()
	if len(data) == 0 {
		return bitmap, nil
	}
	_, err := bitmap.FromBuffer(data)
	return bitmap, err
}

func encodeSocialBitmap(bitmap *roaring.Bitmap) ([]byte, error) {
	if bitmap == nil {
		return roaring.NewBitmap().ToBytes()
	}
	return bitmap.ToBytes()
}

func isUndefinedTable(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UndefinedTable
}
