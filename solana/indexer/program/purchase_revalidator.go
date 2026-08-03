package program

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"api.audius.co/config"
	"api.audius.co/database"
	"api.audius.co/solana/spl/programs/payment_router"
	"github.com/gagliardetto/solana-go"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

const (
	purchaseRevalidationChannel = "pending_purchase_revalidation"
	sweepInterval               = 5 * time.Minute
	listenerReconnectDelay      = 5 * time.Second
)

// Revalidator resolves sol_purchases rows whose is_valid was left NULL at
// insert time because their valid_after_blocknumber hadn't been indexed yet.
// Triggered by notify_pending_purchase_revalidation when tracks/playlists
// blocknumber advances, plus a periodic sweep for safety.
type Revalidator struct {
	pool   database.DbPool
	config config.Config
	logger *zap.Logger
}

func NewRevalidator(pool database.DbPool, cfg config.Config, logger *zap.Logger) *Revalidator {
	return &Revalidator{
		pool:   pool,
		config: cfg,
		logger: logger.Named("PurchaseRevalidator"),
	}
}

func (r *Revalidator) Start(ctx context.Context) {
	go r.runListener(ctx)
	go r.runSweep(ctx)
}

func (r *Revalidator) runListener(ctx context.Context) {
	for {
		if ctx.Err() != nil {
			return
		}
		if err := r.listenLoop(ctx); err != nil && !errors.Is(err, context.Canceled) {
			r.logger.Error("listener loop ended, reconnecting", zap.Error(err))
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(listenerReconnectDelay):
		}
	}
}

func (r *Revalidator) listenLoop(ctx context.Context) error {
	conn, err := r.pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire listener conn: %w", err)
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, "LISTEN "+purchaseRevalidationChannel); err != nil {
		return fmt.Errorf("LISTEN: %w", err)
	}
	r.logger.Info("listening for purchase revalidation notifications")

	for {
		n, err := conn.Conn().WaitForNotification(ctx)
		if err != nil {
			return err
		}
		contentType, contentId, ok := parseRevalidationPayload(n.Payload)
		if !ok {
			r.logger.Warn("malformed revalidation payload", zap.String("payload", n.Payload))
			continue
		}
		if err := r.revalidateContent(ctx, contentType, contentId); err != nil {
			r.logger.Error("revalidate content failed",
				zap.Error(err),
				zap.String("content_type", contentType),
				zap.Int32("content_id", contentId),
			)
		}
	}
}

func (r *Revalidator) runSweep(ctx context.Context) {
	// Sweep once on startup so rows that went pending while the indexer was
	// down (NOTIFY drops if no listener is connected) get picked up.
	r.sweep(ctx)

	ticker := time.NewTicker(sweepInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.sweep(ctx)
		}
	}
}

func (r *Revalidator) sweep(ctx context.Context) {
	// MAX(blocks.number) is the same gating predicate validatePurchase uses;
	// matching it here avoids waking up rows the validator would just put back
	// to pending.
	sql := `
		SELECT DISTINCT content_type, content_id
		FROM sol_purchases
		WHERE is_valid IS NULL
		  AND valid_after_blocknumber <= (SELECT COALESCE(MAX(number), 0) FROM blocks)
	`
	rows, err := r.pool.Query(ctx, sql)
	if err != nil {
		r.logger.Error("sweep query failed", zap.Error(err))
		return
	}
	type contentRef struct {
		ContentType string
		ContentID   int32
	}
	refs, err := pgx.CollectRows(rows, pgx.RowToStructByPos[contentRef])
	if err != nil {
		r.logger.Error("sweep collect failed", zap.Error(err))
		return
	}
	if len(refs) == 0 {
		return
	}
	r.logger.Info("revalidation sweep", zap.Int("eligible_content_count", len(refs)))
	for _, ref := range refs {
		if ctx.Err() != nil {
			return
		}
		if err := r.revalidateContent(ctx, ref.ContentType, ref.ContentID); err != nil {
			r.logger.Error("sweep revalidate failed",
				zap.Error(err),
				zap.String("content_type", ref.ContentType),
				zap.Int32("content_id", ref.ContentID),
			)
		}
	}
}

type pendingRow struct {
	Signature             string
	InstructionIndex      int32
	ContentType           string
	ContentID             int32
	BuyerUserID           int32
	AccessType            string
	ValidAfterBlocknumber int64
	PurchaseTime          time.Time
}

