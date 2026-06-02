CREATE TABLE IF NOT EXISTS user_social_sets (
    user_id integer PRIMARY KEY,
    followees_bitmap bytea NOT NULL DEFAULT '\x'::bytea,
    followers_bitmap bytea NOT NULL DEFAULT '\x'::bytea,
    updated_at timestamp without time zone NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS user_social_set_dirty (
    user_id integer PRIMARY KEY,
    followees_dirty boolean NOT NULL DEFAULT false,
    followers_dirty boolean NOT NULL DEFAULT false,
    updated_at timestamp without time zone NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS user_social_set_dirty_updated_at_idx
    ON user_social_set_dirty (updated_at, user_id);

CREATE OR REPLACE FUNCTION mark_user_social_set_dirty() RETURNS trigger AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        INSERT INTO user_social_set_dirty (user_id, followees_dirty, followers_dirty, updated_at)
        VALUES (OLD.follower_user_id, true, false, CURRENT_TIMESTAMP)
        ON CONFLICT (user_id) DO UPDATE SET
            followees_dirty = user_social_set_dirty.followees_dirty OR EXCLUDED.followees_dirty,
            followers_dirty = user_social_set_dirty.followers_dirty OR EXCLUDED.followers_dirty,
            updated_at = CURRENT_TIMESTAMP;

        INSERT INTO user_social_set_dirty (user_id, followees_dirty, followers_dirty, updated_at)
        VALUES (OLD.followee_user_id, false, true, CURRENT_TIMESTAMP)
        ON CONFLICT (user_id) DO UPDATE SET
            followees_dirty = user_social_set_dirty.followees_dirty OR EXCLUDED.followees_dirty,
            followers_dirty = user_social_set_dirty.followers_dirty OR EXCLUDED.followers_dirty,
            updated_at = CURRENT_TIMESTAMP;

        RETURN OLD;
    END IF;

    INSERT INTO user_social_set_dirty (user_id, followees_dirty, followers_dirty, updated_at)
    VALUES (NEW.follower_user_id, true, false, CURRENT_TIMESTAMP)
    ON CONFLICT (user_id) DO UPDATE SET
        followees_dirty = user_social_set_dirty.followees_dirty OR EXCLUDED.followees_dirty,
        followers_dirty = user_social_set_dirty.followers_dirty OR EXCLUDED.followers_dirty,
        updated_at = CURRENT_TIMESTAMP;

    INSERT INTO user_social_set_dirty (user_id, followees_dirty, followers_dirty, updated_at)
    VALUES (NEW.followee_user_id, false, true, CURRENT_TIMESTAMP)
    ON CONFLICT (user_id) DO UPDATE SET
        followees_dirty = user_social_set_dirty.followees_dirty OR EXCLUDED.followees_dirty,
        followers_dirty = user_social_set_dirty.followers_dirty OR EXCLUDED.followers_dirty,
        updated_at = CURRENT_TIMESTAMP;

    IF TG_OP = 'UPDATE' THEN
        IF OLD.follower_user_id IS DISTINCT FROM NEW.follower_user_id THEN
            INSERT INTO user_social_set_dirty (user_id, followees_dirty, followers_dirty, updated_at)
            VALUES (OLD.follower_user_id, true, false, CURRENT_TIMESTAMP)
            ON CONFLICT (user_id) DO UPDATE SET
                followees_dirty = user_social_set_dirty.followees_dirty OR EXCLUDED.followees_dirty,
                followers_dirty = user_social_set_dirty.followers_dirty OR EXCLUDED.followers_dirty,
                updated_at = CURRENT_TIMESTAMP;
        END IF;

        IF OLD.followee_user_id IS DISTINCT FROM NEW.followee_user_id THEN
            INSERT INTO user_social_set_dirty (user_id, followees_dirty, followers_dirty, updated_at)
            VALUES (OLD.followee_user_id, false, true, CURRENT_TIMESTAMP)
            ON CONFLICT (user_id) DO UPDATE SET
                followees_dirty = user_social_set_dirty.followees_dirty OR EXCLUDED.followees_dirty,
                followers_dirty = user_social_set_dirty.followers_dirty OR EXCLUDED.followers_dirty,
                updated_at = CURRENT_TIMESTAMP;
        END IF;
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS mark_user_social_set_dirty ON follows;
CREATE TRIGGER mark_user_social_set_dirty
AFTER INSERT OR UPDATE OR DELETE ON follows
FOR EACH ROW EXECUTE FUNCTION mark_user_social_set_dirty();

INSERT INTO user_social_set_dirty (user_id, followees_dirty, followers_dirty, updated_at)
SELECT u.user_id, au.following_count > 0, au.follower_count > 0, CURRENT_TIMESTAMP
FROM users u
JOIN aggregate_user au USING (user_id)
WHERE u.is_current = TRUE
  AND (au.following_count > 0 OR au.follower_count > 0)
ON CONFLICT (user_id) DO UPDATE SET
    followees_dirty = user_social_set_dirty.followees_dirty OR EXCLUDED.followees_dirty,
    followers_dirty = user_social_set_dirty.followers_dirty OR EXCLUDED.followers_dirty,
    updated_at = CURRENT_TIMESTAMP;
