-- identical to get_user_score but for a user batch
-- used for updating score in aggregate_user
-- this score is used in shadowbanning
drop function if exists get_user_scores(integer[]);
create or replace function get_user_scores(
        target_user_ids integer [] default null::integer []
    ) returns table(
        user_id integer,
        handle_lc text,
        play_count bigint,
        distinct_tracks_played bigint,
        follower_count bigint,
        following_count bigint,
        challenge_count bigint,
        chat_block_count bigint,
        is_audius_impersonator boolean,
        has_profile_picture boolean,
        karma bigint,
        score bigint
    ) language sql as $function$ with chat_blocks as (
        select chat_blocked_users.blockee_user_id as user_id,
            count(*) as block_count
        from chat_blocked_users
            join users on chat_blocked_users.blockee_user_id = users.user_id
        where target_user_ids is null
            or chat_blocked_users.blockee_user_id = any(target_user_ids)
        group by chat_blocked_users.blockee_user_id
    ),
    aggregate_scores as (
        select users.user_id,
            users.handle_lc,
            coalesce(user_distinct_play_hours.hours_with_play, 0)::bigint as play_count,
            coalesce(user_distinct_play_tracks.track_count, 0)::bigint as distinct_tracks_played,
            coalesce(aggregate_user.following_count, 0) as following_count,
            coalesce(aggregate_user.follower_count, 0) as follower_count,
            coalesce(user_score_features.challenge_count, 0)::bigint as challenge_count,
            coalesce(chat_blocks.block_count, 0) as chat_block_count,
            case
                when (
                    users.handle_lc ilike '%audius%'
                    or lower(users.name) ilike '%audius%'
                )
                and users.is_verified = false then true
                else false
            end as is_audius_impersonator,
            (users.profile_picture_sizes is not null) as has_profile_picture,
            case
                when (
                    -- give max karma to users with more than 1000 followers
                    -- karma is too slow for users with many followers
                    aggregate_user.follower_count > 1000
                ) then 100
                when (
                    aggregate_user.follower_count = 0
                ) then 0
                else (
                    select LEAST(
                            (sum(fau.follower_count) / 100)::bigint,
                            100
                        )
                    from follows
                        join aggregate_user fau on follows.follower_user_id = fau.user_id
                    where follows.followee_user_id = users.user_id
                        and fau.following_count < 10000 -- ignore users with too many following
                        and follows.is_delete = false
                )
            end as karma
        from users
            left join user_distinct_play_hours on users.user_id = user_distinct_play_hours.user_id
            left join user_distinct_play_tracks on users.user_id = user_distinct_play_tracks.user_id
            left join user_score_features on users.user_id = user_score_features.user_id
            left join chat_blocks on users.user_id = chat_blocks.user_id
            left join aggregate_user on aggregate_user.user_id = users.user_id
        where users.handle_lc is not null
            and users.is_current
            and (
                target_user_ids is null
                or users.user_id = any(target_user_ids)
            )
    )
select a.*,
    compute_user_score(
        a.play_count,
        a.follower_count,
        a.challenge_count,
        a.chat_block_count,
        a.following_count,
        a.is_audius_impersonator,
        a.has_profile_picture,
        a.distinct_tracks_played,
        a.karma
    ) as score
from aggregate_scores a;
$function$;
