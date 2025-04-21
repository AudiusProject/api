package api

import (
	"context"
	"os"
	"testing"

	"bridgerton.audius.co/config"
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/peterldowns/pgtestdb"
	"github.com/peterldowns/pgtestdb/migrators/pgmigrator"
	"github.com/stretchr/testify/assert"
)

func emptyTestApp(t *testing.T) *ApiServer {
	t.Helper()
	// t.Parallel()

	var err error

	dir := os.DirFS("../sql")
	pgm, err := pgmigrator.New(dir)
	assert.NoError(t, err)

	dbconf := pgtestdb.Config{
		DriverName: "pgx",
		User:       "postgres",
		Password:   "example",
		Host:       "localhost",
		Port:       "21300",
	}
	c := pgtestdb.Custom(t, dbconf, pgm)

	app := NewApiServer(config.Config{
		DbUrl: c.URL(),
	})

	t.Cleanup(func() {
		app.pool.Close()
	})

	return app
}

func fixturesTestApp(t *testing.T) *ApiServer {

	app := emptyTestApp(t)

	// stupid block fixture
	_, err := app.pool.Exec(context.Background(), `
	INSERT INTO public.blocks (
		blockhash,
		parenthash,
		is_current,
		number
	) VALUES (
		'block1',   -- blockhash
		'block0',   -- parenthash
		true,
		101
	);
	`)
	checkErr(err)

	insertFixtures(app, "aggregate_user", map[string]any{}, "testdata/aggregate_user_fixtures.csv")
	insertFixtures(app, "users", userBaseRow, "testdata/user_fixtures.csv")
	insertFixtures(app, "tracks", trackBaseRow, "testdata/track_fixtures.csv")
	insertFixtures(app, "playlists", playlistBaseRow, "testdata/playlist_fixtures.csv")
	insertFixtures(app, "follows", followBaseRow, "testdata/follow_fixtures.csv")
	insertFixtures(app, "reposts", repostBaseRow, "testdata/repost_fixtures.csv")
	insertFixtures(app, "developer_apps", developerAppBaseRow, "testdata/developer_app_fixtures.csv")
	insertFixtures(app, "track_trending_scores", trackTrendingScoreBaseRow, "testdata/track_trending_scores_fixtures.csv")
	insertFixtures(app, "associated_wallets", connectedWalletsBaseRow, "testdata/connected_wallets_fixtures.csv")
	insertFixtures(app, "aggregate_user_tips", aggregateUserTipsBaseRow, "testdata/aggregate_user_tips_fixtures.csv")

	return app
}

func TestCount1(t *testing.T) {
	app := emptyTestApp(t)

	count := -1
	err := app.pool.QueryRow(t.Context(), `select count(*) from tracks`).Scan(&count)
	assert.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestCount2(t *testing.T) {
	app := emptyTestApp(t)

	count := -1
	err := app.pool.QueryRow(t.Context(), `select count(*) from tracks`).Scan(&count)
	assert.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestCount3(t *testing.T) {
	app := fixturesTestApp(t)

	count := -1
	err := app.pool.QueryRow(t.Context(), `select count(*) from users`).Scan(&count)
	assert.NoError(t, err)
	assert.Greater(t, count, 0)
}
