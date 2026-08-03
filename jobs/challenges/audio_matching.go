package challenges

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// AudioMatchingProcessor implements challenges "b" and "s" — both fire on
// USDC content purchases (tracks/playlists/albums), with different
// recipients and gates.
//
// Source: v_usdc_purchases — a stable view over sol_purchases that exposes
// (buyer_user_id, seller_user_id, content_id, amount, slot, created_at).
//
// Specifier (matches apps for both b and s):
//
//	<hex_buyer_user_id>:<hex_content_id>
//
// Amount = challenge.amount × dollars (catalog: b=1, s=5; dollars = amount/1e6).
// "b" recipient = buyer; "s" recipient = seller, and only if seller is verified.
type AudioMatchingProcessor struct {
	ID         string
	ForBuyer   bool // true => buyer earns; false => seller earns
	VerifyOnly bool // only s gates on seller is_verified
}

func NewAudioMatchingBuyerProcessor() Processor {
	return &AudioMatchingProcessor{ID: "b", ForBuyer: true, VerifyOnly: false}
}
func NewAudioMatchingSellerProcessor() Processor {
	return &AudioMatchingProcessor{ID: "s", ForBuyer: false, VerifyOnly: true}
}

func (p *AudioMatchingProcessor) ChallengeID() string { return p.ID }

const usdcDecimals = 6

func (p *AudioMatchingProcessor) checkpointName() string {
	return "challenges:" + p.ID + ":last_slot"
}

func (p *AudioMatchingProcessor) Reconcile(ctx context.Context, tx pgx.Tx) error {
	c, ok, err := LoadChallenge(ctx, tx, p.ChallengeID())
	if err != nil {
		return fmt.Errorf("load challenge: %w", err)
	}
	if !ok || !c.Active {
		return nil
	}
	catalogAmount := c.AmountInt()

	prev, err := readCheckpointInt(ctx, tx, p.checkpointName())
	if err != nil {
		return fmt.Errorf("read checkpoint: %w", err)
	}

	// VerifyOnly adds a JOIN to filter seller verification at query time so
	// we don't allocate rows we'll throw away.
	verifyJoin := ""
	if p.VerifyOnly {
		verifyJoin = `
			JOIN users seller ON seller.user_id = v.seller_user_id
			    AND seller.is_current = true
			    AND seller.is_verified = true
		`
	}

	query := `
		SELECT v.slot, v.buyer_user_id, v.seller_user_id, v.content_id, v.amount
		FROM v_usdc_purchases v
		` + verifyJoin + `
		WHERE v.slot > $1
		  AND v.seller_user_id IS NOT NULL
		ORDER BY v.slot ASC
	`
	rows, err := tx.Query(ctx, query, prev)
	if err != nil {
		return fmt.Errorf("scan purchases: %w", err)
	}
	type purchaseRow struct {
		slot       int64
		buyerID    int64
		sellerID   int64
		contentID  int64
		amountMicro int64
	}
	var results []purchaseRow
	maxSlot := prev
	for rows.Next() {
		var r purchaseRow
		if err := rows.Scan(&r.slot, &r.buyerID, &r.sellerID, &r.contentID, &r.amountMicro); err != nil {
			rows.Close()
			return err
		}
		results = append(results, r)
		if r.slot > maxSlot {
			maxSlot = r.slot
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	for _, r := range results {
		dollars := int32(r.amountMicro / 1_000_000)
		if dollars <= 0 {
			continue // shouldn't happen for valid purchases; defensive
		}
		recipient := r.buyerID
		if !p.ForBuyer {
			recipient = r.sellerID
		}
		rewardAmount := catalogAmount * dollars
		specifier := fmt.Sprintf("%x:%x", r.buyerID, r.contentID)
		if err := UpsertUserChallenge(ctx, tx,
			p.ChallengeID(), specifier, recipient, 1, 1, rewardAmount,
		); err != nil {
			return fmt.Errorf("upsert: %w", err)
		}
	}

	if maxSlot > prev {
		if err := writeCheckpointInt(ctx, tx, p.checkpointName(), maxSlot); err != nil {
			return fmt.Errorf("save checkpoint: %w", err)
		}
	}
	return nil
}
