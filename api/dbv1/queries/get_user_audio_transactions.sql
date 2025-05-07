-- name: GetUserAudioTransactionsSortedByDate :many
WITH base AS (
SELECT ath.created_at, transaction_type, ath.signature, method, ath.user_bank, tx_metadata, change::text, balance::text
FROM users
JOIN user_bank_accounts uba ON uba.ethereum_address = users.wallet
JOIN audio_transactions_history ath ON ath.user_bank = uba.bank_account
WHERE users.user_id = @user_id::int AND users.is_current = TRUE
)
SELECT * FROM base
ORDER BY
  CASE WHEN @sort_direction = 'asc' THEN created_at END ASC,
  CASE WHEN @sort_direction = 'desc' THEN created_at END DESC
LIMIT @limit_val
OFFSET @offset_val;


-- name: GetUserAudioTransactionsSortedByType :many
WITH base AS (
SELECT ath.created_at, transaction_type, ath.signature, method, ath.user_bank, tx_metadata, change::text, balance::text
FROM users
JOIN user_bank_accounts uba ON uba.ethereum_address = users.wallet
JOIN audio_transactions_history ath ON ath.user_bank = uba.bank_account
WHERE users.user_id = @user_id::int AND users.is_current = TRUE
)
SELECT * FROM base
ORDER BY
  CASE WHEN @sort_direction = 'asc' THEN transaction_type END ASC,
  CASE WHEN @sort_direction = 'desc' THEN transaction_type END DESC
LIMIT @limit_val
OFFSET @offset_val;
