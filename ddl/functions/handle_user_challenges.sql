create or replace function handle_on_user_challenge() returns trigger as $$
declare
  cooldown_days integer;
  existing_notification integer;
  listen_streak_value integer;
begin
    if (new.is_complete = true) then
        -- attempt to insert a new notification, ignoring conflicts
        select challenges.cooldown_days into cooldown_days from challenges where id = new.challenge_id;

        if (cooldown_days is null or cooldown_days = 0) then
            -- Check if there is an existing notification with the same fields in the last 15 minutes

            if new.challenge_id not in ('tt', 'tp', 'tut') then
                insert into notification
                (blocknumber, user_ids, timestamp, type, group_id, specifier, data)
                values
                (
                    new.completed_blocknumber,
                    ARRAY [new.user_id],
                    new.completed_at,
                    'claimable_reward',
                    'claimable_reward:' || new.user_id || ':challenge:' || new.challenge_id || ':specifier:' || new.specifier,
                    new.specifier,
                    json_build_object('specifier', new.specifier, 'challenge_id', new.challenge_id, 'amount', new.amount)
                )
                on conflict do nothing;
            end if;

            if new.challenge_id = 'e' then
                select listen_streak into listen_streak_value
                from challenge_listen_streak
                where user_id = new.user_id
                limit 1;
            end if;

            insert into notification
            (blocknumber, user_ids, timestamp, type, group_id, specifier, data)
            values
            (
                new.completed_blocknumber,
                ARRAY [new.user_id],
                new.completed_at,
                'challenge_reward',
                'challenge_reward:' || new.user_id || ':challenge:' || new.challenge_id || ':specifier:' || new.specifier,
                new.user_id,
                case
                    when new.challenge_id = 'e' then
                        json_build_object(
                            'specifier', new.specifier,
                            'challenge_id', new.challenge_id,
                            'amount', new.amount::text || '00000000',
                            'listen_streak', coalesce(listen_streak_value, 0)
                        )
                    else
                        json_build_object(
                            'specifier', new.specifier,
                            'challenge_id', new.challenge_id,
                            'amount', new.amount::text || '00000000'
                        )
                end
            )
            on conflict do nothing;
        else
            -- transactional notifications cover this
            if (new.challenge_id != 'b' and new.challenge_id != 's') then
                select id into existing_notification
                from notification
                where
                type = 'reward_in_cooldown' and
                new.user_id = any(user_ids) and
                timestamp >= (new.completed_at - interval '1 hour')
                limit 1;

                if existing_notification is null then
                    insert into notification
                    (blocknumber, user_ids, timestamp, type, group_id, specifier, data)
                    values
                    (
                        new.completed_blocknumber,
                        ARRAY [new.user_id],
                        new.completed_at,
                        'reward_in_cooldown',
                        'reward_in_cooldown:' || new.user_id || ':challenge:' || new.challenge_id || ':specifier:' || new.specifier,
                        new.specifier,
                        json_build_object('specifier', new.specifier, 'challenge_id', new.challenge_id, 'amount', new.amount)
                    )
                    on conflict do nothing;
                end if;
            end if;
        end if;

        -- update user fast challenge count
        INSERT INTO user_score_features (user_id, challenge_count, updated_at)
        SELECT
            NEW.user_id,
            COUNT(*)::int AS challenge_count,
            now()
        FROM user_challenges uc
        JOIN users u
        ON u.user_id = uc.user_id
        WHERE uc.user_id = NEW.user_id
            AND uc.is_complete
            AND uc.challenge_id NOT IN ('m','b')
            AND uc.completed_at <= (u.created_at + interval '3 minutes')
        ON CONFLICT (user_id) DO UPDATE
            SET challenge_count = EXCLUDED.challenge_count,
                updated_at      = EXCLUDED.updated_at
        WHERE user_score_features.challenge_count IS DISTINCT FROM EXCLUDED.challenge_count;


    end if;

    return new;

end;
$$ language plpgsql;


do $$ begin
  create trigger on_user_challenge
    after insert or update on user_challenges
    for each row execute procedure handle_on_user_challenge();
exception
  when others then null;
end $$;
