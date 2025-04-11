-- name: GetUserConnectedWallets :many
SELECT chain, wallet
FROM associated_wallets 
WHERE is_current = true
	AND is_delete = false 
	AND user_id = @user_id;