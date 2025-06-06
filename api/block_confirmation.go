package api

import (
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type BlockConfirmationQueryParams struct {
	BlockHash   string `query:"blockhash"`
	BlockNumber int64  `query:"blocknumber"`
}

func (app *ApiServer) BlockConfirmation(c *fiber.Ctx) error {
	var params BlockConfirmationQueryParams
	err := app.ParseAndValidateQueryParams(c, &params)
	if err != nil {
		return err
	}

	sql := `
	SELECT
		(
			SELECT EXISTS (
				SELECT 1 FROM core_blocks 
				WHERE chain_id = @chainId AND height > @blockNumber 
				LIMIT 1
			)
		) AS block_passed,
		(
			SELECT EXISTS (
				SELECT 1 FROM core_blocks 
				WHERE chain_id = @chainId AND hash = @blockHash
				LIMIT 1
			)
		) AS block_found
	;`
	rows, err := app.pool.Query(c.Context(), sql, pgx.NamedArgs{
		"blockHash":   params.BlockHash,
		"blockNumber": params.BlockNumber,
		"chainId":     "audius-mainnet-alpha-beta",
	})
	if err != nil {
		return err
	}

	type BlockConfirmationResult struct {
		BlockFound  bool `db:"block_found" json:"block_found"`
		BlockPassed bool `db:"block_passed" json:"block_passed"`
	}
	res, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[BlockConfirmationResult])
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"data": res,
	})
}
