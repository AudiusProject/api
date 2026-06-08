package api

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"time"

	"api.audius.co/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/maypok86/otter"
	"go.uber.org/zap"
)

const defaultRPS = 5

// rpmMonthCacheTTL - on expiry, next request triggers DB refresh to pick up flushed metrics.
const rpmMonthCacheTTL = 5 * time.Minute

type apiKeysLimits struct {
	// Requests per second
	RPS int
	// Requests per month
	RPM int
}

// rpsState tracks request timestamps per identifier for sliding window RPS
type rpsState struct {
	mu   sync.Mutex
	data map[string][]int64
}

// normalizeAPIKeyForLookup canonicalizes api_key for exact primary-key lookup.
func normalizeAPIKeyForLookup(apiKey string) string {
	if apiKey == "" {
		return ""
	}
	apiKey = strings.ToLower(apiKey)
	if strings.HasPrefix(apiKey, "0x") {
		return apiKey
	}
	return "0x" + apiKey
}

func (r *rpsState) allow(identifier string, limit int, now int64) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.data == nil {
		r.data = make(map[string][]int64)
	}
	windowStart := now - int64(time.Second)
	// Prune old timestamps
	timestamps := r.data[identifier]
	i := 0
	for _, ts := range timestamps {
		if ts > windowStart {
			timestamps[i] = ts
			i++
		}
	}
	timestamps = timestamps[:i]
	if len(timestamps) >= limit {
		return false
	}
	timestamps = append(timestamps, now)
	r.data[identifier] = timestamps
	return true
}

// NewRateLimitMiddleware creates middleware that enforces RPS and RPM (requests per month) from api_keys.
func NewRateLimitMiddleware(logger *zap.Logger, writePool *pgxpool.Pool) *RateLimitMiddleware {
	limitsCache, err := otter.MustBuilder[string, apiKeysLimits](50_000).
		WithTTL(5 * time.Minute).
		CollectStats().
		Build()
	if err != nil {
		panic(err)
	}
	rpmMonthCacheVal, err := otter.MustBuilder[string, int64](50_000).
		WithTTL(rpmMonthCacheTTL).
		Build()
	if err != nil {
		panic(err)
	}
	return &RateLimitMiddleware{
		logger:          logger.With(zap.String("component", "RateLimitMiddleware")),
		writePool:       writePool,
		limitsCache:     &limitsCache,
		rpmMonthCache:   &rpmMonthCacheVal,
		rpmMonthMu:      &sync.Mutex{},
		rpmMonthPending: make(map[string]int64),
		rpsState:        &rpsState{data: make(map[string][]int64)},
	}
}

// RateLimitMiddleware enforces RPS and RPM (requests per month) from api_keys table.
type RateLimitMiddleware struct {
	logger          *zap.Logger
	writePool       *pgxpool.Pool
	limitsCache     *otter.Cache[string, apiKeysLimits]
	rpmMonthCache   *otter.Cache[string, int64] // base count from api_metrics_apps at last refresh
	rpmMonthMu      *sync.Mutex
	rpmMonthPending map[string]int64 // identifier -> count allowed since last DB refresh
	rpsState        *rpsState
}

