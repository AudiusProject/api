package api

import (
	"fmt"
	"time"

	"github.com/gagliardetto/solana-go/rpc"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type solanaHealth struct {
	SlotDiff            uint64     `json:"slot_diff"`
	ChainSlot           uint64     `json:"chain_slot"`
	IndexedSlot         uint64     `json:"indexed_slot"`
	LastIndexerUpdateAt *time.Time `json:"last_indexer_update_at"`
	UnprocessedCount    int        `json:"unprocessed_count"`
}

type solanaCheckpoint struct {
	ToSlot    *int64     `db:"to_slot"`
	UpdatedAt *time.Time `db:"updated_at"`
}

const MAX_SLOT_DIFF = 100
const MAX_UNPROCESSED_TXS = 10

func (app *ApiServer) solanaHealth(c *fiber.Ctx) error {
	sql := `
		SELECT 
			to_slot, 
			updated_at
		FROM sol_slot_checkpoints
		ORDER BY updated_at DESC
		LIMIT 1
	`

	rows, err := app.pool.Query(c.Context(), sql)
	if err != nil {
		return err
	}

	checkpoint, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[solanaCheckpoint])
	if err != nil {
		return err
	}

	chainSlot, err := app.solanaRpcClient.GetSlot(c.Context(), rpc.CommitmentConfirmed)
	if err != nil {
		return fmt.Errorf("failed to get chain slot: %w", err)
	}
	health := solanaHealth{
		ChainSlot:           chainSlot,
		LastIndexerUpdateAt: checkpoint.UpdatedAt,
	}

	err = app.pool.QueryRow(c.Context(), `SELECT COUNT(*) FROM sol_unprocessed_txs`).Scan(&health.UnprocessedCount)
	if err != nil {
		if err == pgx.ErrNoRows {
			health.UnprocessedCount = 0
		} else {
			return fmt.Errorf("failed to get unprocessed transactions count: %w", err)
		}
	}

	if checkpoint.ToSlot != nil {
		health.IndexedSlot = uint64(*checkpoint.ToSlot)
	}
	if health.IndexedSlot < health.ChainSlot {
		health.SlotDiff = health.ChainSlot - health.IndexedSlot
	}
	if health.SlotDiff > MAX_SLOT_DIFF {
		c.Status(fiber.StatusInternalServerError)
	}

	if health.UnprocessedCount > MAX_UNPROCESSED_TXS {
		c.Status(fiber.StatusInternalServerError)
	}

	return c.JSON(fiber.Map{
		"data": health,
	})
}
