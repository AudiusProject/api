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
	// todo: specific handlers to re-index records
	logNotify := pgxlisten.HandlerFunc(func(ctx context.Context, notification *pgconn.Notification, conn *pgx.Conn) error {
		fmt.Println(notification.Channel, notification.Payload)
		return nil
	})

	// Register handler for "my_channel"
	listener.Handle("reposts", logNotify)
	listener.Handle("saves", logNotify)
	listener.Handle("listens", logNotify)
	listener.Handle("tracks", logNotify)
	listener.Handle("users", logNotify)
	listener.Handle("playlists", logNotify)
	listener.Handle("follows", logNotify)
	listener.Handle("aggregate_track", logNotify)
	listener.Handle("aggregate_user", logNotify)
	listener.Handle("playlist_track", logNotify)
	listener.Handle("aggregate_plays", logNotify)

	fmt.Println("🚀 Starting listener...")
	if err := listener.Listen(ctx); err != nil {
		log.Fatalf("Listener terminated: %v", err)
	}
}
