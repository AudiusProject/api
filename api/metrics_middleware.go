package api

import (
	"context"
	"runtime"
	"strings"
	"sync"
	"time"

	"api.audius.co/hll"
	"api.audius.co/utils"
	"github.com/axiomhq/hyperloglog"
	"github.com/gofiber/fiber/v2"
	fiberutils "github.com/gofiber/fiber/v2/utils"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/maypok86/otter"
	"go.uber.org/zap"
)

// MetricsCollector handles in-memory collection and periodic flushing of request metrics
type MetricsCollector struct {
	logger     *zap.Logger
	writePool  *pgxpool.Pool
	serverId   string
	flushTimer *time.Ticker
	stopCh     chan struct{}

	appMetrics       otter.Cache[string, *AppMetricsData]
	routeMetrics     otter.Cache[string, *RouteMetricsData]
	countMetrics     *hll.HLL
	appUniqueMetrics map[string]*hll.HLL
	appUniqueMu      sync.RWMutex
}

// AppMetricsData holds request count data for a specific app identifier
type AppMetricsData struct {
	ApiKey       string
	AppName      string
	RequestCount int64
	LastSeen     time.Time
}

// RouteMetricsData holds request count data for a specific route pattern
type RouteMetricsData struct {
	RoutePattern string
	Method       string
	RequestCount int64
	LastSeen     time.Time
}

func NewMetricsCollector(logger *zap.Logger, writePool *pgxpool.Pool) *MetricsCollector {
	flushTimer := 1 * time.Minute

	appMetricsCache, err := otter.
		// 250 bytes per entry, 1M entries = 250MB
		MustBuilder[string, *AppMetricsData](1_000_000).
		// Timeout entries after 10 flushes
		WithTTL(10 * flushTimer).
		Build()
	if err != nil {
		logger.Error("Failed to create app metrics cache", zap.Error(err))
		panic(err)
	}

	routeMetricsCache, err := otter.
		// 250 bytes per entry, 1M entries = 250MB
		MustBuilder[string, *RouteMetricsData](1_000_000).
		// Timeout entries after 10 flushes
		WithTTL(10 * flushTimer).
		Build()
	if err != nil {
		logger.Error("Failed to create route metrics cache", zap.Error(err))
		panic(err)
	}

	countMetricsAggregator, err := hll.NewHLL(logger, writePool, "api_metrics_counts", 12)
	if err != nil {
		logger.Error("Failed to create count metrics", zap.Error(err))
		panic(err)
	}

	collector := &MetricsCollector{
		logger:           logger.With(zap.String("component", "MetricsCollector")),
		writePool:        writePool,
		appMetrics:       appMetricsCache,
		routeMetrics:     routeMetricsCache,
		countMetrics:     countMetricsAggregator,
		appUniqueMetrics: make(map[string]*hll.HLL),
		flushTimer:       time.NewTicker(flushTimer),
		stopCh:           make(chan struct{}),
	}

	// Start the flush routine
	go collector.flushRoutine()

	return collector
}

// Fiber middleware that collects metrics. Pass apiServer to resolve identifier from Basic Auth signer first; if nil or no signer, falls back to api_key/app_name query params.
func (rmc *MetricsCollector) Middleware(apiServer *ApiServer) fiber.Handler {
	return func(c *fiber.Ctx) error {
		err := c.Next()

		var apiKey, appName string
		if apiServer != nil {
			signer, signerErr := apiServer.getApiSigner(c)
			if signerErr == nil && signer != nil {
				apiKey = fiberutils.CopyString(strings.ToLower(signer.Address))
			}
		}
		if apiKey == "" && appName == "" {
			apiKey = c.Query("api_key")
			appName = c.Query("app_name")
		}
		ipAddress := utils.GetIP(c)

		// Only record if we have some identifier
		if apiKey != "" || appName != "" {
			rmc.recordAppMetric(
				fiberutils.CopyString(apiKey),
				fiberutils.CopyString(appName),
				ipAddress,
			)
		}

		// Record route metrics for all requests
		if route := c.Route(); route != nil {
			routePattern := route.Path
			method := c.Method()
			rmc.recordRouteMetric(
				fiberutils.CopyString(routePattern),
				fiberutils.CopyString(method),
			)
		}

		// Extract IP address for unique tracking (global)
		if ipAddress != "" {
			rmc.recordCountMetric(ipAddress)
		}

		return err
	}
}

