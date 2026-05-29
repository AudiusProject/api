--
-- PostgreSQL database dump
--


-- Dumped from database version 17.9 (Debian 17.9-1.pgdg13+1)
-- Dumped by pg_dump version 17.9 (Debian 17.9-1.pgdg13+1)

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET transaction_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Data for Name: schema_version; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.schema_version (file_name, md5, applied_at) FROM stdin;
utils/hashids.sql	a8038ea3d27ee416047807f1bc5b039e	2026-05-27 00:07:29.523763+00
migrations/0141_user_profile_type.sql	4f7151011ce295ed361cd8e3a0d105ee	2026-05-27 00:07:29.636571+00
migrations/0142_fix_missing_trending20250606.sql	b6e6df58c9f5208ddd4d4e762e0a598d	2026-05-27 00:07:29.718487+00
migrations/0143_add_user_total_tracks.sql	2aa111aa957bfb96283f9a137855d8c3	2026-05-27 00:07:29.788168+00
migrations/0143_follows_ix.sql	df958da384c2e656d3733956f70c6905	2026-05-27 00:07:29.861995+00
migrations/0144_follows_ix_delete_partial.sql	5231de073a8d78cb21bc9b17186836f4	2026-05-27 00:07:29.934416+00
migrations/0145_create_shares_table.sql	4d9bbbbb79051868b154a4a2ceb89c43	2026-05-27 00:07:30.008672+00
migrations/0146_request_metrics.sql	89904070b76dcc2cc16581a06bb1ec77	2026-05-27 00:07:30.083271+00
migrations/0147_add_new_solana_indexing_tables.sql	86d4147dcca9199d34c10f5c259ff045	2026-05-27 00:07:30.194298+00
migrations/0148_index_for_solana_balance_changes.sql	a838ca426a4a2a81ffaa263b175e4286	2026-05-27 00:07:30.268036+00
migrations/0149_token_account_balances.sql	18b5cf9c02e256a262e330f3d370e6c4	2026-05-27 00:22:30.117108+00
migrations/0150_trending_scores_idx.sql	34ea720c167d249fc525735a2da97463	2026-05-27 00:22:30.195664+00
migrations/0151_edit_sol_slot_checkpoints.sql	6f064516ac1e32ba52e3f4e8fa5b4800	2026-05-27 00:22:30.264399+00
migrations/0152_migrate_user_banks.sql	26c5e3bad2c7f59720f41c08dbb489cf	2026-05-27 00:22:30.343265+00
migrations/0153_fix_blocktimestamp_index.sql	0df7ce9f715f334b68937e1558e5a356	2026-05-27 00:22:30.41354+00
migrations/0154_add_sol_user_balances.sql	e467bcad082eb00ba1063dd95e0bdea8	2026-05-27 00:22:30.486313+00
migrations/0155_add_artist_coins_logo_description.sql	5cfe65a861cf6c5d28359f37c968b4f5	2026-05-27 00:22:30.559955+00
migrations/0156_sol_unprocessed_txs.sql	e6d5619d2cd04663b902d83211d52f02	2026-05-27 00:22:30.639308+00
migrations/0157_artist_coin_names.sql	45ceab6665d7de661223c763541ff41b	2026-05-27 00:22:30.715889+00
migrations/0158_artist_coin_dbc_pool.sql	d448ca1888495435d37764c14ad96925	2026-05-27 00:22:30.789269+00
migrations/0159_artist_coin_stats.sql	79d9e7c25107a94de6350b88f61585be	2026-05-27 00:22:30.862635+00
migrations/0160_artist_coin_pools.sql	1e19840c5ffbb4ecbbd70a8a56c859dd	2026-05-27 00:22:30.95123+00
migrations/0161_artist_coins_unique_ticker.sql	c8fb0f5383bce68cee587a42dfb90833	2026-05-27 00:22:31.041514+00
migrations/0162_artist_coins_discord.sql	f519976668bbbb15ae3d6c0b85031145	2026-05-27 00:22:31.13394+00
migrations/0163_remove_pool.sql	d63312450b509c5d516458618691664c	2026-05-27 00:22:31.202271+00
migrations/0164_agg_user_track_idx.sql	56de5ca7507745e5db96173b4271e213	2026-05-27 00:22:31.269828+00
migrations/0165_artist_coins_updated_at_and_social_links.sql	f20c202f78f355b88cc6fa46141d8ec4	2026-05-27 00:22:31.339441+00
migrations/0166_artist_coins_generic_links.sql	15c8c7a5e19b63aaf875cce396da8a1e	2026-05-27 00:22:31.406343+00
migrations/0167_add_artist_coin_pools_fee_columns.sql	2578215b8d7059196275e48e01d87969	2026-05-27 00:22:31.472489+00
migrations/0168_add_creator_wallet_address.sql	4452a8091ce8845d0bab001de8951e92	2026-05-27 00:22:31.560421+00
migrations/0168_sol_keypairs.sql	85f3507ed76dea23eee96e8ddb48952a	2026-05-27 00:22:31.678972+00
migrations/0169_add_artist_coin_all_time_stats.sql	8bfb5f6136bb4dfbd4436db08b745c79	2026-05-27 00:22:31.785746+00
migrations/0169_damm_and_positions.sql	aeed8da0d76045dad3aa9774105c1566	2026-05-27 00:22:31.934795+00
migrations/0169_hide_nft_gated_tracks.sql	2354d5e8ec1c3385004633c13a135f38	2026-05-27 00:22:32.029224+00
migrations/0170_sol_retry_queue.sql	5a02d7dabc93ebd3b61eda760a667bcf	2026-05-27 00:22:32.104892+00
migrations/0171_artist_coins_pools.sql	971d08be542f6bec5b2a9fab5b9c4b71	2026-05-27 00:22:32.17558+00
migrations/0172_recipient_eth_address_rewards.sql	62f0130e5a9b70e771869c807ca60096	2026-05-27 00:22:32.249843+00
migrations/0173_drop_dbc_pool_again.sql	cd191b3ab3dee523488c3d00c26247f8	2026-05-27 00:22:32.317968+00
migrations/0174_dbc_pool_substructs.sql	ba38cfbea9b36f8c20d43c87b1e7928f	2026-05-27 00:22:32.388727+00
migrations/0175_add_user_coin_badge_preference.sql	ee69bdfa766852a7a287ced664159b1f	2026-05-27 00:22:32.46057+00
migrations/0175_add_user_score_tables.sql	c2992c2a670979215f2dee23ce3859b1	2026-05-27 00:22:32.54083+00
migrations/0175_cleanup_old_tables.sql	a135cbcc4737e2706c14886f566694f8	2026-05-27 00:22:32.622739+00
migrations/0175_dbc_pool_configs.sql	0f0f5ccf8aa06ebadc5ff5c2d84feb10	2026-05-27 00:22:32.691448+00
migrations/0176_sol_reward_inits.sql	451adb897d3a136b86cb5e338c809996	2026-05-27 00:22:32.75726+00
migrations/0177_sol_locker_vesting_escrows.sql	9c27388639e851afd39edb1c09997175	2026-05-27 00:22:32.818406+00
migrations/0178_add_balance_change_fee_payer.sql	eed0de6f0357acbd8472548b177a0615	2026-05-27 00:22:32.880423+00
migrations/0178_user_balance_history.sql	aa1b401a0a12e919abaaf4d87b0d6524	2026-05-27 00:22:32.946071+00
migrations/0179_add_dvl_challenge.sql	aca7983db3e7ad242a80826b47ad5f94	2026-05-27 00:22:33.024753+00
migrations/0180_add_volume_leader_exclusions.sql	6da6c2d5c5780c3b87cf42524d070de7	2026-05-27 00:22:33.089005+00
migrations/0181_reward_codes.sql	42b5b17830c8fdffe416b749942e32ab	2026-05-27 00:22:33.152113+00
migrations/0182_reward_codes_is_used.sql	eb1cc82d8d488ee49d6d64b5ed8f1d49	2026-05-27 00:22:33.218806+00
migrations/0183_reward_codes_remaining_uses.sql	7f069d2fc2b41d576aeeedd4412c19f6	2026-05-27 00:22:33.292923+00
migrations/0184_custom_damm_v2_creations.sql	0cb35bda42d7fa168ecde0f69bc424b9	2026-05-27 00:22:33.368195+00
migrations/0185_add_signature_to_reward_codes.sql	23f689ec5fea7fcd88e3fe511916742d	2026-05-27 00:22:33.448033+00
migrations/0185_make_hll_sketch_nullable.sql	8ec35b6396741868098bb1b3245b4fb6	2026-05-27 00:22:33.537914+00
migrations/0186_artist_coins_banner_image.sql	42b8a0fc1d7bc208d6e6717bac9c70b0	2026-05-27 00:22:33.618967+00
migrations/0186_update_dvl_challenge_amount.sql	f31d18141c0a9eb23318ba3954693cd6	2026-05-27 00:22:33.69274+00
migrations/0187_prizes_and_claimed_prizes.sql	ac1f3879ef290c3cac25b5473a43ea20	2026-05-27 00:22:33.768718+00
migrations/0188_api_metrics_apps_unique.sql	99e97f4d246b4f0a6f6257da5e71baec	2026-05-27 00:22:33.844276+00
migrations/0189_api_keys_tables.sql	c1ed85e3011ef937d4a2869bcfa791c7	2026-05-27 00:22:33.924009+00
migrations/0190_access_authorities_tracks.sql	36fa21c25f497479cc1c4daacc69165c	2026-05-27 00:22:33.999091+00
migrations/0190_oauth_pkce.sql	e60bd2f22d471ef295b78a7eb1423cb2	2026-05-27 00:22:34.075122+00
migrations/0191_grants_grantee_address_idx.sql	9accd6a227613e05671efdd2fc2e8bd3	2026-05-27 00:22:34.15025+00
migrations/0192_notification_campaign_push_open.sql	8bb2f58732f9074e558cccac0563b4bd	2026-05-27 00:22:34.22819+00
migrations/0193_comments_is_members_only.sql	f31aeb856b4d0343a4ed6aa756d9b322	2026-05-27 00:22:34.306511+00
migrations/0194_add_comments_video_url.sql	2cec7f8da15992d9722a023e8a8e4cdf	2026-05-27 00:22:34.379084+00
migrations/0196_subscriptions_generic_entity.sql	79cf35ee210b861f7f61ad2e4580b3c1	2026-05-27 00:22:34.461522+00
migrations/0197_playlists_albums_partial_idx.sql	9b6cbad5bb81ef1e570473e1fa75f9ea	2026-05-27 00:22:34.541173+00
migrations/0198_sol_reward_disbursements_created_at.sql	3ed1fa0832788855e970d376d68ad97b	2026-05-27 00:22:34.615427+00
migrations/0198_track_trending_scores_for_you_idx.sql	e73ca8afd6b3ae45c10ce49c6c004da1	2026-05-27 00:22:34.688152+00
migrations/0199_backfill_sol_purchases.sql	77f8149a155db1709a89dcd46904354d	2026-05-27 00:22:34.766628+00
migrations/0200_user_payout_wallet_history_wallet_idx.sql	7de8e1bd6aa4b9ac3b3aa472ece3325e	2026-05-27 00:22:34.84024+00
migrations/0201_backfill_missing_reward_disbursements.sql	822965cac401417a5544f00e5e32a6e6	2026-05-27 00:22:34.91433+00
migrations/0202_tracks_isrc_normalized_idx.sql	e7ad196cd1b8dc78243e2a7155487266	2026-05-27 00:22:35.000438+00
migrations/0203_eth_wallet_balances.sql	d6d43a0c638ebdf0ff7f141f1466b163	2026-05-27 00:22:35.087858+00
functions/calculate_artist_coin_fee_earnings.sql	08de06d6cf1e4953acf6311ca94a1b5c	2026-05-27 00:22:35.194282+00
functions/calculate_artist_coin_locker.sql	d1b2d3b8736d58b87fcb875d97114093	2026-05-27 00:22:35.269564+00
functions/chat_allowed.sql	b00fe55b99ded6b8981c86874f638eb8	2026-05-27 00:22:35.349986+00
functions/chat_blast_audience.sql	3202f26a9bdf02f6d0e967e275aea7ee	2026-05-27 00:22:35.429591+00
functions/compute_user_score.sql	814b5fa3d1383d3e216943b4ff8c4877	2026-05-27 00:22:35.517899+00
functions/country_to_iso_alpha2.sql	218832f0607aeca4fce99815a07f7a85	2026-05-27 00:22:35.592939+00
functions/find_track.sql	f09431015118f31aa7b4ac49d14e7639	2026-05-27 00:22:35.667572+00
functions/get_user_score.sql	2d5e3003ae1074cbe22ee780dd0ff901	2026-05-27 00:22:35.742438+00
functions/get_user_scores.sql	b22e36599e6503649d7cabff0609bc05	2026-05-27 00:22:35.829695+00
functions/handle_artist_coins.sql	21ed1610d80cceca2262efb1338060ed	2026-05-27 00:22:35.905536+00
functions/handle_associated_wallet.sql	6c9299c3b14bc1db4a578f4ad46e2225	2026-05-27 00:22:35.99395+00
functions/handle_challenge_disbursements.sql	5177268be28586a350606b8cec13b294	2026-05-27 00:22:36.072508+00
functions/handle_chat_blast.sql	ccd276957c1b68bfc82fb5a11bac09ee	2026-05-27 00:22:36.155215+00
functions/handle_chat_message.sql	31bebee3d0437133f1f53c2eadb7b391	2026-05-27 00:22:36.229275+00
functions/handle_chat_message_reaction.sql	b898313aa8f31c61df7f1e6dd791ef31	2026-05-27 00:22:36.304728+00
functions/handle_comment.sql	619233e3cda476da5695d22ff995ace4	2026-05-27 00:22:36.384986+00
functions/handle_comment_remix_contest_update.sql	1f5209f511e03a3f4d8e01a0a300b678	2026-05-27 00:22:36.469378+00
functions/handle_comms_rpc_log.sql	bca8170b77a97b521b050f49873d43d4	2026-05-27 00:22:36.545119+00
functions/handle_dbc_pools.sql	5d8727fa203bb5f204868f4209a2022a	2026-05-27 00:22:36.618827+00
functions/handle_event.sql	85ff1a58bb6da94b20a9ba4335fdabc5	2026-05-27 00:22:36.697044+00
functions/handle_follow.sql	f552cb2453bb86161f40cc7a0b23b0aa	2026-05-27 00:22:36.771042+00
functions/handle_manager_request.sql	426004c1b9ac2be9e5721afb580679be	2026-05-27 00:22:36.848838+00
functions/handle_play.sql	c710d1ef4805d7f99817baffc6ed8e25	2026-05-27 00:22:36.924069+00
functions/handle_playlist.sql	4b338726d86db94fb3339e8407968cdf	2026-05-27 00:22:36.987976+00
functions/handle_playlist_track.sql	bbb4dce9244617aa2d6580aae05d154a	2026-05-27 00:22:37.057245+00
functions/handle_reaction.sql	679796675e687b45288c517c568692d4	2026-05-27 00:22:37.13432+00
functions/handle_repost.sql	fdb05891752cb029b097925ad2d5367e	2026-05-27 00:22:37.218515+00
functions/handle_save.sql	4893d5ca7730b055a3a20cb0eb546d97	2026-05-27 00:22:37.295378+00
functions/handle_share.sql	84efa350bfd7f5501ccd34a93f6207a6	2026-05-27 00:22:37.371581+00
functions/handle_sol_claimable_accounts.sql	de881daabbfb8eb5222e3ae10c6b72c7	2026-05-27 00:22:37.462649+00
functions/handle_sol_token_balance_change.sql	212ea3ca708570c506051a167e1e9117	2026-05-27 00:22:37.555359+00
functions/handle_supporter_rank_ups.sql	dea497f1859ade282b4f7d33687e6fe7	2026-05-27 00:22:37.633644+00
functions/handle_track.sql	89ef5a957b3662310fb30f914aab9ed4	2026-05-27 00:22:37.715524+00
functions/handle_usdc_purchase.sql	c35bccc2789ae0641d413d28a131ef8d	2026-05-27 00:22:37.800109+00
functions/handle_user.sql	169778112e5362ee20af56d9b8f274c5	2026-05-27 00:22:37.955619+00
functions/handle_user_balance_changes.sql	1ae7f99f4f37194cdf27dc3189ebc858	2026-05-27 00:22:38.02983+00
functions/handle_user_challenges.sql	c2adfee92cab36dee37ab7b8af5449c1	2026-05-27 00:22:38.103198+00
functions/handle_user_tip.sql	e4bde4e7e04b0ed8254690959513bd69	2026-05-27 00:22:38.167097+00
functions/is_country_eur.sql	c5641d570edb9cd47cd4e38d883e941a	2026-05-27 00:22:38.234372+00
functions/notify_on_row.sql	8b0232ed60eb108aa45f68c6dfe05d59	2026-05-27 00:22:38.304698+00
functions/notify_pending_purchase_revalidation.sql	beb9eebc6bd34ab069c7b90a51bb8bb3	2026-05-27 00:22:38.378429+00
functions/price_from_sqrt_price.sql	1b217f211adba88e3f12d1d6c81fc97d	2026-05-27 00:22:38.451965+00
functions/refresh_all_user_scores.sql	04935173f102e5e28b5c384e312102c1	2026-05-27 00:22:38.539364+00
functions/update_sol_user_balance.sql	4a297a671c74a683814ff217bd097589	2026-05-27 00:22:38.619283+00
functions/user_mint_balance_at.sql	6d227781d4d97500e0ec2bb76e8fa295	2026-05-27 00:22:38.701765+00
views/artist_coin_prices.sql	10a65b64b7d13aaabe18ad1055e9fd7b	2026-05-27 00:22:38.820919+00
views/v_challenge_disbursements.sql	74a0a05af6a02f82af3695d5c3ade45c	2026-05-27 00:22:38.899214+00
views/v_usdc_purchases.sql	322a527e132aed647c39b70d63aa6b51	2026-05-27 00:22:39.061667+00
preflight/0001_initial_block.sql	1e59cca66b2208eda67a87ac0d09b67d	2026-05-27 00:22:39.249328+00
migrations/0204_backfill_eth_wallet_balances_tracked.sql	d8b752794c42edb94f0d43633e14208c	2026-05-28 00:56:10.178339+00
views/v_user_balances.sql	ff6db739c1a77101021d4629647a44df	2026-05-28 17:28:07.629633+00
migrations/0205_sol_transfer_memo_tables.sql	49aa6bdf68df733710e7820153a78393	2026-05-28 19:54:42.181969+00
migrations/0206_backfill_sol_transfer_memo_types.sql	8b63b2e420c94b740a15e7f1fbd02ae7	2026-05-28 19:54:42.266989+00
functions/handle_usdc_withdrawal.sql	f590eb5d53ec82c63d3d2d7b1913cbef	2026-05-28 19:54:42.61986+00
views/v_token_transactions_history.sql	a00eeaf663c8b16dd60e26b7c75a8fff	2026-05-28 19:54:42.836012+00
migrations/0207_canonicalize_associated_wallets_eth.sql	83e88b1102c1bc5e2526a919e5266864	2026-05-28 20:11:26.200227+00
migrations/0203_seed_phase_1_challenges.sql	b027784464de897b26d4b420ca51a970	2026-05-29 16:22:36.535877+00
migrations/0204_seed_phase_2_challenges.sql	168a6d57c056e2e8f7fe14c36fc1c367	2026-05-29 16:22:36.811563+00
migrations/0205_seed_phase_3_challenges.sql	dc2a08647a63c0e355c6a3b2cc23a8bd	2026-05-29 16:22:37.200322+00
migrations/0208_seed_challenge_checkpoints.sql	ed11876806de4dd1d80b389894b4db45	2026-05-29 16:22:38.000000+00
migrations/0209_user_events_blocknumber_idx.sql	19ed339385266f28e83399125b6593df	2026-05-29 19:30:00.000000+00
migrations/0210_notification_cooldown_partial_gin.sql	7156a9b6e236e17acef7d6b91cb1291b	2026-05-29 19:30:00.100000+00
migrations/0211_seed_phase_3_user_event_checkpoints.sql	e37093a8ba1d1a4a3bcca7f98b612be2	2026-05-29 19:30:00.200000+00
\.


--
-- PostgreSQL database dump complete
--


