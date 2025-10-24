package dbc

import (
	"context"

	"api.audius.co/database"
	"api.audius.co/solana/spl/programs/meteora_locker"
	"github.com/gagliardetto/solana-go"
	"github.com/jackc/pgx/v5"
)

func upsertVestingEscrow(
	ctx context.Context,
	db database.DBTX,
	slot uint64,
	account solana.PublicKey,
	escrow *meteora_locker.VestingEscrow,
) error {
	sql := `
		INSERT INTO sol_locker_vesting_escrows (
			account,
			slot,
			recipient,
			token_mint,
			creator,
			base,
			escrow_bump,
			update_recipient_mode,
			cancel_mode,
			token_program_flag,
			cliff_time,
			frequency,
			cliff_unlock_amount,
			amount_per_period,
			number_of_period,
			total_claimed_amount,
			vesting_start_time,
			cancelled_at,
			created_at,
			updated_at
		) VALUES (
			@account,
			@slot,
			@recipient,
			@tokenMint,
			@creator,
			@base,
			@escrowBump,
			@updateRecipientMode,
			@cancelMode,
			@tokenProgramFlag,
			@cliffTime,
			@frequency,
			@cliffUnlockAmount,
			@amountPerPeriod,
			@numberOfPeriod,
			@totalClaimedAmount,
			@vestingStartTime,
			@cancelledAt,
			NOW(),
			NOW()
		)
		ON CONFLICT (account) DO UPDATE SET
			slot = EXCLUDED.slot,
			recipient = EXCLUDED.recipient,
			token_mint = EXCLUDED.token_mint,
			creator = EXCLUDED.creator,
			base = EXCLUDED.base,
			escrow_bump = EXCLUDED.escrow_bump,
			update_recipient_mode = EXCLUDED.update_recipient_mode,
			cancel_mode = EXCLUDED.cancel_mode,
			token_program_flag = EXCLUDED.token_program_flag,
			cliff_time = EXCLUDED.cliff_time,
			frequency = EXCLUDED.frequency,
			cliff_unlock_amount = EXCLUDED.cliff_unlock_amount,
			amount_per_period = EXCLUDED.amount_per_period,
			number_of_period = EXCLUDED.number_of_period,
			total_claimed_amount = EXCLUDED.total_claimed_amount,
			vesting_start_time = EXCLUDED.vesting_start_time,
			cancelled_at = EXCLUDED.cancelled_at,
			updated_at = NOW()
		WHERE sol_locker_vesting_escrows.slot > EXCLUDED.slot
	;`
	_, err := db.Exec(ctx, sql, pgx.NamedArgs{
		"account":             account.String(),
		"slot":                slot,
		"recipient":           escrow.Recipient.String(),
		"tokenMint":           escrow.TokenMint.String(),
		"creator":             escrow.Creator.String(),
		"base":                escrow.Base.String(),
		"escrowBump":          escrow.EscrowBump,
		"updateRecipientMode": int8(escrow.UpdateRecipientMode),
		"cancelMode":          int8(escrow.CancelMode),
		"tokenProgramFlag":    int8(escrow.TokenProgramFlag),
		"cliffTime":           escrow.CliffTime,
		"frequency":           escrow.Frequency,
		"cliffUnlockAmount":   escrow.CliffUnlockAmount,
		"amountPerPeriod":     escrow.AmountPerPeriod,
		"numberOfPeriod":      escrow.NumberOfPeriod,
		"totalClaimedAmount":  escrow.TotalClaimedAmount,
		"vestingStartTime":    escrow.VestingStartTime,
		"cancelledAt":         escrow.CancelledAt,
	})
	return err
}
