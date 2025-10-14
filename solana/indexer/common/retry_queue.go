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

type retryQueueItem struct {
	ID        string
	Indexer   string
	Update    retryQueueUpdate
	Error     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type retryQueueUpdate struct {
	*pb.SubscribeUpdate
}

var (
	_ json.Marshaler   = (*retryQueueUpdate)(nil)
	_ json.Unmarshaler = (*retryQueueUpdate)(nil)
)

func (r retryQueueUpdate) MarshalJSON() ([]byte, error) {
	if r.SubscribeUpdate == nil {
		return []byte("{}"), nil
	}
	res, err := protojson.Marshal(r.SubscribeUpdate)
	fmt.Printf("Marshaled JSON: %s, error: %v\n", res, err)
	return res, err
}

func (r *retryQueueUpdate) UnmarshalJSON(data []byte) error {
	fmt.Printf("Unmarshaling JSON: %s\n", data)
	if r.SubscribeUpdate == nil {
		r.SubscribeUpdate = &pb.SubscribeUpdate{}
	}
	return protojson.Unmarshal(data, r.SubscribeUpdate)
}

func GetRetryQueue(ctx context.Context, db database.DBTX, limit, offset int) ([]retryQueueItem, error) {
	sql := `SELECT id, indexer, update, error, created_at, updated_at
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

	items, err := pgx.CollectRows(rows, pgx.RowToStructByName[retryQueueItem])
	if err != nil {
		return nil, fmt.Errorf("failed to collect retry queue items: %w", err)
	}
	return items, nil
}

func AddToRetryQueue(ctx context.Context, db database.DBTX, indexer string, update *pb.SubscribeUpdate, errorMessage string) error {
	sql := `
		INSERT INTO sol_retry_queue (indexer, update, error)
		VALUES (@indexer, @update, @error)
		ON CONFLICT (id) DO UPDATE SET error = @error, updated_at = NOW()
	;`
	_, err := db.Exec(ctx, sql, pgx.NamedArgs{
		"indexer": indexer,
		"update":  retryQueueUpdate{update},
		"error":   errorMessage,
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
