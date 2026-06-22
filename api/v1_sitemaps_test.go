package api

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"api.audius.co/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sitemapTestApp(t *testing.T) *ApiServer {
	app := emptyTestApp(t)

	// Users: user 1 has 100 followers (>=10, included), user 2 has 5 (<10, excluded), user 3 is deactivated (excluded)
	database.Seed(app.pool.Replicas[0], database.FixtureMap{
		"users": {
			{"user_id": 1, "handle": "artist1", "handle_lc": "artist1", "is_deactivated": false},
			{"user_id": 2, "handle": "smallartist", "handle_lc": "smallartist", "is_deactivated": false},
			{"user_id": 3, "handle": "gone", "handle_lc": "gone", "is_deactivated": true},
		},
		"aggregate_user": {
			{"user_id": 1, "follower_count": 100},
			{"user_id": 2, "follower_count": 5},
			{"user_id": 3, "follower_count": 200},
		},
		"tracks": {
			{"track_id": 1, "owner_id": 1, "title": "Listed Track", "is_unlisted": "f"},
			{"track_id": 2, "owner_id": 1, "title": "Unlisted Track", "is_unlisted": "t"},
			{"track_id": 3, "owner_id": 2, "title": "Small Artist Track", "is_unlisted": "f"},
			{"track_id": 4, "owner_id": 1, "title": "Deleted Track", "is_unlisted": "f", "is_delete": true},
		},
		"track_routes": {
			{"slug": "listed-track", "title_slug": "listed-track", "collision_id": 0, "owner_id": 1, "track_id": 1},
			{"slug": "unlisted-track", "title_slug": "unlisted-track", "collision_id": 0, "owner_id": 1, "track_id": 2},
			{"slug": "small-artist-track", "title_slug": "small-artist-track", "collision_id": 0, "owner_id": 2, "track_id": 3},
			{"slug": "deleted-track", "title_slug": "deleted-track", "collision_id": 0, "owner_id": 1, "track_id": 4},
		},
		"playlists": {
			{"playlist_id": 1, "playlist_owner_id": 1, "playlist_name": "My Playlist", "is_album": false, "is_private": false, "playlist_contents": "{}"},
			{"playlist_id": 2, "playlist_owner_id": 1, "playlist_name": "My Album", "is_album": true, "is_private": false, "playlist_contents": "{}"},
			{"playlist_id": 3, "playlist_owner_id": 1, "playlist_name": "Private PL", "is_album": false, "is_private": true, "playlist_contents": "{}"},
			{"playlist_id": 4, "playlist_owner_id": 2, "playlist_name": "Small PL", "is_album": false, "is_private": false, "playlist_contents": "{}"},
		},
		"playlist_routes": {
			{"slug": "my-playlist", "title_slug": "my-playlist", "collision_id": 0, "owner_id": 1, "playlist_id": 1},
			{"slug": "my-album", "title_slug": "my-album", "collision_id": 0, "owner_id": 1, "playlist_id": 2},
			{"slug": "private-pl", "title_slug": "private-pl", "collision_id": 0, "owner_id": 1, "playlist_id": 3},
			{"slug": "small-pl", "title_slug": "small-pl", "collision_id": 0, "owner_id": 2, "playlist_id": 4},
		},
	})

	return app
}

func clearCachedSitemapCount(entityType string) {
	sitemapCountMu.Lock()
	defer sitemapCountMu.Unlock()
	delete(sitemapCountCache, entityType)
}

func TestSitemapDefault(t *testing.T) {
	app := sitemapTestApp(t)
	status, body := testGet(t, app, "/sitemaps/default.xml")
	require.Equal(t, 200, status)

	xml := string(body)
	assert.Contains(t, xml, "<sitemapindex")
	assert.Contains(t, xml, "/sitemaps/defaults.xml")
	assert.Contains(t, xml, "/sitemaps/track/index.xml")
	assert.Contains(t, xml, "/sitemaps/playlist/index.xml")
	assert.Contains(t, xml, "/sitemaps/user/index.xml")
}

func TestSitemapDefaults(t *testing.T) {
	app := sitemapTestApp(t)
	status, body := testGet(t, app, "/sitemaps/defaults.xml")
	require.Equal(t, 200, status)

	xml := string(body)
	assert.Contains(t, xml, "<urlset")
	assert.Contains(t, xml, "legal/privacy-policy")
	assert.Contains(t, xml, "trending")
	assert.Contains(t, xml, "signup")
}

func TestSitemapTrackIndex(t *testing.T) {
	app := sitemapTestApp(t)

	clearCachedSitemapCount("track")

	status, body := testGet(t, app, "/sitemaps/track/index.xml")
	require.Equal(t, 200, status)

	xml := string(body)
	assert.Contains(t, xml, "<sitemapindex")
	assert.Contains(t, xml, "/sitemaps/track/1.xml")
}

func TestSitemapTrackIndexCache(t *testing.T) {
	app := sitemapTestApp(t)

	cached := []byte(`<?xml version="1.0" encoding="UTF-8"?><sitemapindex><sitemap><loc>cached-track-index</loc></sitemap></sitemapindex>`)
	app.setCachedSitemapXML("track:index.xml", cached)

	status, body := testGet(t, app, "/sitemaps/track/index.xml")
	require.Equal(t, 200, status)
	assert.Contains(t, string(body), "cached-track-index")
}

