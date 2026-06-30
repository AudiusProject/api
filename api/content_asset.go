package api

import (
	"net/url"
	"slices"
	"strings"

	"api.audius.co/config"
	"api.audius.co/rendezvous"
	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

const contentAssetUpstreamHeader = "X-Audius-Upstream"

func (app *ApiServer) contentAssetRedirect(c *fiber.Ctx) error {
	cid := c.Params("cid")
	asset := c.Params("asset")

	if !isValidContentPathSegment(cid) || (asset != "" && !isValidContentPathSegment(asset)) {
		return fiber.NewError(fiber.StatusBadRequest, "invalid content path")
	}

	target, upstream, ok := contentAssetTarget(cid, asset, string(c.Context().URI().QueryString()))
	if !ok {
		return fiber.NewError(fiber.StatusServiceUnavailable, "no content nodes available")
	}

	c.Locals("upstream", upstream)
	c.Set(contentAssetUpstreamHeader, upstream)
	c.Set(fiber.HeaderCacheControl, "public, max-age=300")
	app.logger.Debug("content asset redirect",
		zap.String("cid", cid),
		zap.String("asset", asset),
		zap.String("upstream", upstream),
	)

	return c.Redirect(target, fiber.StatusTemporaryRedirect)
}

func contentAssetTarget(cid string, asset string, rawQuery string) (string, string, bool) {
	for _, endpoint := range contentAssetCandidates(cid) {
		target, ok := contentAssetURL(endpoint, cid, asset, rawQuery)
		if ok {
			return target, endpoint, true
		}
	}
	return "", "", false
}

func contentAssetCandidates(cid string) []string {
	candidates := make([]string, 0, len(config.Cfg.StoreAllNodes)+4)
	seen := map[string]bool{}

	appendCandidate := func(endpoint string) {
		endpoint = strings.TrimRight(strings.TrimSpace(endpoint), "/")
		if endpoint == "" || seen[endpoint] || isBlockedContentEndpoint(endpoint) {
			return
		}
		u, err := url.Parse(endpoint)
		if err != nil || u.Scheme == "" || u.Host == "" {
			return
		}
		seen[endpoint] = true
		candidates = append(candidates, endpoint)
	}

	for _, endpoint := range config.Cfg.StoreAllNodes {
		appendCandidate(endpoint)
	}

	first, rest := rendezvous.GlobalHasher.Select(cid)
	appendCandidate(first)
	for _, endpoint := range rest {
		appendCandidate(endpoint)
	}

	return candidates
}

func contentAssetURL(endpoint string, cid string, asset string, rawQuery string) (string, bool) {
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", false
	}

	path := "/content/" + url.PathEscape(cid)
	if asset != "" {
		path += "/" + url.PathEscape(asset)
	}
	u.Path = path
	u.RawPath = ""
	u.RawQuery = rawQuery
	u.Fragment = ""

	return u.String(), true
}

func isBlockedContentEndpoint(endpoint string) bool {
	return slices.Contains(config.Cfg.BlacklistedNodes, endpoint) ||
		slices.Contains(config.Cfg.DeadNodes, endpoint)
}

func isValidContentPathSegment(segment string) bool {
	if segment == "" || segment == "." || segment == ".." {
		return false
	}
	return !strings.ContainsAny(segment, `/\`+"\x00")
}
