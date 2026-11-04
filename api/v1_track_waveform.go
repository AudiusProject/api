package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"api.audius.co/rendezvous"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
	"github.com/maypok86/otter"
	"golang.org/x/sync/singleflight"
)

// Content nodes precompute a 750-bucket amplitude envelope per track
// (GET /waveform/:cid). Only nodes that opt in compute them, so most lookups
// miss. Proxying here keeps clients off the nodes and caches both outcomes,
// so a single waveform node is not hit once per feed tile.

const (
	waveformProbeHosts   = 4
	waveformProbeTimeout = 3 * time.Second
	waveformMissTTL      = 10 * time.Minute
)

type waveform struct {
	DurationMs int64 `json:"duration_ms"`
	Buckets    int   `json:"buckets"`
	Peaks      []int `json:"peaks"`
}

// nodeWaveform is the node's response. Peaks is []byte, so it arrives base64.
type nodeWaveform struct {
	DurationMs int64  `json:"duration_ms"`
	Buckets    int    `json:"buckets"`
	Peaks      []byte `json:"peaks"`
}

var (
	waveformClient = &http.Client{Timeout: waveformProbeTimeout}
	waveformHits   = mustCache(otter.MustBuilder[string, *waveform](20_000).Build())
	waveformMisses = mustCache(otter.MustBuilder[string, bool](100_000).WithTTL(waveformMissTTL).Build())
	waveformGroup  singleflight.Group
)

func mustCache[K comparable, V any](cache otter.Cache[K, V], err error) otter.Cache[K, V] {
	if err != nil {
		panic(err)
	}
	return cache
}

func (app *ApiServer) v1TrackWaveform(c *fiber.Ctx) error {
	trackId := c.Locals("trackId").(int)

	var cid string
	err := app.pool.QueryRow(c.Context(), `
		SELECT coalesce(track_cid, '')
		FROM tracks
		WHERE track_id = $1 AND is_current AND NOT is_delete
	`, trackId).Scan(&cid)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && cid == "") {
		return fiber.NewError(fiber.StatusNotFound, "track not found")
	}
	if err != nil {
		return err
	}

	w, err := app.getWaveform(c.Context(), cid)
	if err != nil {
		return err
	}
	if w == nil {
		c.Set(fiber.HeaderCacheControl, fmt.Sprintf("public, max-age=%d", int(waveformMissTTL.Seconds())))
		return fiber.NewError(fiber.StatusNotFound, "waveform not found")
	}

	// Keyed by track id, and a replaced track gets a new cid, so not immutable.
	c.Set(fiber.HeaderCacheControl, "public, max-age=86400")
	return c.JSON(fiber.Map{"data": w})
}

// getWaveform returns nil, nil when no node has a waveform for cid.
func (app *ApiServer) getWaveform(ctx context.Context, cid string) (*waveform, error) {
	if w, ok := waveformHits.Get(cid); ok {
		return w, nil
	}
	if _, ok := waveformMisses.Get(cid); ok {
		return nil, nil
	}

	v, err, _ := waveformGroup.Do(cid, func() (any, error) {
		w := fetchWaveform(context.WithoutCancel(ctx), app.waveformHosts(cid), cid)
		if w == nil {
			waveformMisses.Set(cid, true)
		} else {
			waveformHits.Set(cid, w)
		}
		return w, nil
	})
	if err != nil {
		return nil, err
	}
	return v.(*waveform), nil
}

func (app *ApiServer) waveformHosts(cid string) []string {
	if app.waveformHostsOverride != nil {
		return app.waveformHostsOverride(cid)
	}
	return waveformProbeOrder(app.config.WaveformHosts, rendezvous.GlobalHasher.Rank(cid))
}

// waveformProbeOrder puts the configured waveform hosts first, then fills up to
// waveformProbeHosts from the rendezvous ranking.
func waveformProbeOrder(configured, ranked []string) []string {
	hosts := append([]string{}, configured...)
	seen := make(map[string]bool, len(hosts))
	for _, h := range hosts {
		seen[h] = true
	}
	for _, h := range ranked {
		if len(hosts) >= waveformProbeHosts {
			break
		}
		if !seen[h] {
			seen[h] = true
			hosts = append(hosts, h)
		}
	}
	return hosts
}

// fetchWaveform asks the top rendezvous hosts in parallel and takes the first
// answer. localOnly stops each node from probing its own peers.
func fetchWaveform(ctx context.Context, hosts []string, cid string) *waveform {
	if len(hosts) == 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, waveformProbeTimeout)
	defer cancel()

	results := make(chan *waveform, len(hosts))
	for _, host := range hosts {
		go func(host string) {
			results <- fetchWaveformFrom(ctx, host, cid)
		}(host)
	}

	for range hosts {
		if w := <-results; w != nil {
			return w
		}
	}
	return nil
}

func fetchWaveformFrom(ctx context.Context, host, cid string) *waveform {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/waveform/%s?localOnly=true", host, cid), nil)
	if err != nil {
		return nil
	}
	resp, err := waveformClient.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil
	}

	var nw nodeWaveform
	if err := json.NewDecoder(resp.Body).Decode(&nw); err != nil || len(nw.Peaks) == 0 {
		return nil
	}

	peaks := make([]int, len(nw.Peaks))
	for i, p := range nw.Peaks {
		peaks[i] = int(p)
	}
	return &waveform{
		DurationMs: nw.DurationMs,
		Buckets:    len(peaks),
		Peaks:      peaks,
	}
}