func TestSitemapTrackIndexStaleCacheRefreshesInBackground(t *testing.T) {
	app := sitemapTestApp(t)
	clearCachedSitemapCount("track")

	stale := []byte(`<?xml version="1.0" encoding="UTF-8"?><sitemapindex><sitemap><loc>stale-track-index</loc></sitemap></sitemapindex>`)
	require.True(t, app.sitemapXMLCache.Set("track:index.xml", sitemapXMLCacheEntry{
		data:      stale,
		expiresAt: time.Now().Add(-time.Minute),
	}))

	status, body := testGet(t, app, "/sitemaps/track/index.xml")
	require.Equal(t, 200, status)
	assert.Contains(t, string(body), "stale-track-index")

	require.Eventually(t, func() bool {
		entry, ok := app.sitemapXMLCache.Get("track:index.xml")
		return ok &&
			!entry.refreshing &&
			time.Now().Before(entry.expiresAt) &&
			strings.Contains(string(entry.data), "/sitemaps/track/1.xml") &&
			!strings.Contains(string(entry.data), "stale-track-index")
	}, time.Second, 10*time.Millisecond)
}

func TestSitemapTrackPage(t *testing.T) {
	app := sitemapTestApp(t)
	status, body := testGet(t, app, "/sitemaps/track/1.xml")
	require.Equal(t, 200, status)

	xml := string(body)
	// Should include listed track from user with >=10 followers
	assert.Contains(t, xml, "artist1/listed-track")
	// Should NOT include unlisted track
	assert.NotContains(t, xml, "unlisted-track")
	// Should NOT include track from user with <10 followers
	assert.NotContains(t, xml, "small-artist-track")
	// Should NOT include deleted track
	assert.NotContains(t, xml, "deleted-track")
	// Slashes should not be encoded
	assert.NotContains(t, xml, "%2F")
}

func TestSitemapTrackPageCache(t *testing.T) {
	app := sitemapTestApp(t)

	cached := []byte(`<?xml version="1.0" encoding="UTF-8"?><urlset><url><loc>cached-track-page</loc></url></urlset>`)
	app.setCachedSitemapXML("track:1.xml", cached)

	status, body := testGet(t, app, "/sitemaps/track/1.xml")
	require.Equal(t, 200, status)
	assert.Contains(t, string(body), "cached-track-page")
}

func TestSitemapTrackPageStaleCacheRefreshesInBackground(t *testing.T) {
	app := sitemapTestApp(t)

	stale := []byte(`<?xml version="1.0" encoding="UTF-8"?><urlset><url><loc>stale-track-page</loc></url></urlset>`)
	require.True(t, app.sitemapXMLCache.Set("track:1.xml", sitemapXMLCacheEntry{
		data:      stale,
		expiresAt: time.Now().Add(-time.Minute),
	}))

	status, body := testGet(t, app, "/sitemaps/track/1.xml")
	require.Equal(t, 200, status)
	assert.Contains(t, string(body), "stale-track-page")

	require.Eventually(t, func() bool {
		entry, ok := app.sitemapXMLCache.Get("track:1.xml")
		return ok &&
			!entry.refreshing &&
			time.Now().Before(entry.expiresAt) &&
			strings.Contains(string(entry.data), "artist1/listed-track") &&
			!strings.Contains(string(entry.data), "stale-track-page")
	}, time.Second, 10*time.Millisecond)
}

func TestSitemapPlaylistPage(t *testing.T) {
	app := sitemapTestApp(t)
	status, body := testGet(t, app, "/sitemaps/playlist/1.xml")
	require.Equal(t, 200, status)

	xml := string(body)
	// Public playlist from user with >=10 followers
	assert.Contains(t, xml, "artist1/playlist/my-playlist")
	// Album uses "album" prefix
	assert.Contains(t, xml, "artist1/album/my-album")
	// Private playlist excluded
	assert.NotContains(t, xml, "private-pl")
	// Playlist from user with <10 followers excluded
	assert.NotContains(t, xml, "small-pl")
}

func TestSitemapUserPage(t *testing.T) {
	app := sitemapTestApp(t)
	status, body := testGet(t, app, "/sitemaps/user/1.xml")
	require.Equal(t, 200, status)

	xml := string(body)
	// User with >=10 followers included
	assert.Contains(t, xml, "artist1")
	// User with <10 followers excluded
	assert.NotContains(t, xml, "smallartist")
	// Deactivated user excluded
	assert.NotContains(t, xml, "gone")
}

func TestSitemapInvalidType(t *testing.T) {
	app := sitemapTestApp(t)
	status, _ := testGet(t, app, "/sitemaps/bogus/index.xml")
	assert.Equal(t, 400, status)
}

func TestSitemapInvalidPage(t *testing.T) {
	app := sitemapTestApp(t)
	status, _ := testGet(t, app, "/sitemaps/track/notanumber.xml")
	assert.Equal(t, 400, status)
}

func TestSitemapContentType(t *testing.T) {
	app := sitemapTestApp(t)
	req := strings.NewReader("")
	_ = req
	status, body := testGet(t, app, "/sitemaps/default.xml")
	require.Equal(t, 200, status)
	assert.Contains(t, string(body), "<?xml")
}

func TestSitemapCacheControlHeaders(t *testing.T) {
	app := sitemapTestApp(t)

	req := httptest.NewRequest("GET", "/sitemaps/track/index.xml", nil)
	res, err := app.Test(req, -1)
	require.NoError(t, err)
	defer res.Body.Close()

	require.Equal(t, 200, res.StatusCode)
	assert.Equal(t, sitemapCacheControl, res.Header.Get("Cache-Control"))
}

func TestSitemapErrorCacheControlHeaders(t *testing.T) {
	app := sitemapTestApp(t)

	req := httptest.NewRequest("GET", "/sitemaps/bogus/index.xml", nil)
	res, err := app.Test(req, -1)
	require.NoError(t, err)
	defer res.Body.Close()

	require.Equal(t, 400, res.StatusCode)
	assert.Equal(t, "no-store", res.Header.Get("Cache-Control"))
}
