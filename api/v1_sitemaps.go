package api

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"api.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

const sitemapLimit = 40_000
const sitemapCountCacheTTL = 1 * time.Hour

var sitemapPageRegex = regexp.MustCompile(`^(\d+)\.xml$`)

var defaultRoutes = []string{
	"legal/privacy-policy",
	"legal/terms-of-use",
	"download",
	"feed",
	"trending",
	"explore",
	"upload",
	"favorites",
	"history",
	"messages",
	"dashboard",
	"explore/playlists",
	"explore/top-albums",
	"explore/remixables",
	"explore/feeling-lucky",
	"explore/chill",
	"explore/upbeat",
	"explore/intense",
	"explore/provoking",
	"explore/intimate",
	"signup",
	"signin",
	"audio",
	"settings",
}

// cachedCount stores a count value with an expiration time.
type cachedCount struct {
	value      int64
	expiresAt  time.Time
	refreshing bool // true if a background refresh is already in flight
}

var (
	sitemapCountCache = make(map[string]cachedCount)
	sitemapCountMu    sync.Mutex
)

// getCachedCount returns the cached value and whether any value exists (even stale).
// If the entry is expired, it returns the stale value with needsRefresh=true.
func getCachedCount(key string) (value int64, exists bool, needsRefresh bool) {
	sitemapCountMu.Lock()
	defer sitemapCountMu.Unlock()
	entry, ok := sitemapCountCache[key]
	if !ok {
		return 0, false, false
	}
	expired := time.Now().After(entry.expiresAt)
	if expired && !entry.refreshing {
		entry.refreshing = true
		sitemapCountCache[key] = entry
		return entry.value, true, true
	}
	return entry.value, true, false
}

func setCachedCount(key string, value int64) {
	sitemapCountMu.Lock()
	defer sitemapCountMu.Unlock()
	sitemapCountCache[key] = cachedCount{
		value:     value,
		expiresAt: time.Now().Add(sitemapCountCacheTTL),
	}
}

type sitemapURL struct {
	XMLName xml.Name `xml:"url"`
	Loc     string   `xml:"loc"`
}

type sitemapURLSet struct {
	XMLName xml.Name     `xml:"urlset"`
	Xmlns   string       `xml:"xmlns,attr"`
	URLs    []sitemapURL `xml:"url"`
}

type sitemapEntry struct {
	XMLName xml.Name `xml:"sitemap"`
	Loc     string   `xml:"loc"`
}

type sitemapIndex struct {
	XMLName  xml.Name       `xml:"sitemapindex"`
	Xmlns    string         `xml:"xmlns,attr"`
	Sitemaps []sitemapEntry `xml:"sitemap"`
}

// escapePathSegments escapes each segment of a path individually,
// preserving the "/" separators.
func escapePathSegments(path string) string {
	parts := strings.Split(path, "/")
	for i, p := range parts {
		parts[i] = url.PathEscape(p)
	}
	return strings.Join(parts, "/")
}

func buildURLSet(urls []string, baseURL string) ([]byte, error) {
	set := sitemapURLSet{
		Xmlns: "http://www.sitemaps.org/schemas/sitemap/0.9",
	}
	for _, u := range urls {
		set.URLs = append(set.URLs, sitemapURL{Loc: baseURL + "/" + escapePathSegments(u)})
	}
	out, err := xml.MarshalIndent(set, "", "  ")
	if err != nil {
		return nil, err
	}
	return append([]byte(xml.Header), out...), nil
}

func buildSitemapIndex(entries []sitemapEntry) ([]byte, error) {
	idx := sitemapIndex{
		Xmlns:    "http://www.sitemaps.org/schemas/sitemap/0.9",
		Sitemaps: entries,
	}
	out, err := xml.MarshalIndent(idx, "", "  ")
	if err != nil {
		return nil, err
	}
	return append([]byte(xml.Header), out...), nil
}

func numPages(count int64, limit int) int {
	if count == 0 {
		return 1
	}
	pages := int(count) / limit
	if int(count)%limit != 0 {
		pages++
	}
	return pages
}

func (app *ApiServer) fetchSitemapCount(entityType string) (int64, error) {
	var count int64
	var err error
	switch entityType {
	case "track":
		count, err = app.queries.GetSitemapTrackCount(context.Background())
	case "playlist":
		count, err = app.queries.GetSitemapPlaylistCount(context.Background())
	case "user":
		count, err = app.queries.GetSitemapUserCount(context.Background())
	}
	if err != nil {
		return 0, err
	}
	setCachedCount(entityType, count)
	return count, nil
}

