package esindexer

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgxlisten"
	"github.com/tidwall/gjson"

	_ "github.com/joho/godotenv/autoload"
)

func (indexer *EsIndexer) listen() error {
	ctx := context.Background()

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

	// todo: these should probably queue IDs for N seconds
	// and then call index
	// especially with the aggregate_user batch update...
	// that touches lots of rows..
	// don't want to be doing those one at a time.
	userHandler := pgxlisten.HandlerFunc(func(ctx context.Context, notification *pgconn.Notification, conn *pgx.Conn) error {
		id := gjson.Get(notification.Payload, "user_id").Int()
		fmt.Println("reindex user:", notification.Channel, id)
		indexer.indexIds("users", id)
		return nil
	})

	trackHandler := pgxlisten.HandlerFunc(func(ctx context.Context, notification *pgconn.Notification, conn *pgx.Conn) error {
		id := gjson.Get(notification.Payload, "track_id").Int()
		fmt.Println("reindex track:", notification.Channel, id)
		indexer.indexIds("tracks", id)
		return nil
	})

	playlistHandler := pgxlisten.HandlerFunc(func(ctx context.Context, notification *pgconn.Notification, conn *pgx.Conn) error {
		id := gjson.Get(notification.Payload, "playlist_id").Int()
		fmt.Println("reindex playlist:", notification.Channel, id)
		indexer.indexIds("playlists", id)
		return nil
	})

	listener.Handle("users", userHandler)
	listener.Handle("aggregate_user", userHandler)
	listener.Handle("tracks", trackHandler)
	listener.Handle("aggregate_track", trackHandler)
	listener.Handle("playlists", playlistHandler)
	listener.Handle("playlist_track", playlistHandler)

	listener.Handle("follows", pgxlisten.HandlerFunc(func(ctx context.Context, notification *pgconn.Notification, conn *pgx.Conn) error {
		followerId := gjson.Get(notification.Payload, "follower_user_id").Int()
		followeeId := gjson.Get(notification.Payload, "followee_user_id").Int()
		fmt.Println("____ reindex socials:", followerId, "add follow", followeeId)
		indexer.indexIds("socials", followerId)
		return nil
	}))

	listener.Handle("reposts", pgxlisten.HandlerFunc(func(ctx context.Context, notification *pgconn.Notification, conn *pgx.Conn) error {
		userId := gjson.Get(notification.Payload, "user_id").Int()
		isTrack := gjson.Get(notification.Payload, "repost_type").String() == "track"
		id := gjson.Get(notification.Payload, "repost_item_id").Int()
		fmt.Println("___ reindex socials:", userId, "add repost", isTrack, id)
		indexer.indexIds("socials", userId)
		return nil
	}))

	listener.Handle("saves", pgxlisten.HandlerFunc(func(ctx context.Context, notification *pgconn.Notification, conn *pgx.Conn) error {
		userId := gjson.Get(notification.Payload, "user_id").Int()
		isTrack := gjson.Get(notification.Payload, "save_type").String() == "track"
		id := gjson.Get(notification.Payload, "save_item_id").Int()
		fmt.Println("____: reindex socials:", userId, "add save", isTrack, id)
		indexer.indexIds("socials", userId)
		return nil
	}))

	// this fires all the time...
	// but we're not even indexing play_count atm...
	// todo: index play count
	// also... we might not need to update this so often...
	// listener.Handle("aggregate_plays", logNotify)

	fmt.Println("🚀 Starting listener...")
	return listener.Listen(ctx)
}