// Increments the request count for a given app identifier
func (rmc *MetricsCollector) recordAppMetric(apiKey, appName, ipAddress string) {
	// Prioritize api_key over app_name as identifier
	identifier := apiKey
	if identifier == "" {
		identifier = appName
	}

	// Get existing data or create new and increment count
	data, exists := rmc.appMetrics.Get(identifier)
	lastSeen := time.Now()
	if !exists {
		data = &AppMetricsData{
			ApiKey:       apiKey,
			AppName:      appName,
			RequestCount: 0,
			LastSeen:     lastSeen,
		}
	}
	data.RequestCount++
	data.LastSeen = lastSeen
	rmc.appMetrics.Set(identifier, data)

	// Record IP address to app-specific HLL sketch for unique user tracking
	if ipAddress != "" {
		rmc.appUniqueMu.Lock()
		appHLL, exists := rmc.appUniqueMetrics[identifier]
		if !exists {
			// Create new HLL instance for this app
			var err error
			appHLL, err = hll.NewHLL(rmc.logger, rmc.writePool, "api_metrics_apps_unique", 12)
			if err != nil {
				rmc.logger.Error("Failed to create app unique HLL", zap.Error(err), zap.String("identifier", identifier))
				rmc.appUniqueMu.Unlock()
				return
			}
			rmc.appUniqueMetrics[identifier] = appHLL
		}
		rmc.appUniqueMu.Unlock()

		appHLL.Record(ipAddress)
	}
}

// Increments the request count for a given route pattern
func (rmc *MetricsCollector) recordRouteMetric(routePattern, method string) {
	// Get existing data or create new and increment count
	data, exists := rmc.routeMetrics.Get(routePattern)
	lastSeen := time.Now()
	if !exists {
		data = &RouteMetricsData{
			RoutePattern: routePattern,
			Method:       method,
			RequestCount: 0,
			LastSeen:     lastSeen,
		}
	}
	data.RequestCount++
	data.LastSeen = lastSeen
	rmc.routeMetrics.Set(routePattern, data)
}

// Records an IP address in count metrics hll sketch
func (rmc *MetricsCollector) recordCountMetric(ipAddress string) {
	rmc.countMetrics.Record(ipAddress)
}

// flushRoutine runs periodically to flush metrics to the database
func (rmc *MetricsCollector) flushRoutine() {
	for {
		select {
		case <-rmc.flushTimer.C:
			rmc.flushMetrics()
		case <-rmc.stopCh:
			rmc.logger.Info("Stopping metrics flush routine")
			return
		}
	}
}

