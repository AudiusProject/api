-- identical to get_user_scores but for a single user
-- used to generate a user score for attestations and in UI tool
drop function if exists get_user_score(integer);
create or replace function get_user_score(target_user_id integer) returns table(
        -- order matters
        user_id integer,
        handle_lc text,
        play_count bigint,
        distinct_tracks_played bigint,
        challenge_count bigint,
        following_count bigint,
        follower_count bigint,
        chat_block_count bigint,
        is_audius_impersonator boolean,
        has_profile_picture boolean,
        karma bigint,
        score bigint
    ) language sql as $function$ with chat_blocks as (
        select c.blockee_user_id as user_id,
            count(*) as block_count
        from chat_blocked_users c
        where c.blockee_user_id = target_user_id
        group by c.blockee_user_id
    ),
    aggregate_scores as (
        select u.user_id,
            u.handle_lc,
            coalesce(udph.hours_with_play, 0)::bigint as play_count,
            coalesce(udpt.track_count, 0)::bigint as distinct_tracks_played,
            coalesce(usf.challenge_count, 0)::bigint as challenge_count,
            coalesce(au.following_count, 0) as following_count,
            coalesce(au.follower_count, 0) as follower_count,
            coalesce(cb.block_count, 0) as chat_block_count,
            case
                when (
                    u.handle_lc ilike '%audius%'
                    or lower(u.name) ilike '%audius%'
                )
                and u.is_verified = false then true
                else false
            end as is_audius_impersonator,
            (u.profile_picture_sizes is not null) as has_profile_picture,
            case
                when (
                    -- give max karma to users with more than 1000 followers
                    -- karma is too slow for users with many followers
                    au.follower_count > 1000
                ) then 100
                when (
                    au.follower_count = 0
                ) then 0
                else (
                    select LEAST(
                            (sum(fau.follower_count) / 100)::bigint,
                            100
                        )
                    from follows
                        join aggregate_user fau on follows.follower_user_id = fau.user_id
                    where follows.followee_user_id = target_user_id
                        and fau.following_count < 10000 -- ignore users with too many following
                        and follows.is_delete = false
                )
            end as karma
        from users u
            left join user_distinct_play_hours udph on u.user_id = udph.user_id
            left join user_distinct_play_tracks udpt on u.user_id = udpt.user_id
            left join user_score_features usf on u.user_id = usf.user_id
            left join chat_blocks cb on u.user_id = cb.user_id
            left join aggregate_user au on u.user_id = au.user_id
        where u.user_id = target_user_id
            and u.is_current
            and u.handle_lc is not null
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
