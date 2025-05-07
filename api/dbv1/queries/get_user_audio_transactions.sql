-- name: GetUserAudioTransactions :many
SELECT ath.created_at as transaction_date, transaction_type, ath.signature, method, ath.user_bank, tx_metadata as metadata, change::text, balance::text
FROM users
JOIN user_bank_accounts uba ON uba.ethereum_address = users.wallet
JOIN audio_transactions_history ath ON ath.user_bank = uba.bank_account
WHERE users.user_id = @user_id::int AND users.is_current = TRUE
ORDER BY
    CASE WHEN @sort_method::text = 'date' AND @sort_direction::text = 'asc' THEN ath.created_at END ASC,
    CASE WHEN @sort_method::text = 'date' AND @sort_direction::text = 'desc' THEN ath.created_at END DESC,
    CASE WHEN @sort_method::text = 'type' AND @sort_direction::text = 'asc' THEN transaction_type END ASC,
    CASE WHEN @sort_method::text = 'type' AND @sort_direction::text = 'desc' THEN transaction_type END DESC
LIMIT @limit_val
OFFSET @offset_val;

-- name: GetUserAudioTransactionsCount :one
SELECT count(*)
FROM users
JOIN user_bank_accounts uba ON uba.ethereum_address = users.wallet
JOIN audio_transactions_history ath ON ath.user_bank = uba.bank_account
WHERE users.user_id = @user_id::int AND users.is_current = TRUE;