// flushMetrics writes the accumulated metrics to the database
func (rmc *MetricsCollector) flushMetrics() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Get all current metrics and clear the caches
	currentMetrics := make(map[string]*AppMetricsData)
	rmc.appMetrics.Range(func(key string, value *AppMetricsData) bool {
		currentMetrics[key] = value
		return true
	})

	currentRouteMetrics := make(map[string]*RouteMetricsData)
	rmc.routeMetrics.Range(func(key string, value *RouteMetricsData) bool {
		currentRouteMetrics[key] = value
		return true
	})

	// Clear the caches after collecting data
	rmc.appMetrics.Clear()
	rmc.routeMetrics.Clear()

	// Get HLL sketch copy
	currentHLL, currentTotalRequests := rmc.countMetrics.GetSketchCopy()

	type AppUniqueData struct {
		Identifier string
		Sketch     *hyperloglog.Sketch
		TotalCount int64
	}
	appUniqueData := make(map[string]*AppUniqueData)
	rmc.appUniqueMu.Lock()
	for identifier, appHLL := range rmc.appUniqueMetrics {
		sketchCopy, totalCount := appHLL.GetSketchCopy()
		appUniqueData[identifier] = &AppUniqueData{
			Identifier: identifier,
			Sketch:     sketchCopy,
			TotalCount: totalCount,
		}
	}
	rmc.appUniqueMu.Unlock()

	// Begin transaction
	tx, err := rmc.writePool.Begin(ctx)
	if err != nil {
		rmc.logger.Error("Failed to begin transaction for metrics flush", zap.Error(err))
		return
	}
	defer tx.Rollback(ctx)

	// Get current date bucket
	date := time.Now().Format(time.DateOnly)

	// Flush app metrics using UPSERT
	if len(currentMetrics) > 0 {
		appUpserted := 0
		for _, data := range currentMetrics {
			_, err := tx.Exec(ctx, `
				INSERT INTO api_metrics_apps (date, api_key, app_name, request_count, created_at, updated_at)
				VALUES ($1, $2, $3, $4, NOW(), NOW())
				ON CONFLICT (date, api_key, app_name)
				DO UPDATE SET
					request_count = api_metrics_apps.request_count + EXCLUDED.request_count,
					updated_at = NOW()
			`, date, data.ApiKey, data.AppName, data.RequestCount)

			if err != nil {
				rmc.logger.Error("Failed to upsert app metrics",
					zap.Error(err),
					zap.String("api_key", data.ApiKey),
					zap.String("app_name", data.AppName))
				continue
			}
			appUpserted++
		}
		rmc.logger.Debug("Flushed app metrics",
			zap.Int("upserted", appUpserted),
			zap.Int("total", len(currentMetrics)))
	}

	// Flush route metrics using UPSERT
	if len(currentRouteMetrics) > 0 {
		routeUpserted := 0
		for _, data := range currentRouteMetrics {
			_, err := tx.Exec(ctx, `
				INSERT INTO api_metrics_routes (date, route_pattern, method, request_count, created_at, updated_at)
				VALUES ($1, $2, $3, $4, NOW(), NOW())
				ON CONFLICT (date, route_pattern, method)
				DO UPDATE SET
					request_count = api_metrics_routes.request_count + EXCLUDED.request_count,
					updated_at = NOW()
			`, date, data.RoutePattern, data.Method, data.RequestCount)

			if err != nil {
				rmc.logger.Error("Failed to upsert route metrics",
					zap.Error(err),
					zap.String("route_pattern", data.RoutePattern),
					zap.String("method", data.Method))
				continue
			}
			routeUpserted++
		}
		rmc.logger.Debug("Flushed route metrics",
			zap.Int("upserted", routeUpserted),
			zap.Int("total", len(currentRouteMetrics)))
	}

	// Aggregate HLL sketch directly into daily table if we have requests
	if currentTotalRequests > 0 && currentHLL != nil {
		if err := rmc.countMetrics.AggregateSketch(
			ctx,
			tx,
			currentHLL,
			currentTotalRequests,
			date,
		); err != nil {
			rmc.logger.Error("Failed to aggregate count metrics hll sketch",
				zap.Error(err),
				zap.String("date", date))
		} else {
			rmc.logger.Debug("Aggregated HLL sketch directly",
				zap.String("date", date),
				zap.Int64("total_count", currentTotalRequests),
				zap.Uint64("unique_count", currentHLL.Estimate()))
		}
	}

	// Flush app unique metrics
	if len(appUniqueData) > 0 {
		appUniqueUpserted := 0
		for _, data := range appUniqueData {
			if data.Sketch == nil {
				continue
			}

			// Clone the sketch to avoid modifying the original
			newSketch := data.Sketch.Clone()
			if newSketch == nil {
				continue
			}

			var existingSketchData []byte
			var existingCount int64
			query := `
				SELECT hll_sketch, total_count 
				FROM api_metrics_apps_unique 
				WHERE date = $1 AND app_name = $2 
				FOR UPDATE`
			err = tx.QueryRow(ctx, query, date, data.Identifier).Scan(&existingSketchData, &existingCount)

			if err != nil && err != pgx.ErrNoRows {
				rmc.logger.Error("Failed to query existing app unique metrics",
					zap.Error(err),
					zap.String("identifier", data.Identifier))
				continue
			}

			var finalSketchData []byte
			var finalTotalCount int64
			var finalUniqueCount int64

			if err == pgx.ErrNoRows {
				// No existing row - use new sketch as-is
				var marshalErr error
				finalSketchData, marshalErr = newSketch.MarshalBinary()
				if marshalErr != nil {
					rmc.logger.Error("Failed to marshal new sketch",
						zap.Error(marshalErr),
						zap.String("identifier", data.Identifier))
					continue
				}
				finalTotalCount = data.TotalCount
				finalUniqueCount = int64(newSketch.Estimate())
			} else {
				// Row exists - merge sketches
				if existingSketchData != nil {
					// Merge with existing sketch
					existingSketch, unmarshalErr := hll.UnmarshalSketch(existingSketchData, 12)
					if unmarshalErr != nil {
						rmc.logger.Error("Failed to unmarshal existing sketch",
							zap.Error(unmarshalErr),
							zap.String("identifier", data.Identifier))
						continue
					}

					if mergeErr := existingSketch.Merge(newSketch); mergeErr != nil {
						rmc.logger.Error("Failed to merge sketches",
							zap.Error(mergeErr),
							zap.String("identifier", data.Identifier))
						continue
					}

					var marshalErr error
					finalSketchData, marshalErr = existingSketch.MarshalBinary()
					if marshalErr != nil {
						rmc.logger.Error("Failed to marshal merged sketch",
							zap.Error(marshalErr),
							zap.String("identifier", data.Identifier))
						continue
					}
					finalUniqueCount = int64(existingSketch.Estimate())
				} else {
					// Row exists but sketch is NULL - use new sketch
					var marshalErr error
					finalSketchData, marshalErr = newSketch.MarshalBinary()
					if marshalErr != nil {
						rmc.logger.Error("Failed to marshal new sketch",
							zap.Error(marshalErr),
							zap.String("identifier", data.Identifier))
						continue
					}
					finalUniqueCount = int64(newSketch.Estimate())
				}
				finalTotalCount = existingCount + data.TotalCount
			}

			// Use INSERT ... ON CONFLICT for atomic upsert (same pattern as api_metrics_apps)
			upsertQuery := `
				INSERT INTO api_metrics_apps_unique (date, app_name, hll_sketch, total_count, unique_count, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
				ON CONFLICT (date, app_name)
				DO UPDATE SET
					hll_sketch = EXCLUDED.hll_sketch,
					total_count = EXCLUDED.total_count,
					unique_count = EXCLUDED.unique_count,
					updated_at = NOW()`

			_, err = tx.Exec(ctx, upsertQuery, date, data.Identifier, finalSketchData, finalTotalCount, finalUniqueCount)
			if err != nil {
				rmc.logger.Error("Failed to upsert app unique metrics",
					zap.Error(err),
					zap.String("identifier", data.Identifier))
				continue
			}

			appUniqueUpserted++
		}

		rmc.logger.Debug("Flushed app unique metrics",
			zap.Int("upserted", appUniqueUpserted),
			zap.Int("total", len(appUniqueData)))
	}

	// Commit transaction
	if err := tx.Commit(ctx); err != nil {
		rmc.logger.Error("Failed to commit metrics transaction", zap.Error(err))
		return
	}

	rmc.logger.Debug("Successfully flushed metrics",
		zap.Int("api_metrics_apps", len(currentMetrics)),
		zap.Int("route_metrics", len(currentRouteMetrics)),
		zap.Int64("total_count", currentTotalRequests))
}

func (rmc *MetricsCollector) Debug() map[string]interface{} {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	result := map[string]interface{}{
		"memory_stats": map[string]interface{}{
			"used_bytes":        memStats.Alloc,
			"total_bytes":       memStats.TotalAlloc,
			"sys_bytes":         memStats.Sys,
			"usage_percent":     float64(memStats.Alloc) / float64(memStats.TotalAlloc) * 100,
			"num_gc":            memStats.NumGC,
			"last_gc":           time.Unix(0, int64(memStats.LastGC)),
			"gc_pause_total_ns": memStats.PauseTotalNs,
		},

		"api_metrics_apps_cache_size":   rmc.appMetrics.Size(),
		"api_metrics_routes_cache_size": rmc.routeMetrics.Size(),
	}
	countStats := rmc.countMetrics.GetStats()
	for k, v := range countStats {
		result["api_metrics_counts_"+k] = v
	}

	return result
}

func (rmc *MetricsCollector) Shutdown() {
	rmc.logger.Info("Shutting down metrics collector")
	close(rmc.stopCh)
	rmc.flushTimer.Stop()
	rmc.flushMetrics()
	rmc.logger.Info("Metrics collector shutdown complete")
}
