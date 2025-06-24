package esindexer

import (
	"context"
	"fmt"
	"log"
	"os"
	"slices"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgxlisten"
	"github.com/tidwall/gjson"

	_ "github.com/joho/godotenv/autoload"
)

type pendingChanges struct {
	sync.Mutex
	idMap map[string][]int64
}

func (pending *pendingChanges) enqueue(collection string, id int64) {
	pending.Lock()
	defer pending.Unlock()

	if _, ok := pending.idMap[collection]; !ok {
		pending.idMap[collection] = []int64{}
	}

	if !slices.Contains(pending.idMap[collection], id) {
		pending.idMap[collection] = append(pending.idMap[collection], id)
	}
}

func (pending *pendingChanges) take() map[string][]int64 {
	pending.Lock()
	defer pending.Unlock()

	idMap := pending.idMap
	pending.idMap = map[string][]int64{}
	return idMap
}

func (indexer *EsIndexer) listen(ctx context.Context) error {

	listener := &pgxlisten.Listener{
		Connect: func(ctx context.Context) (*pgx.Conn, error) {
			// Provide a pgx connection for listening
			return pgx.Connect(ctx, os.Getenv("discoveryDbUrl"))
		},
		LogError: func(ctx context.Context, err error) {
			log.Println("Listener error:", err)
		},
		ReconnectDelay: 10 * time.Second,
	}

	pending := &pendingChanges{
		idMap: map[string][]int64{},
	}

	// entity changes get batched up and processed every N seconds
	userHandler := pgxlisten.HandlerFunc(func(ctx context.Context, notification *pgconn.Notification, conn *pgx.Conn) error {
		id := gjson.Get(notification.Payload, "user_id").Int()
		pending.enqueue("users", id)
		return nil
	})

	trackHandler := pgxlisten.HandlerFunc(func(ctx context.Context, notification *pgconn.Notification, conn *pgx.Conn) error {
		id := gjson.Get(notification.Payload, "track_id").Int()
		pending.enqueue("tracks", id)
		return nil
	})

	playlistHandler := pgxlisten.HandlerFunc(func(ctx context.Context, notification *pgconn.Notification, conn *pgx.Conn) error {
		id := gjson.Get(notification.Payload, "playlist_id").Int()
		pending.enqueue("playlists", id)
		return nil
	})

	listener.Handle("users", userHandler)
	listener.Handle("aggregate_user", userHandler)
	listener.Handle("tracks", trackHandler)
	listener.Handle("aggregate_track", trackHandler)
	listener.Handle("playlists", playlistHandler)
	listener.Handle("playlist_track", playlistHandler)

	// "socials" changes do update via a painless script
	listener.Handle("follows", pgxlisten.HandlerFunc(func(ctx context.Context, notification *pgconn.Notification, conn *pgx.Conn) error {
		followerId := gjson.Get(notification.Payload, "follower_user_id").Int()
		followeeId := gjson.Get(notification.Payload, "followee_user_id").Int()
		isDelete := gjson.Get(notification.Payload, "is_delete").Bool()

		indexer.scriptedUpdateSocial(followerId, "following_user_ids", followeeId, isDelete)
		return nil
	}))

	listener.Handle("reposts", pgxlisten.HandlerFunc(func(ctx context.Context, notification *pgconn.Notification, conn *pgx.Conn) error {
		userId := gjson.Get(notification.Payload, "user_id").Int()
		isTrack := gjson.Get(notification.Payload, "repost_type").String() == "track"
		id := gjson.Get(notification.Payload, "repost_item_id").Int()
		isDelete := gjson.Get(notification.Payload, "is_delete").Bool()

		field := "reposted_playlist_ids"
		if isTrack {
			field = "reposted_track_ids"
		}

		indexer.scriptedUpdateSocial(userId, field, id, isDelete)
		return nil
	}))

	listener.Handle("saves", pgxlisten.HandlerFunc(func(ctx context.Context, notification *pgconn.Notification, conn *pgx.Conn) error {
		userId := gjson.Get(notification.Payload, "user_id").Int()
		isTrack := gjson.Get(notification.Payload, "save_type").String() == "track"
		id := gjson.Get(notification.Payload, "save_item_id").Int()
		isDelete := gjson.Get(notification.Payload, "is_delete").Bool()

		field := "saved_playlist_ids"
		if isTrack {
			field = "saved_track_ids"
		}

		indexer.scriptedUpdateSocial(userId, field, id, isDelete)
		return nil
	}))

	// this fires all the time...
	// but we're not even indexing play_count atm...
	// todo: index play count
	// also... we might not need to update this so often...
	// listener.Handle("aggregate_plays", logNotify)

	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			idMap := pending.take()
			for collection, ids := range idMap {
				if err := indexer.indexIds(collection, ids...); err != nil {
					log.Printf("Error indexing %s: %v", collection, err)
				}
			}
		}
	}()

	fmt.Println("🚀 Starting listener...")
	return listener.Listen(ctx)

}
