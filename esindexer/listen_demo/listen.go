package main

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

func main() {
	ctx := context.Background()

	listener := &pgxlisten.Listener{
		// Provide a pgx connection for listening
		Connect: func(ctx context.Context) (*pgx.Conn, error) {
			return pgx.Connect(ctx, os.Getenv("discoveryDbUrl"))
		},
		LogError: func(ctx context.Context, err error) {
			log.Println("Listener error:", err)
		},
		// Reconnect after 10s on error
		ReconnectDelay: 10 * time.Second,
	}

	// generic handler to just print NOTIFY payload
	// logNotify := pgxlisten.HandlerFunc(func(ctx context.Context, notification *pgconn.Notification, conn *pgx.Conn) error {
	// 	fmt.Println(notification.Channel, notification.Payload)
	// 	return nil
	// })

	userHandler := pgxlisten.HandlerFunc(func(ctx context.Context, notification *pgconn.Notification, conn *pgx.Conn) error {
		id := gjson.Get(notification.Payload, "user_id").Int()
		fmt.Println("reindex user:", notification.Channel, id)
		// userIndexer.reindexId(id)
		return nil
	})

	trackHandler := pgxlisten.HandlerFunc(func(ctx context.Context, notification *pgconn.Notification, conn *pgx.Conn) error {
		id := gjson.Get(notification.Payload, "track_id").Int()
		fmt.Println("reindex track:", notification.Channel, id)
		return nil
	})

	playlistHandler := pgxlisten.HandlerFunc(func(ctx context.Context, notification *pgconn.Notification, conn *pgx.Conn) error {
		id := gjson.Get(notification.Payload, "playlist_id").Int()
		fmt.Println("reindex track:", notification.Channel, id)
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
		fmt.Println("reindex socials:", followerId, "add follow", followeeId)
		return nil
	}))

	listener.Handle("reposts", pgxlisten.HandlerFunc(func(ctx context.Context, notification *pgconn.Notification, conn *pgx.Conn) error {
		userId := gjson.Get(notification.Payload, "user_id").Int()
		isTrack := gjson.Get(notification.Payload, "repost_type").String() == "track"
		id := gjson.Get(notification.Payload, "repost_item_id").Int()
		fmt.Println("reindex socials:", userId, "add repost", isTrack, id)
		return nil
	}))

	listener.Handle("saves", pgxlisten.HandlerFunc(func(ctx context.Context, notification *pgconn.Notification, conn *pgx.Conn) error {
		userId := gjson.Get(notification.Payload, "user_id").Int()
		isTrack := gjson.Get(notification.Payload, "save_type").String() == "track"
		id := gjson.Get(notification.Payload, "save_item_id").Int()
		fmt.Println("reindex socials:", userId, "add save", isTrack, id)
		return nil
	}))

	// listener.Handle("reposts", logNotify)
	// listener.Handle("saves", logNotify)

	// listener.Handle("listens", logNotify)
	// listener.Handle("aggregate_plays", logNotify)

	fmt.Println("🚀 Starting listener...")
	if err := listener.Listen(ctx); err != nil {
		log.Fatalf("Listener terminated: %v", err)
	}
}
