package common

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"api.audius.co/database"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/encoding/protojson"

	pb "github.com/rpcpool/yellowstone-grpc/examples/golang/proto"
)

type RetryQueueItem struct {
	ID            string
	Indexer       string
	UpdateMessage RetryQueueUpdate
	Error         string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type RetryQueueUpdate struct {
	*pb.SubscribeUpdate
}

var (
	_ json.Marshaler   = (*RetryQueueUpdate)(nil)
	_ json.Unmarshaler = (*RetryQueueUpdate)(nil)
)

func (r RetryQueueUpdate) MarshalJSON() ([]byte, error) {
	if r.SubscribeUpdate == nil {
		return []byte("{}"), nil
	}
	res, err := protojson.Marshal(r.SubscribeUpdate)
	return res, err
}

func (r *RetryQueueUpdate) UnmarshalJSON(data []byte) error {
	if r.SubscribeUpdate == nil {
		r.SubscribeUpdate = &pb.SubscribeUpdate{}
	}
	return protojson.Unmarshal(data, r.SubscribeUpdate)
}

func GetRetryQueue(ctx context.Context, db database.DBTX, limit, offset int) ([]RetryQueueItem, error) {
	sql := `SELECT id, indexer, update_message, error, created_at, updated_at
			FROM sol_retry_queue
			ORDER BY created_at ASC
			LIMIT @limit OFFSET @offset`

	rows, err := db.Query(ctx, sql, pgx.NamedArgs{
		"limit":  limit,
		"offset": offset,
	})

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil // No items found, return empty slice
		}
		return nil, fmt.Errorf("failed to query retry queue: %w", err)
	}

	items, err := pgx.CollectRows(rows, pgx.RowToStructByName[RetryQueueItem])
	if err != nil {
		return nil, fmt.Errorf("failed to collect retry queue items: %w", err)
	}
	return items, nil
}

func AddToRetryQueue(ctx context.Context, db database.DBTX, indexer string, update *pb.SubscribeUpdate, errorMessage string) error {
	sql := `
		INSERT INTO sol_retry_queue (indexer, update_message, error)
		VALUES (@indexer, @update_message, @error)
		ON CONFLICT (id) DO UPDATE SET error = @error, updated_at = NOW()
	;`
	_, err := db.Exec(ctx, sql, pgx.NamedArgs{
		"indexer":        indexer,
		"update_message": RetryQueueUpdate{update},
		"error":          errorMessage,
	})
	if err != nil {
		return fmt.Errorf("failed to insert into retry queue: %w", err)
	}
	return nil
}

func DeleteFromRetryQueue(ctx context.Context, db database.DBTX, id string) error {
	sql := `DELETE FROM sol_retry_queue WHERE id = @id;`
	_, err := db.Exec(ctx, sql, pgx.NamedArgs{
		"id": id,
	})
	if err != nil {
		return fmt.Errorf("failed to delete from retry queue: %w", err)
	}
	return nil
}
