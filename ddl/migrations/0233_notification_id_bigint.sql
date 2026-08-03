-- notification_id_seq exhausted its signed 32-bit range in production. Widen
-- both the owning column and the sequence so notification inserts can resume.
--
-- ALTER COLUMN integer -> bigint rewrites notification and requires an
-- ACCESS EXCLUSIVE lock. Fail quickly if the lock is not immediately
-- available so an unattended deploy cannot leave requests queued behind this
-- migration. Production should run this while notification readers and writers
-- are quiesced, then retry the migration.

BEGIN;

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '0';

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM pg_attribute
        WHERE attrelid = 'public.notification'::regclass
          AND attname = 'id'
          AND atttypid = 'integer'::regtype
          AND NOT attisdropped
    ) THEN
        LOCK TABLE public.notification IN ACCESS EXCLUSIVE MODE;

        ALTER TABLE public.notification
            ALTER COLUMN id TYPE bigint;
    END IF;

    IF EXISTS (
        SELECT 1
        FROM pg_sequences
        WHERE schemaname = 'public'
          AND sequencename = 'notification_id_seq'
          AND data_type = 'integer'::regtype
    ) THEN
        ALTER SEQUENCE public.notification_id_seq AS bigint;
    END IF;
END
$$;

COMMIT;