func (r *Revalidator) revalidateContent(ctx context.Context, contentType string, contentId int32) error {
	// created_at is within seconds of block time for Go-indexer writes — the
	// only rows that can be pending. Used as the timestamp for historical
	// price + payout-wallet lookups in validatePurchase.
	sql := `
		SELECT signature, instruction_index, content_type, content_id,
		       buyer_user_id, access_type, valid_after_blocknumber,
		       COALESCE(created_at, NOW()) AS purchase_time
		FROM sol_purchases
		WHERE content_type = $1
		  AND content_id = $2
		  AND is_valid IS NULL
	`
	rows, err := r.pool.Query(ctx, sql, contentType, contentId)
	if err != nil {
		return fmt.Errorf("query pending rows: %w", err)
	}
	pending, err := pgx.CollectRows(rows, pgx.RowToStructByPos[pendingRow])
	if err != nil {
		return fmt.Errorf("collect pending rows: %w", err)
	}
	for _, p := range pending {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err := r.revalidateRow(ctx, p); err != nil {
			r.logger.Error("revalidate row failed",
				zap.Error(err),
				zap.String("signature", p.Signature),
				zap.Int32("instruction_index", p.InstructionIndex),
			)
		}
	}
	return nil
}

func (r *Revalidator) revalidateRow(ctx context.Context, p pendingRow) error {
	routes, err := r.loadRoutes(ctx, p.Signature, int(p.InstructionIndex))
	if err != nil {
		return fmt.Errorf("load routes: %w", err)
	}
	if len(routes) == 0 {
		return fmt.Errorf("no sol_payments rows for purchase")
	}

	// Reconstruct just enough Route for validatePurchase — only GetRouteMap is
	// consumed downstream, so the sender/owner/bump are zero-valued.
	inst := payment_router.NewRouteInstruction(
		solana.PublicKey{},
		solana.PublicKey{},
		0,
		routes,
	)

	memo := parsedPurchaseMemo{
		ContentType:           p.ContentType,
		ContentId:             int(p.ContentID),
		BuyerUserId:           int(p.BuyerUserID),
		ValidAfterBlocknumber: int(p.ValidAfterBlocknumber),
		AccessType:            p.AccessType,
	}

	isValid, err := validatePurchase(ctx, r.config, r.pool, inst, memo, p.PurchaseTime)
	if err != nil {
		// validatePurchase returns an error alongside a non-nil false isValid
		// when payments don't match expected splits. That's a final verdict;
		// fall through and write it.
		r.logger.Debug("revalidation determined invalid",
			zap.Error(err),
			zap.String("signature", p.Signature),
		)
	}
	if isValid == nil {
		// Still pending — blocks table hasn't actually caught up, or another
		// edge case. Leave for the next notify/sweep.
		r.logger.Warn("revalidation triggered but not ready",
			zap.String("signature", p.Signature),
		)
		return nil
	}

	// Guarded UPDATE: only set if still pending. Prevents racing with another
	// notification handler that finished a moment earlier.
	_, err = r.pool.Exec(ctx, `
		UPDATE sol_purchases
		   SET is_valid = $1
		 WHERE signature = $2
		   AND instruction_index = $3
		   AND is_valid IS NULL
	`, *isValid, p.Signature, p.InstructionIndex)
	if err != nil {
		return fmt.Errorf("update is_valid: %w", err)
	}
	return nil
}

func (r *Revalidator) loadRoutes(ctx context.Context, signature string, instructionIndex int) (map[solana.PublicKey]uint64, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT to_account, amount FROM sol_payments WHERE signature = $1 AND instruction_index = $2`,
		signature, instructionIndex,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	routes := make(map[solana.PublicKey]uint64)
	for rows.Next() {
		var account string
		var amount int64
		if err := rows.Scan(&account, &amount); err != nil {
			return nil, err
		}
		pk, err := solana.PublicKeyFromBase58(account)
		if err != nil {
			return nil, fmt.Errorf("invalid to_account %q: %w", account, err)
		}
		routes[pk] = uint64(amount)
	}
	return routes, rows.Err()
}

func parseRevalidationPayload(payload string) (contentType string, contentId int32, ok bool) {
	parts := strings.SplitN(payload, ":", 2)
	if len(parts) != 2 {
		return "", 0, false
	}
	id, err := strconv.ParseInt(parts[1], 10, 32)
	if err != nil {
		return "", 0, false
	}
	return parts[0], int32(id), true
}