// Middleware returns the Fiber handler. Pass apiServer to resolve identifier from Bearer or Basic Auth signer first; if nil or no signer, falls back to api_key/app_name query params.
func (rlm *RateLimitMiddleware) Middleware(apiServer *ApiServer) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Skip rate limiting for relay endpoints
		if strings.HasPrefix(c.Path(), "/relay") {
			return c.Next()
		}

		var identifier string
		if apiServer != nil {
			signer, err := apiServer.getApiSigner(c)
			if err == nil && signer != nil {
				identifier = strings.ToLower(signer.Address)
			}
		}
		if identifier == "" {
			apiKey := normalizeAPIKeyForLookup(c.Query("api_key"))
			appName := c.Query("app_name")
			identifier = apiKey
			if identifier == "" {
				identifier = appName
			}
		}
		ipAddress := utils.GetIP(c)

		// Resolve rate limits
		rps, rpm, useDefault := rlm.getLimits(c.Context(), identifier)
		if useDefault {
			// Unlisted: RPS only, no RPM; key by IP when no identifier
			rps = defaultRPS
			if identifier == "" {
				identifier = "ip:" + ipAddress
			}
		}

		now := time.Now().UnixNano()

		// Check RPS
		if !rlm.rpsState.allow(identifier, rps, now) {
			rlm.logger.Warn("RPS limit exceeded",
				zap.String("limit_type", "rps"),
				zap.String("identifier", identifier),
				zap.String("path", c.Path()),
				zap.String("ip", ipAddress),
				zap.Int("limit", rps))
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error": "Rate limit exceeded. Try again later.",
			})
		}

		// Check RPM only for apps with an api_key (useDefault=false)
		var remainingMonth int64 = -1 // -1 = not applicable (no RPM check)
		if !useDefault {
			allowed, remaining := rlm.checkRpm(c.Context(), identifier, int64(rpm))
			remainingMonth = remaining
			if !allowed {
				rlm.logger.Warn("RPM limit exceeded",
					zap.String("limit_type", "rpm"),
					zap.String("identifier", identifier),
					zap.String("path", c.Path()),
					zap.String("ip", ipAddress),
					zap.Int("limit", rpm))
				c.Set("X-RateLimit-Remaining-Month", "0")
				c.Set("X-RateLimit-Limit-Month", strconv.Itoa(rpm))
				return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
					"error": "Rate limit exceeded. Try again later.",
				})
			}
		}

		if !useDefault && remainingMonth >= 0 {
			c.Set("X-RateLimit-Remaining-Month", strconv.FormatInt(remainingMonth, 10))
			c.Set("X-RateLimit-Limit-Month", strconv.FormatInt(int64(rpm), 10))
		}
		return c.Next()
	}
}

// checkRpm returns (allowed, remaining). remaining = requests left in the month after this one.
// On cache miss: queries api_metrics_apps and loads into cache. On cache hit: uses
// baseCount + pending and increments pending on allow.
func (rlm *RateLimitMiddleware) checkRpm(ctx context.Context, identifier string, limit int64) (allowed bool, remaining int64) {
	if rlm.writePool == nil {
		return true, limit
	}
	baseCount, ok := rlm.rpmMonthCache.Get(identifier)
	if !ok {
		var dbCount int64
		err := rlm.writePool.QueryRow(ctx, `
			SELECT COALESCE(SUM(request_count), 0)
			FROM api_metrics_apps
			WHERE (LOWER(api_key) = LOWER($1) OR LOWER(app_name) = LOWER($1))
			  AND date >= CURRENT_DATE - INTERVAL '30 days'
		`, identifier).Scan(&dbCount)
		if err != nil {
			rlm.logger.Debug("Failed to get monthly request count", zap.String("identifier", identifier), zap.Error(err))
			return true, limit
		}
		baseCount = dbCount
		rlm.rpmMonthCache.Set(identifier, baseCount)
		rlm.rpmMonthMu.Lock()
		rlm.rpmMonthPending[identifier] = 0
		rlm.rpmMonthMu.Unlock()
	}

	rlm.rpmMonthMu.Lock()
	pending := rlm.rpmMonthPending[identifier]
	total := baseCount + pending
	if total >= limit {
		rlm.rpmMonthMu.Unlock()
		return false, 0
	}
	rlm.rpmMonthPending[identifier] = pending + 1
	rlm.rpmMonthMu.Unlock()
	// remaining = limit - count after this request
	return true, limit - total - 1
}

// getLimits returns rps, rpm (requests per month), and useDefault (true if identifier not in api_keys).
func (rlm *RateLimitMiddleware) getLimits(ctx context.Context, identifier string) (rps, rpm int, useDefault bool) {
	if identifier == "" {
		return 0, 0, true
	}
	if hit, ok := rlm.limitsCache.Get(identifier); ok {
		return hit.RPS, hit.RPM, false
	}
	if rlm.writePool == nil {
		return 0, 0, true
	}
	var rpsVal, rpmVal int
	err := rlm.writePool.QueryRow(ctx, `
		SELECT COALESCE(rps, 10), COALESCE(rpm, 500000)
		FROM api_keys
		WHERE api_key = $1
	`, identifier).Scan(&rpsVal, &rpmVal)
	if err == pgx.ErrNoRows || err != nil {
		return 0, 0, true
	}
	rlm.limitsCache.Set(identifier, apiKeysLimits{RPS: rpsVal, RPM: rpmVal})
	return rpsVal, rpmVal, false
}