func (app *ApiServer) getSitemapCount(c *fiber.Ctx, entityType string) (int64, error) {
	cached, exists, needsRefresh := getCachedCount(entityType)

	if needsRefresh {
		// Return stale value immediately, refresh in background
		go func() {
			if _, err := app.fetchSitemapCount(entityType); err != nil {
				app.logger.Error("failed to refresh sitemap count", zap.String("type", entityType), zap.Error(err))
			}
		}()
		return cached, nil
	}

	if exists {
		return cached, nil
	}

	// First request ever — must block
	return app.fetchSitemapCount(entityType)
}

// sitemapDefault serves the root sitemap index that references the static default
// sitemap plus all entity type indexes, so crawlers can discover everything.
func (app *ApiServer) sitemapDefault(c *fiber.Ctx) error {
	base := app.audiusAppUrl
	entries := []sitemapEntry{
		{Loc: base + "/sitemaps/defaults.xml"},
		{Loc: base + "/sitemaps/track/index.xml"},
		{Loc: base + "/sitemaps/playlist/index.xml"},
		{Loc: base + "/sitemaps/user/index.xml"},
	}
	data, err := buildSitemapIndex(entries)
	if err != nil {
		return err
	}
	c.Set("Content-Type", "text/xml")
	return c.Send(data)
}

// sitemapDefaults serves the static default routes urlset.
func (app *ApiServer) sitemapDefaults(c *fiber.Ctx) error {
	data, err := buildURLSet(defaultRoutes, app.audiusAppUrl)
	if err != nil {
		return err
	}
	c.Set("Content-Type", "text/xml")
	return c.Send(data)
}

func (app *ApiServer) sitemapTypeIndex(c *fiber.Ctx) error {
	entityType := c.Params("type")

	switch entityType {
	case "track", "playlist", "user":
	default:
		return fiber.NewError(400, fmt.Sprintf("Invalid sitemap type %s, should be one of track, playlist, user", entityType))
	}

	count, err := app.getSitemapCount(c, entityType)
	if err != nil {
		return err
	}

	pages := numPages(count, sitemapLimit)
	entries := make([]sitemapEntry, pages)
	for i := 1; i <= pages; i++ {
		entries[i-1] = sitemapEntry{
			Loc: fmt.Sprintf("%s/sitemaps/%s/%d.xml", app.audiusAppUrl, entityType, i),
		}
	}

	data, err := buildSitemapIndex(entries)
	if err != nil {
		return err
	}
	c.Set("Content-Type", "text/xml")
	return c.Send(data)
}

func (app *ApiServer) sitemapTypePage(c *fiber.Ctx) error {
	entityType := c.Params("type")
	fileName := c.Params("fileName")

	matches := sitemapPageRegex.FindStringSubmatch(fileName)
	if matches == nil {
		return fiber.NewError(400, fmt.Sprintf("Invalid filepath %s, should be of format <integer>.xml", fileName))
	}
	pageNumber, _ := strconv.Atoi(matches[1])
	if pageNumber < 1 {
		return fiber.NewError(400, "Page number must be >= 1")
	}

	offset := int32((pageNumber - 1) * sitemapLimit)
	ctx := c.Context()
	baseURL := app.audiusAppUrl

	var slugs []string
	var err error

	switch entityType {
	case "track":
		rows, e := app.queries.GetSitemapTrackSlugs(ctx, dbv1.GetSitemapTrackSlugsParams{
			Limit:  int32(sitemapLimit),
			Offset: offset,
		})
		if e != nil {
			return e
		}
		for _, r := range rows {
			if r.Handle.Valid {
				slugs = append(slugs, r.Handle.String+"/"+r.Slug)
			}
		}

	case "playlist":
		rows, e := app.queries.GetSitemapPlaylistSlugs(ctx, dbv1.GetSitemapPlaylistSlugsParams{
			Limit:  int32(sitemapLimit),
			Offset: offset,
		})
		if e != nil {
			return e
		}
		for _, r := range rows {
			if r.Handle.Valid {
				kind := "playlist"
				if r.IsAlbum {
					kind = "album"
				}
				slugs = append(slugs, r.Handle.String+"/"+kind+"/"+r.Slug)
			}
		}

	case "user":
		handles, e := app.queries.GetSitemapUserSlugs(ctx, dbv1.GetSitemapUserSlugsParams{
			Limit:  int32(sitemapLimit),
			Offset: offset,
		})
		if e != nil {
			return e
		}
		for _, h := range handles {
			if h.Valid {
				slugs = append(slugs, h.String)
			}
		}

	default:
		return fiber.NewError(400, fmt.Sprintf("Invalid sitemap type %s, should be one of track, playlist, user", entityType))
	}

	data, err := buildURLSet(slugs, baseURL)
	if err != nil {
		return err
	}
	c.Set("Content-Type", "text/xml")
	return c.Send(data)
}
