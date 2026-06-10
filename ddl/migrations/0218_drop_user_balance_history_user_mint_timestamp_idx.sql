-- Production pg_stat_user_indexes showed this wide mint index at roughly 78 GB
-- with zero scans. The API balance-history endpoint filters by user_id and
-- timestamp only, which is covered by user_balance_history_user_timestamp_idx.

drop index concurrently if exists user_balance_history_user_mint_timestamp_idx;
