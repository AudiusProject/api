--
-- PostgreSQL database dump
--

-- Dumped from database version 17.5 (Debian 17.5-1.pgdg120+1)
-- Dumped by pg_dump version 17.4 (Debian 17.4-1.pgdg120+2)

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
-- Name: hashids; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA hashids;


--
-- Name: amcheck; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS amcheck WITH SCHEMA public;


--
-- Name: EXTENSION amcheck; Type: COMMENT; Schema: -; Owner: -
--

COMMENT ON EXTENSION amcheck IS 'functions for verifying relation integrity';


--
-- Name: pg_stat_statements; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS pg_stat_statements WITH SCHEMA public;


--
-- Name: EXTENSION pg_stat_statements; Type: COMMENT; Schema: -; Owner: -
--

COMMENT ON EXTENSION pg_stat_statements IS 'track planning and execution statistics of all SQL statements executed';


--
-- Name: pg_trgm; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS pg_trgm WITH SCHEMA public;


--
-- Name: EXTENSION pg_trgm; Type: COMMENT; Schema: -; Owner: -
--

COMMENT ON EXTENSION pg_trgm IS 'text similarity measurement and index searching based on trigrams';


--
-- Name: tsm_system_rows; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS tsm_system_rows WITH SCHEMA public;


--
-- Name: EXTENSION tsm_system_rows; Type: COMMENT; Schema: -; Owner: -
--

COMMENT ON EXTENSION tsm_system_rows IS 'TABLESAMPLE method which accepts number of rows as a limit';


--
-- Name: challengetype; Type: TYPE; Schema: public; Owner: -
--

CREATE TYPE public.challengetype AS ENUM (
    'boolean',
    'numeric',
    'aggregate',
    'trending'
);


--
-- Name: delist_entity; Type: TYPE; Schema: public; Owner: -
--

CREATE TYPE public.delist_entity AS ENUM (
    'TRACKS',
    'USERS'
);


--
-- Name: delist_track_reason; Type: TYPE; Schema: public; Owner: -
--

CREATE TYPE public.delist_track_reason AS ENUM (
    'DMCA',
    'ACR',
    'MANUAL',
    'ACR_COUNTER_NOTICE',
    'DMCA_RETRACTION',
    'DMCA_COUNTER_NOTICE',
    'DMCA_AND_ACR_COUNTER_NOTICE'
);


--
-- Name: delist_user_reason; Type: TYPE; Schema: public; Owner: -
--

CREATE TYPE public.delist_user_reason AS ENUM (
    'STRIKE_THRESHOLD',
    'COPYRIGHT_SCHOOL',
    'MANUAL'
);


--
-- Name: event_entity_type; Type: TYPE; Schema: public; Owner: -
--

CREATE TYPE public.event_entity_type AS ENUM (
    'track',
    'collection',
    'user'
);


--
-- Name: event_type; Type: TYPE; Schema: public; Owner: -
--

CREATE TYPE public.event_type AS ENUM (
    'remix_contest',
    'live_event',
    'new_release'
);


--
-- Name: parental_warning_type; Type: TYPE; Schema: public; Owner: -
--

CREATE TYPE public.parental_warning_type AS ENUM (
    'explicit',
    'explicit_content_edited',
    'not_explicit',
    'no_advice_available'
);


--
-- Name: profile_type_enum; Type: TYPE; Schema: public; Owner: -
--

CREATE TYPE public.profile_type_enum AS ENUM (
    'label'
);


--
-- Name: proof_status; Type: TYPE; Schema: public; Owner: -
--

CREATE TYPE public.proof_status AS ENUM (
    'unresolved',
    'pass',
    'fail'
);


--
-- Name: reposttype; Type: TYPE; Schema: public; Owner: -
--

CREATE TYPE public.reposttype AS ENUM (
    'track',
    'playlist',
    'album'
);


--
-- Name: savetype; Type: TYPE; Schema: public; Owner: -
--

CREATE TYPE public.savetype AS ENUM (
    'track',
    'playlist',
    'album'
);


--
-- Name: sharetype; Type: TYPE; Schema: public; Owner: -
--

CREATE TYPE public.sharetype AS ENUM (
    'track',
    'playlist'
);


--
-- Name: skippedtransactionlevel; Type: TYPE; Schema: public; Owner: -
--

CREATE TYPE public.skippedtransactionlevel AS ENUM (
    'node',
    'network'
);


--
-- Name: usdc_purchase_access_type; Type: TYPE; Schema: public; Owner: -
--

CREATE TYPE public.usdc_purchase_access_type AS ENUM (
    'stream',
    'download'
);


--
-- Name: usdc_purchase_content_type; Type: TYPE; Schema: public; Owner: -
--

CREATE TYPE public.usdc_purchase_content_type AS ENUM (
    'track',
    'playlist',
    'album'
);


--
-- Name: wallet_chain; Type: TYPE; Schema: public; Owner: -
--

CREATE TYPE public.wallet_chain AS ENUM (
    'eth',
    'sol'
);


--
-- Name: clean_alphabet_from_seps(text, text); Type: FUNCTION; Schema: hashids; Owner: -
--

CREATE FUNCTION hashids.clean_alphabet_from_seps(p_seps text, p_alphabet text) RETURNS text
    LANGUAGE plpgsql IMMUTABLE COST 200
    AS $_$
DECLARE
    p_seps ALIAS for $1;
    p_alphabet ALIAS for $2;
    v_split_seps text[] := regexp_split_to_array(p_seps, '');
    v_split_alphabet text[] := regexp_split_to_array(p_alphabet, '');
    v_i integer := 1;
    v_length integer := length(p_alphabet);
    v_ret text := '';
BEGIN
	-- had to add this function because doing this:
	-- p_alphabet := array_to_string(ARRAY( select chars.cha from (select unnest(regexp_split_to_array(p_alphabet, '')) as cha EXCEPT select unnest(regexp_split_to_array(p_seps, '')) as cha) as chars  ), '');
	-- doesn't preserve the order of the input

	for v_i in 1..v_length loop
		--raise notice 'v_split_alphabet[%]: % != %', v_i, v_split_alphabet[v_i], v_split_alphabet[v_i] <> all (v_split_seps);
		if (v_split_alphabet[v_i] <> all (v_split_seps)) then
			v_ret = v_ret || v_split_alphabet[v_i];
		end if;
	end loop;

	-- raise notice 'v_ret: %', v_ret;
	return v_ret;
END;
$_$;


--
-- Name: clean_seps_from_alphabet(text, text); Type: FUNCTION; Schema: hashids; Owner: -
--

CREATE FUNCTION hashids.clean_seps_from_alphabet(p_seps text, p_alphabet text) RETURNS text
    LANGUAGE plpgsql IMMUTABLE COST 200
    AS $_$
DECLARE
    p_seps ALIAS for $1;
    p_alphabet ALIAS for $2;
    v_split_seps text[] := regexp_split_to_array(p_seps, '');
    v_split_alphabet text[] := regexp_split_to_array(p_alphabet, '');
    v_i integer := 1;
    v_length integer := length(p_seps);
    v_ret text := '';
BEGIN
	-- had to add this function because doing this:
	-- p_seps := array_to_string(ARRAY(select chars.cha from (select unnest(regexp_split_to_array(p_seps, '')) as cha intersect select unnest(regexp_split_to_array(p_alphabet, '')) as cha ) as chars order by ascii(cha) desc), '');
	-- doesn't preserve the order of the input

	for v_i in 1..v_length loop
		-- raise notice 'v_split_seps[%]: %  == %', v_i, v_split_seps[v_i], v_split_seps[v_i] = any (v_split_alphabet);
		if (v_split_seps[v_i] = any (v_split_alphabet)) then
			v_ret = v_ret || v_split_seps[v_i];
		end if;
	end loop;

	-- raise notice 'v_ret: %', v_ret;
	return v_ret;
END;
$_$;


--
-- Name: consistent_shuffle(text, text); Type: FUNCTION; Schema: hashids; Owner: -
--

CREATE FUNCTION hashids.consistent_shuffle(p_alphabet text, p_salt text) RETURNS text
    LANGUAGE plpgsql IMMUTABLE COST 200
    AS $_$
DECLARE p_alphabet ALIAS FOR $1;
	p_salt ALIAS FOR $2;
	v_ls int;
	v_i int;
	v_v int := 0;
	v_p int := 0;
	v_n int := 0;
	v_j int := 0;
	v_temp char(1);
BEGIN

	-- Null or Whitespace?
	IF p_salt IS NULL OR length(LTRIM(RTRIM(p_salt))) = 0 THEN
		RETURN p_alphabet;
	END IF;

	v_ls := length(p_salt);
	v_i := length(p_alphabet) - 1;

	WHILE v_i > 0 LOOP

		v_v := v_v % v_ls;
		v_n := ascii(SUBSTRING(p_salt, v_v + 1, 1)); -- need some investigation to see if +1 here is because of 1 based arrays in sql ... this isn't in the reference JS or .net code.
		v_p := v_p + v_n;
		v_j := (v_n + v_v + v_p) % v_i;
		v_temp := SUBSTRING(p_alphabet, v_j + 1, 1);
		p_alphabet :=
				SUBSTRING(p_alphabet, 1, v_j) ||
				SUBSTRING(p_alphabet, v_i + 1, 1) ||
				SUBSTRING(p_alphabet, v_j + 2, 255);
		p_alphabet :=  SUBSTRING(p_alphabet, 1, v_i) || v_temp || SUBSTRING(p_alphabet, v_i + 2, 255);
		v_i := v_i - 1;
		v_v := v_v + 1;

	END LOOP; -- WHILE

	RETURN p_alphabet;

END;
$_$;


--
-- Name: decode(text); Type: FUNCTION; Schema: hashids; Owner: -
--

CREATE FUNCTION hashids.decode(p_hash text) RETURNS bigint[]
    LANGUAGE plpgsql IMMUTABLE COST 300
    AS $_$
    DECLARE
        p_numbers ALIAS for $1;
        p_salt text := ''; -- default
        p_min_hash_length integer := 0; -- default
        p_alphabet text := 'abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890'; -- default
        p_zero_offset boolean := true ; -- adding an offset so that this can work with values from a zero based array language
BEGIN
    RETURN hashids.decode(p_hash, p_salt, p_min_hash_length, p_alphabet, p_zero_offset);
END;
$_$;


--
-- Name: decode(text, text); Type: FUNCTION; Schema: hashids; Owner: -
--

CREATE FUNCTION hashids.decode(p_hash text, p_salt text) RETURNS text
    LANGUAGE plpgsql IMMUTABLE COST 300
    AS $_$
    DECLARE
        p_numbers ALIAS for $1;
        p_salt ALIAS for $2; -- default
        p_min_hash_length integer := 0; -- default
        p_alphabet text := 'abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890'; -- default
        p_zero_offset boolean := true ; -- adding an offset so that this can work with values from a zero based array language
BEGIN
    RETURN hashids.decode(p_hash, p_salt, p_min_hash_length, p_alphabet, p_zero_offset);
END;
$_$;


--
-- Name: decode(text, text, integer); Type: FUNCTION; Schema: hashids; Owner: -
--

CREATE FUNCTION hashids.decode(p_hash text, p_salt text, p_min_hash_length integer) RETURNS bigint[]
    LANGUAGE plpgsql IMMUTABLE COST 300
    AS $_$
    DECLARE
        p_numbers ALIAS for $1;
        p_salt ALIAS for $2; -- default
        p_min_hash_length ALIAS for $3; -- default
        p_alphabet text := 'abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890'; -- default
        p_zero_offset boolean := true ; -- adding an offset so that this can work with values from a zero based array language
BEGIN
    RETURN hashids.decode(p_hash, p_salt, p_min_hash_length, p_alphabet, p_zero_offset);
END;
$_$;


--
-- Name: decode(text, text, integer, text); Type: FUNCTION; Schema: hashids; Owner: -
--

CREATE FUNCTION hashids.decode(p_hash text, p_salt text, p_min_hash_length integer, p_alphabet text) RETURNS bigint[]
    LANGUAGE plpgsql IMMUTABLE COST 300
    AS $_$
    DECLARE
        p_numbers ALIAS for $1;
        p_salt ALIAS for $2; -- default
        p_min_hash_length ALIAS for $3; -- default
        p_alphabet ALIAS for $4; -- default
        p_zero_offset boolean := true ; -- adding an offset so that this can work with values from a zero based array language
BEGIN
    RETURN hashids.decode(p_hash, p_salt, p_min_hash_length, p_alphabet, p_zero_offset);
END;
$_$;


--
-- Name: decode(text, text, integer, text, boolean); Type: FUNCTION; Schema: hashids; Owner: -
--

CREATE FUNCTION hashids.decode(p_hash text, p_salt text, p_min_hash_length integer, p_alphabet text, p_zero_offset boolean DEFAULT true) RETURNS bigint[]
    LANGUAGE plpgsql IMMUTABLE COST 300
    AS $_$
DECLARE
    p_hash ALIAS for $1;
    p_salt ALIAS for $2;
    p_min_hash_length ALIAS for $3;
    p_alphabet ALIAS for $4;
    p_zero_offset ALIAS for $5; -- adding an offset so that this can work with values from a zero based array language

    v_seps text;
    v_guards text;
    v_alphabet text := p_alphabet;
    v_lottery char(1);

    v_hashBreakdown varchar(255);
    v_hashArray text[];
    v_index integer := 1;
    v_j integer := 1;
    v_hashArrayLength integer;
    v_subHash varchar;
    v_buffer varchar(255);
    v_encodeCheck varchar(255);
    v_ret_temp bigint;
    v_ret bigint[];
BEGIN

    select * from hashids.setup_alphabet(p_salt, v_alphabet) into v_alphabet, v_seps, v_guards;
    --raise notice 'v_seps: %', v_seps;
    --raise notice 'v_alphabet: %', v_alphabet;
    --raise notice 'v_guards: %', v_guards;

    v_hashBreakdown := regexp_replace(p_hash, '[' || v_guards || ']', ' ');
    v_hashArray := regexp_split_to_array(p_hash, '[' || v_guards || ']');

    -- take the guards and replace with space,
    -- split on space
    -- if length is 3 or 2, set index to 1 else start at zero

    -- if first index in idBreakDown isn't default

    if ((array_length(v_hashArray, 1) = 3) or (array_length(v_hashArray, 1) = 2)) then
        v_index := 2; -- in the example code (C# and js) it is 1 here, but postgresql arrays start at 1, so switching to 2
    END IF;
    --raise notice '%', v_hashArray;

    v_hashBreakdown := v_hashArray[v_index];
    --raise notice 'v_hashArray[%] %', v_index, v_hashBreakdown;
    if (left(v_hashBreakdown, 1) <> '') IS NOT false then
        v_lottery := left(v_hashBreakdown, 1);
        --raise notice 'v_lottery %', v_lottery;
        --raise notice 'SUBSTRING(%, 2, % - 1) %', v_hashBreakdown, length(v_hashBreakdown), SUBSTRING(v_hashBreakdown, 2);

        v_hashBreakdown := SUBSTRING(v_hashBreakdown, 2);
        v_hashArray := regexp_split_to_array(v_hashBreakdown, '[' || v_seps || ']');
        --raise notice 'v_hashArray % -- %', v_hashArray, array_length(v_hashArray, 1);
        v_hashArrayLength := array_length(v_hashArray, 1);
        for v_j in 1..v_hashArrayLength LOOP
            v_subHash := v_hashArray[v_j];
            --raise notice 'v_subHash %', v_subHash;
            v_buffer := v_lottery || p_salt || v_alphabet;
            --raise notice 'v_buffer %', v_buffer;
            --raise notice 'v_alphabet: hashids.consistent_shuffle(%, %) == %', v_alphabet, SUBSTRING(v_buffer, 1, length(v_alphabet)), hashids.consistent_shuffle(v_alphabet, SUBSTRING(v_buffer, 1, length(v_alphabet)));
            v_alphabet := hashids.consistent_shuffle(v_alphabet, SUBSTRING(v_buffer, 1, length(v_alphabet)));
            v_ret_temp := hashids.unhash(v_subHash, v_alphabet, p_zero_offset);
            --raise notice 'v_ret_temp: %', v_ret_temp;
            v_ret := array_append(v_ret, v_ret_temp);
        END LOOP;
        v_encodeCheck := hashids.encode_list(v_ret, p_salt, p_min_hash_length, p_alphabet, p_zero_offset);
        IF (v_encodeCheck <> p_hash) then
            raise notice 'hashids.encodeList(%): % <> %', v_ret, v_encodeCheck, p_hash;
            return ARRAY[]::bigint[];
        end if;
    end if;

    RETURN v_ret;
END;
$_$;


--
-- Name: distinct_alphabet(text); Type: FUNCTION; Schema: hashids; Owner: -
--

CREATE FUNCTION hashids.distinct_alphabet(p_alphabet text) RETURNS text
    LANGUAGE plpgsql IMMUTABLE COST 200
    AS $_$
DECLARE
    p_alphabet ALIAS for $1;
    v_split_alphabet text[] := regexp_split_to_array(p_alphabet, '');
    v_i integer := 2;
    v_length integer := length(p_alphabet);
    v_ret_array text[];
BEGIN
	-- had to add this function because doing this:
	-- p_alphabet := string_agg(distinct chars.split_chars, '') from (select unnest(regexp_split_to_array(p_alphabet, '')) as split_chars) as chars;
	-- doesn't preserve the order of the input, which was causing issues
	if (v_length = 0) then
		RAISE EXCEPTION 'alphabet must contain at least 1 char' USING HINT = 'Please check your alphabet';
	end if;
	v_ret_array := array_append(v_ret_array, v_split_alphabet[1]);

	-- starting at 2 because already appended 1 to it.
	for v_i in 2..v_length loop
		-- raise notice 'v_split_alphabet[%]: % != %', v_i, v_split_alphabet[v_i], v_split_alphabet[v_i] <> all (v_ret_array);

		if (v_split_alphabet[v_i] <> all (v_ret_array)) then
			v_ret_array := array_append(v_ret_array, v_split_alphabet[v_i]);
		end if;
	end loop;

	-- raise notice 'v_ret_array: %', array_to_string(v_ret_array, '');
	return array_to_string(v_ret_array, '');
END;
$_$;


--
-- Name: encode(bigint); Type: FUNCTION; Schema: hashids; Owner: -
--

CREATE FUNCTION hashids.encode(p_number bigint) RETURNS text
    LANGUAGE plpgsql IMMUTABLE COST 300
    AS $_$
DECLARE
    p_number ALIAS for $1;
    p_salt text := ''; -- default
    p_min_hash_length integer := 0; -- default
    p_alphabet text := 'abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890'; -- default
    p_zero_offset boolean := true ; -- adding an offset so that this can work with values from a zero based array language
BEGIN
    RETURN hashids.encode_list(ARRAY[p_number::bigint]::bigint[], p_salt, p_min_hash_length, p_alphabet, p_zero_offset);
END;
$_$;


--
-- Name: encode(bigint, text); Type: FUNCTION; Schema: hashids; Owner: -
--

CREATE FUNCTION hashids.encode(p_number bigint, p_salt text) RETURNS text
    LANGUAGE plpgsql IMMUTABLE COST 300
    AS $_$
DECLARE
    p_number ALIAS for $1;
    p_salt ALIAS for $2;
    p_min_hash_length integer := 0; -- default
    p_alphabet text := 'abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890'; -- default
    p_zero_offset boolean := true ; -- adding an offset so that this can work with values from a zero based array language
BEGIN
    RETURN hashids.encode_list(ARRAY[p_number::bigint]::bigint[], p_salt, p_min_hash_length, p_alphabet, p_zero_offset);
END;
$_$;


--
-- Name: encode(bigint, text, integer); Type: FUNCTION; Schema: hashids; Owner: -
--

CREATE FUNCTION hashids.encode(p_number bigint, p_salt text, p_min_hash_length integer) RETURNS text
    LANGUAGE plpgsql IMMUTABLE COST 300
    AS $_$
DECLARE
    p_number ALIAS for $1;
    p_salt ALIAS for $2;
    p_min_hash_length ALIAS for $3; -- default
    p_alphabet text := 'abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890'; -- default
    p_zero_offset boolean := true ; -- adding an offset so that this can work with values from a zero based array language
BEGIN
    RETURN hashids.encode_list(ARRAY[p_number::bigint]::bigint[], p_salt, p_min_hash_length, p_alphabet, p_zero_offset);
END;
$_$;


--
-- Name: encode(bigint, text, integer, text); Type: FUNCTION; Schema: hashids; Owner: -
--

CREATE FUNCTION hashids.encode(p_number bigint, p_salt text, p_min_hash_length integer, p_alphabet text) RETURNS text
    LANGUAGE plpgsql IMMUTABLE COST 300
    AS $_$
DECLARE
    p_number ALIAS for $1;
    p_salt ALIAS for $2;
    p_min_hash_length ALIAS for $3; -- default
    p_alphabet ALIAS for $4; -- default
    p_zero_offset boolean := true ; -- adding an offset so that this can work with values from a zero based array language
BEGIN
    RETURN hashids.encode_list(ARRAY[p_number::bigint]::bigint[], p_salt, p_min_hash_length, p_alphabet, p_zero_offset);
END;
$_$;


--
-- Name: encode(bigint, text, integer, text, boolean); Type: FUNCTION; Schema: hashids; Owner: -
--

CREATE FUNCTION hashids.encode(p_number bigint, p_salt text, p_min_hash_length integer, p_alphabet text, p_zero_offset boolean) RETURNS text
    LANGUAGE plpgsql IMMUTABLE COST 300
    AS $_$
DECLARE
    p_number ALIAS for $1;
    p_salt ALIAS for $2;
    p_min_hash_length ALIAS for $3; -- default
    p_alphabet ALIAS for $4; -- default
    p_zero_offset ALIAS for $5 ; -- adding an offset so that this can work with values from a zero based array language
BEGIN
    RETURN hashids.encode_list(ARRAY[p_number::bigint]::bigint[], p_salt, p_min_hash_length, p_alphabet, p_zero_offset);
END;
$_$;


--
-- Name: encode_list(bigint[]); Type: FUNCTION; Schema: hashids; Owner: -
--

CREATE FUNCTION hashids.encode_list(p_numbers bigint[]) RETURNS text
    LANGUAGE plpgsql IMMUTABLE COST 300
    AS $_$
-- Options Data - generated by hashids-tsql
    DECLARE
        p_numbers ALIAS for $1;
        p_salt text := ''; -- default
        p_min_hash_length integer := 0; -- default
        p_alphabet text := 'abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890'; -- default
        p_zero_offset boolean := true ; -- adding an offset so that this can work with values from a zero based array language
BEGIN
    RETURN hashids.encode_list(p_numbers, p_salt, p_min_hash_length, p_alphabet, p_zero_offset);
END;
$_$;


--
-- Name: encode_list(bigint[], text); Type: FUNCTION; Schema: hashids; Owner: -
--

CREATE FUNCTION hashids.encode_list(p_numbers bigint[], p_salt text) RETURNS text
    LANGUAGE plpgsql IMMUTABLE COST 300
    AS $_$
-- Options Data - generated by hashids-tsql
    DECLARE
        p_numbers ALIAS for $1;
        p_salt ALIAS for $2; -- default
        p_min_hash_length integer := 0; -- default
        p_alphabet text := 'abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890'; -- default
        p_zero_offset boolean := true ; -- adding an offset so that this can work with values from a zero based array language
BEGIN
    RETURN hashids.encode_list(p_numbers, p_salt, p_min_hash_length, p_alphabet, p_zero_offset);
END;
$_$;


--
-- Name: encode_list(bigint[], text, integer); Type: FUNCTION; Schema: hashids; Owner: -
--

CREATE FUNCTION hashids.encode_list(p_numbers bigint[], p_salt text, p_min_hash_length integer) RETURNS text
    LANGUAGE plpgsql IMMUTABLE COST 300
    AS $_$
-- Options Data - generated by hashids-tsql
    DECLARE
        p_numbers ALIAS for $1;
        p_salt ALIAS for $2; -- default
        p_min_hash_length ALIAS for $3; -- default
        p_alphabet text := 'abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890'; -- default
        p_zero_offset boolean := true ; -- adding an offset so that this can work with values from a zero based array language
BEGIN
    RETURN hashids.encode_list(p_numbers, p_salt, p_min_hash_length, p_alphabet, p_zero_offset);
END;
$_$;


--
-- Name: encode_list(bigint[], text, integer, text, boolean); Type: FUNCTION; Schema: hashids; Owner: -
--

CREATE FUNCTION hashids.encode_list(p_numbers bigint[], p_salt text, p_min_hash_length integer, p_alphabet text, p_zero_offset boolean DEFAULT true) RETURNS text
    LANGUAGE plpgsql IMMUTABLE COST 350
    AS $_$
    DECLARE
        p_numbers ALIAS for $1;
        p_salt ALIAS for $2;
        p_min_hash_length ALIAS for $3;
        p_alphabet ALIAS for $4;
        p_zero_offset integer := case when $5 = true then 1 else 0 end ; -- adding an offset so that this can work with values from a zero based array language
        v_seps text;
        v_guards text;

        -- Working Data
        v_alphabet text := p_alphabet;
        v_numbersHashInt int = 0;
        v_lottery char(1);
        v_buffer varchar(255);
        v_last varchar(255);
        v_ret varchar(255);
        v_sepsIndex int;
        v_lastId int;
        v_count int = array_length(p_numbers, 1);
        v_i int = 0;
        v_id int = 0;
        v_number int;
        v_guardIndex int;
        v_guard char(1);
        v_halfLength int;
        v_excess int;
BEGIN

    select * from hashids.setup_alphabet(p_salt, p_alphabet) into v_alphabet, v_seps, v_guards;
    --raise notice 'v_seps: %', v_seps;
    --raise notice 'v_alphabet: %', v_alphabet;
    --raise notice 'v_guards: %', v_guards;

    -- Calculate numbersHashInt
    for v_lastId in 1..v_count LOOP
        v_numbersHashInt := v_numbersHashInt + (p_numbers[v_lastId] % ((v_lastId-p_zero_offset) + 100));
    END LOOP;

    -- Choose lottery
    v_lottery := SUBSTRING(v_alphabet, (v_numbersHashInt % length(v_alphabet)) + 1, 1); -- is this a +1 because of sql 1 based index, need to double check to see if can be replaced with param.
    v_ret := v_lottery;

    -- Encode many
    v_i := 0;
    v_id := 0;
    for v_i in 1..v_count LOOP
        v_number := p_numbers[v_i];
        -- raise notice '%[%]: % for %', p_numbers, v_i, v_number, v_count;

        v_buffer := v_lottery || p_salt || v_alphabet;
        v_alphabet := hashids.consistent_shuffle(v_alphabet, SUBSTRING(v_buffer, 1, length(v_alphabet)));
        v_last := hashids.hash(v_number, v_alphabet, cast(p_zero_offset as boolean));
        v_ret := v_ret || v_last;
        --raise notice 'v_ret: %', v_ret;
        --raise notice '(v_i < v_count: % < % == %', v_i, v_count, (v_i < v_count);
        IF (v_i) < v_count THEN
            --raise notice 'v_sepsIndex:  % mod (% + %) == %', v_number, ascii(SUBSTRING(v_last, 1, 1)), v_i, (v_number % (ascii(SUBSTRING(v_last, 1, 1)) + v_i));
            v_sepsIndex := v_number % (ascii(SUBSTRING(v_last, 1, 1)) + (v_i-p_zero_offset)); -- since this is 1 base vs 0 based bringing the number back down so that the mod is the same for zero based records
            v_sepsIndex := v_sepsIndex % length(v_seps);
            v_ret := v_ret || SUBSTRING(v_seps, v_sepsIndex+1, 1);
        END IF;

    END LOOP;

    ----------------------------------------------------------------------------
    -- Enforce minHashLength
    ----------------------------------------------------------------------------
    IF length(v_ret) < p_min_hash_length THEN

        ------------------------------------------------------------------------
        -- Add first 2 guard characters
        ------------------------------------------------------------------------
        v_guardIndex := (v_numbersHashInt + ascii(SUBSTRING(v_ret, 1, 1))) % length(v_guards);
        v_guard := SUBSTRING(v_guards, v_guardIndex + 1, 1);
        --raise notice '% || % is %', v_guard, v_ret, v_guard || v_ret;
        v_ret := v_guard || v_ret;
        IF length(v_ret) < p_min_hash_length THEN
            v_guardIndex := (v_numbersHashInt + ascii(SUBSTRING(v_ret, 3, 1))) % length(v_guards);
            v_guard := SUBSTRING(v_guards, v_guardIndex + 1, 1);
            v_ret := v_ret || v_guard;
        END IF;
        ------------------------------------------------------------------------
        -- Add the rest
        ------------------------------------------------------------------------
        WHILE length(v_ret) < p_min_hash_length LOOP
            v_halfLength := COALESCE(v_halfLength, CAST((length(v_alphabet) / 2) as int));
            v_alphabet := hashids.consistent_shuffle(v_alphabet, v_alphabet);
            v_ret := SUBSTRING(v_alphabet, v_halfLength + 1, 255) || v_ret || SUBSTRING(v_alphabet, 1, v_halfLength);
            v_excess := length(v_ret) - p_min_hash_length;
            IF v_excess > 0 THEN
                v_ret := SUBSTRING(v_ret, CAST((v_excess / 2) as int) + 1, p_min_hash_length);
            END IF;
        END LOOP;
    END IF;
    RETURN v_ret;
END;
$_$;


--
-- Name: hash(bigint, text, boolean); Type: FUNCTION; Schema: hashids; Owner: -
--

CREATE FUNCTION hashids.hash(p_input bigint, p_alphabet text, p_zero_offset boolean DEFAULT true) RETURNS text
    LANGUAGE plpgsql IMMUTABLE COST 250
    AS $_$
DECLARE
    p_input ALIAS for $1;
    p_alphabet ALIAS for $2;
    p_zero_offset integer := case when $3 = true then 1 else 0 end ; -- adding an offset so that this can work with values from a zero based array language
    v_hash varchar(255) := '';
    v_alphabet_length integer := length($2);
    v_pos integer;
BEGIN

    WHILE 1 = 1 LOOP
        v_pos := (p_input % v_alphabet_length) + p_zero_offset; -- have to add one, because SUBSTRING in SQL starts at 1 instead of 0 (like it does in other languages)
        --raise notice '% mod % == %', p_input, v_alphabet_length, v_pos;
        --raise notice 'SUBSTRING(%, %, 1): %', p_alphabet, v_pos, (SUBSTRING(p_alphabet, v_pos, 1));
        --raise notice '% || % == %', SUBSTRING(p_alphabet, v_pos, 1), v_hash, SUBSTRING(p_alphabet, v_pos, 1) || v_hash;
        v_hash := SUBSTRING(p_alphabet, v_pos, 1) || v_hash;
        p_input := CAST((p_input / v_alphabet_length) as int);
        --raise notice 'p_input %', p_input;
        IF p_input <= 0 THEN
            EXIT;
        END IF;
    END LOOP;

    RETURN v_hash;
END;
$_$;


--
-- Name: setup_alphabet(text, text); Type: FUNCTION; Schema: hashids; Owner: -
--

CREATE FUNCTION hashids.setup_alphabet(p_salt text DEFAULT ''::text, INOUT p_alphabet text DEFAULT 'abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890'::text, OUT p_seps text, OUT p_guards text) RETURNS record
    LANGUAGE plpgsql IMMUTABLE COST 200
    AS $_$
DECLARE
    p_salt ALIAS for $1;
    p_alphabet ALIAS for $2;
    p_seps ALIAS for $3;
    p_guards ALIAS for $4;
    v_sep_div float := 3.5;
    v_guard_div float := 12.0;
    v_guard_count integer;
    v_seps_length integer;
    v_seps_diff integer;
BEGIN
    p_seps := 'cfhistuCFHISTU';
    -- p_alphabet := string_agg(distinct chars.split_chars, '') from (select unnest(regexp_split_to_array(p_alphabet, '')) as split_chars) as chars;
		-- this also doesn't preserve the order of alphabet, but it doesn't appear to matter, never mind on that
		p_alphabet := hashids.distinct_alphabet(p_alphabet);


    if length(p_alphabet) < 16 then
        RAISE EXCEPTION 'alphabet must contain 16 unique characters, it is: %', length(p_alphabet) USING HINT = 'Please check your alphabet';
    end if;

    -- seps should only contain character present in the passed alphabet
    -- p_seps := array_to_string(ARRAY(select chars.cha from (select unnest(regexp_split_to_array(p_seps, '')) as cha intersect select unnest(regexp_split_to_array(p_alphabet, '')) as cha ) as chars order by ascii(cha) desc), '');
    -- this doesn't preserve the input order, which is bad
    p_seps := hashids.clean_seps_from_alphabet(p_seps, p_alphabet);

    -- alphabet should not contain seps.
    -- p_alphabet := array_to_string(ARRAY( select chars.cha from (select unnest(regexp_split_to_array(p_alphabet, '')) as cha EXCEPT select unnest(regexp_split_to_array(p_seps, '')) as cha) as chars  ), '');
    -- this also doesn't prevserve the order
    p_alphabet := hashids.clean_alphabet_from_seps(p_seps, p_alphabet);


	p_seps := hashids.consistent_shuffle(p_seps, p_salt);

	if (length(p_seps) = 0) or ((length(p_alphabet) / length(p_seps)) > v_sep_div) then
		v_seps_length := cast( ceil( length(p_alphabet)/v_sep_div ) as integer);
		if v_seps_length = 1 then
			v_seps_length := 2;
		end if;
		if v_seps_length > length(p_seps) then
			v_seps_diff := v_seps_length - length(p_seps);
			p_seps := p_seps || SUBSTRING(p_alphabet, 1, v_seps_diff);
			p_alphabet := SUBSTRING(p_alphabet, v_seps_diff + 1);
		else
			p_seps := SUBSTRING(p_seps, 1, v_seps_length + 1);
		end if;
	end if;

	p_alphabet := hashids.consistent_shuffle(p_alphabet, p_salt);

	v_guard_count := cast(ceil(length(p_alphabet) / v_guard_div ) as integer);

	if length(p_alphabet) < 3 then
		p_guards := SUBSTRING(p_seps, 1, v_guard_count);
		p_seps := SUBSTRING(p_seps, v_guard_count + 1);
	else
		p_guards := SUBSTRING(p_alphabet, 1, v_guard_count);
		p_alphabet := SUBSTRING(p_alphabet, v_guard_count + 1);
	end if;

END;
$_$;


--
-- Name: unhash(text, text, boolean); Type: FUNCTION; Schema: hashids; Owner: -
--

CREATE FUNCTION hashids.unhash(p_input text, p_alphabet text, p_zero_offset boolean DEFAULT true) RETURNS bigint
    LANGUAGE plpgsql IMMUTABLE
    AS $_$
DECLARE
    p_input ALIAS for $1;
    p_alphabet ALIAS for $2;
    p_zero_offset integer := case when $3 = true then 1 else 0 end ; -- adding an offset so that this can work with values from a zero based array language
    v_input_length integer := length($1);
    v_alphabet_length integer := length($2);
    v_ret bigint := 0;
    v_input_char char(1);
    v_pos integer;
    v_i integer := 1;
BEGIN
    for v_i in 1..v_input_length loop
        v_input_char := SUBSTRING(p_input, (v_i), 1);
        v_pos := POSITION(v_input_char in p_alphabet) - p_zero_offset; -- have to remove one to interface with .net because it is a zero based index
        --raise notice '%[%] is % to position % in %', p_input, v_i, v_input_char, v_pos, p_alphabet;
        --raise notice '  % + (% * power(%, % - % - 1)) == %', v_ret, v_pos, v_alphabet_length, v_input_length, (v_i - 1), v_ret + (v_pos * power(v_alphabet_length, v_input_length - (v_i-1) - 1));
        v_ret := v_ret + (v_pos * power(v_alphabet_length, v_input_length - (v_i-p_zero_offset) - 1));
    end loop;

    RETURN v_ret;
END;
$_$;


--
-- Name: add_fk_constraints(text[]); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.add_fk_constraints(_table_names text[]) RETURNS void
    LANGUAGE plpgsql
    AS $$
DECLARE
   _table_name text;
BEGIN
   FOREACH _table_name IN ARRAY _table_names
   LOOP
      -- Logging the action
      RAISE NOTICE 'Adding foreign key constraint to table %', _table_name;

      EXECUTE format('ALTER TABLE %s ADD CONSTRAINT %s FOREIGN KEY (blocknumber) REFERENCES blocks (number) ON DELETE CASCADE', 
                     quote_ident(_table_name), 
                     quote_ident(_table_name || '_blocknumber_fkey'));

   END LOOP;
END
$$;


--
-- Name: chat_allowed(integer, integer); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.chat_allowed(from_user_id integer, to_user_id integer) RETURNS boolean
    LANGUAGE plpgsql
    AS $$
DECLARE
  can_message BOOLEAN;
  permission_row chat_permissions%ROWTYPE;
BEGIN

  -- explicit block
  IF EXISTS (
    SELECT 1
    FROM chat_blocked_users
    WHERE
      -- don't allow blockee to message blocker
      (blocker_user_id = to_user_id AND blockee_user_id = from_user_id)
      -- also don't allower blocker to message blockee (prohibit one way send)
      OR (blocker_user_id = from_user_id AND blockee_user_id = to_user_id)
  ) THEN
    RETURN FALSE;
  END IF;

  -- no permissions set... assume ok
  SELECT COUNT(*) = 0 INTO can_message
  FROM chat_permissions
  WHERE user_id = to_user_id;

  IF can_message THEN
    RETURN TRUE;
  END IF;

  -- existing chat takes priority over permissions
  SELECT COUNT(*) > 0 INTO can_message
  FROM chat_member member_a
  JOIN chat_member member_b USING (chat_id)
  JOIN chat_message USING (chat_id)
  WHERE member_a.user_id = from_user_id
    AND member_b.user_id = to_user_id
    AND (member_b.cleared_history_at IS NULL OR chat_message.created_at > member_b.cleared_history_at)
  ;

  IF can_message THEN
    RETURN TRUE;
  END IF;


  -- check permissions in turn:
  FOR permission_row IN select * from chat_permissions WHERE user_id = to_user_id AND allowed = TRUE
  LOOP
    CASE permission_row.permits

      WHEN 'followees' THEN
        IF EXISTS (
          SELECT 1
          FROM follows
          WHERE followee_user_id = from_user_id
          AND follower_user_id = to_user_id
          AND is_delete = false
        ) THEN
          RETURN TRUE;
        END IF;

      WHEN 'followers' THEN
        IF EXISTS (
          SELECT 1
          FROM follows
          WHERE follower_user_id = from_user_id
          AND followee_user_id = to_user_id
          AND is_delete = false
        ) THEN
          RETURN TRUE;
        END IF;

      WHEN 'tippees' THEN
        IF EXISTS (
          SELECT 1
          FROM user_tips tip
          WHERE receiver_user_id = from_user_id
          AND sender_user_id = to_user_id
        ) THEN
          RETURN TRUE;
        END IF;

      WHEN 'tippers' THEN
        IF EXISTS (
          SELECT 1
          FROM user_tips tip
          WHERE receiver_user_id = to_user_id
          AND sender_user_id = from_user_id
        ) THEN
          RETURN TRUE;
        END IF;

      WHEN 'verified' THEN
        IF EXISTS (
          SELECT 1 FROM USERS WHERE user_id = from_user_id AND is_verified = TRUE
        ) THEN
          RETURN TRUE;
        END IF;

      WHEN 'none' THEN
        RETURN FALSE;

      WHEN 'all' THEN
        RETURN TRUE;

      ELSE
        RAISE WARNING 'unknown permits: %s', permission_row.permits;
    END CASE;
  END LOOP;

  RETURN FALSE;

END;
$$;


--
-- Name: chat_blast_audience(text); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.chat_blast_audience(blast_id_param text) RETURNS TABLE(blast_id text, to_user_id integer)
    LANGUAGE plpgsql
    AS $$
BEGIN

  RETURN QUERY
  -- follower_audience
  SELECT chat_blast.blast_id, follower_user_id AS to_user_id
  FROM follows
  JOIN chat_blast ON chat_blast.blast_id = blast_id_param
    AND chat_blast.audience = 'follower_audience'
    AND follows.followee_user_id = chat_blast.from_user_id
    AND follows.is_delete = false
    AND follows.created_at < chat_blast.created_at

  UNION

  -- tipper_audience
  SELECT chat_blast.blast_id, sender_user_id AS to_user_id
  FROM user_tips tip
  JOIN chat_blast ON chat_blast.blast_id = blast_id_param
    AND chat_blast.audience = 'tipper_audience'
    AND receiver_user_id = chat_blast.from_user_id
    AND tip.created_at < chat_blast.created_at

  UNION

  -- remixer_audience
  SELECT chat_blast.blast_id, t.owner_id AS to_user_id
  FROM tracks t
  JOIN remixes ON remixes.child_track_id = t.track_id
  JOIN tracks og ON remixes.parent_track_id = og.track_id
  JOIN chat_blast ON chat_blast.blast_id = blast_id_param
    AND chat_blast.audience = 'remixer_audience'
    AND og.owner_id = chat_blast.from_user_id
    AND (
      chat_blast.audience_content_id IS NULL
      OR (
        chat_blast.audience_content_type = 'track'
        AND chat_blast.audience_content_id = og.track_id
      )
    )

  UNION

  -- customer_audience
  SELECT chat_blast.blast_id, buyer_user_id AS to_user_id
  FROM usdc_purchases p
  JOIN chat_blast ON chat_blast.blast_id = blast_id_param
    AND chat_blast.audience = 'customer_audience'
    AND p.seller_user_id = chat_blast.from_user_id
    AND (
      chat_blast.audience_content_id IS NULL
      OR (
        chat_blast.audience_content_type = p.content_type::text
        AND chat_blast.audience_content_id = p.content_id
      )
    )

  UNION

  -- coin_holder_audience
  SELECT chat_blast.blast_id, sol_user_balances.user_id AS to_user_id
  FROM chat_blast
  JOIN artist_coins 
    ON artist_coins.user_id = chat_blast.from_user_id
  JOIN sol_user_balances 
    ON sol_user_balances.mint = artist_coins.mint
    AND sol_user_balances.balance > 0
  WHERE chat_blast.blast_id = blast_id_param
    AND chat_blast.audience = 'coin_holder_audience';

END;
$$;


--
-- Name: clear_user_records(integer[]); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.clear_user_records(user_ids integer[]) RETURNS void
    LANGUAGE plpgsql
    AS $$
begin

  delete from "user_bank_accounts" where "ethereum_address" in (select "wallet" from "users" where "user_id" = any(user_ids));
  delete from "usdc_user_bank_accounts" where "ethereum_address" in (select "wallet" from "users" where "user_id" = any(user_ids));

  delete from "users" where "user_id" = any(user_ids);
  delete from "aggregate_user" where "user_id" = any(user_ids);
  delete from "aggregate_user_tips" where "sender_user_id" = any(user_ids);
  delete from "aggregate_user_tips" where "receiver_user_id" = any(user_ids);
  delete from "user_tips" where "sender_user_id" = any(user_ids);
  delete from "user_tips" where "receiver_user_id" = any(user_ids);
  delete from "user_challenges" where "user_id" = any(user_ids);
  delete from "follows" where "follower_user_id" = any(user_ids);
  delete from "follows" where "followee_user_id" = any(user_ids);
  delete from "user_pubkeys" where "user_id" = any(user_ids);
  delete from "user_events" where "user_id" = any(user_ids);
  delete from "saves" where "user_id" = any(user_ids);
  delete from "challenge_disbursements" where "user_id" = any(user_ids);
  delete from "challenge_profile_completion" where "user_id" = any(user_ids);
  delete from "subscriptions" where "subscriber_id" = any(user_ids);
  delete from "associated_wallets" where "user_id" = any(user_ids);
  delete from "plays" where "user_id" = any(user_ids);
  delete from "related_artists" where "user_id" = any(user_ids);
  delete from "trending_results" where "user_id" = any(user_ids);
  delete from "supporter_rank_ups" where "sender_user_id" = any(user_ids);
  delete from "supporter_rank_ups" where "receiver_user_id" = any(user_ids);
  delete from "user_balance_changes" where "user_id" = any(user_ids);
  delete from "user_listening_history" where "user_id" = any(user_ids);
  delete from "challenge_listen_streak" where "user_id" = any(user_ids);
  delete from "user_balances" where "user_id" = any(user_ids);
  delete from "chat_permissions" where "user_id" = any(user_ids);
  delete from "chat_message_reactions" where "user_id" = any(user_ids);
  delete from "playlist_seen" where "user_id" = any(user_ids);
  delete from "chat_ban" where "user_id" = any(user_ids);
  delete from "chat_blocked_users" where "blocker_user_id" = any(user_ids);
  delete from "chat_blocked_users" where "blockee_user_id" = any(user_ids);
  delete from "chat_member" where "user_id" = any(user_ids);
  delete from "chat_message" where "user_id" = any(user_ids);
  delete from "user_delist_statuses" where "user_id" = any(user_ids);
  delete from "grants" where "user_id" = any(user_ids);
  delete from "notification_seen" where "user_id" = any(user_ids);
  delete from "developer_apps" where "user_id" = any(user_ids);
  delete from "reposts" where "user_id" = any(user_ids);
  delete from "playlists" where "playlist_owner_id" = any(user_ids);
  delete from "playlist_routes" where "owner_id" = any(user_ids);
  delete from "track_delist_statuses" where "owner_id" = any(user_ids);
  delete from "track_routes" where "owner_id" = any(user_ids);
  delete from "tracks" where "owner_id" = any(user_ids);
  delete from "usdc_purchases" where "buyer_user_id" = any(user_ids);
  delete from "usdc_purchases" where "seller_user_id" = any(user_ids);

end;
$$;


--
-- Name: compute_user_score(bigint, bigint, bigint, bigint, bigint, boolean, bigint, bigint); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.compute_user_score(play_count bigint, follower_count bigint, challenge_count bigint, chat_block_count bigint, following_count bigint, is_audius_impersonator boolean, distinct_tracks_played bigint, karma bigint) RETURNS bigint
    LANGUAGE sql IMMUTABLE
    AS $$
select (play_count / 2) + follower_count - challenge_count - (chat_block_count * 100) + karma + case
        when following_count < 5 then -1
        else 0
    end + case
        when is_audius_impersonator then -1000
        else 0
    end + case
        when distinct_tracks_played <= 3 then -10
        else 0
    end $$;


--
-- Name: country_to_iso_alpha2(text); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.country_to_iso_alpha2(country_name text) RETURNS text
    LANGUAGE plpgsql
    AS $$
DECLARE
    iso2_code TEXT;
BEGIN
    SELECT INTO iso2_code
    CASE
        -- standards
        WHEN country_name ILIKE 'Andorra' THEN 'AD'
        WHEN country_name ILIKE 'United Arab Emirates' THEN 'AE'
        WHEN country_name ILIKE 'Afghanistan' THEN 'AF'
        WHEN country_name ILIKE 'Antigua and Barbuda' THEN 'AG'
        WHEN country_name ILIKE 'Anguilla' THEN 'AI'
        WHEN country_name ILIKE 'Albania' THEN 'AL'
        WHEN country_name ILIKE 'Armenia' THEN 'AM'
        WHEN country_name ILIKE 'Netherlands Antilles' THEN 'AN'
        WHEN country_name ILIKE 'Angola' THEN 'AO'
        WHEN country_name ILIKE 'Antarctica' THEN 'AQ'
        WHEN country_name ILIKE 'Argentina' THEN 'AR'
        WHEN country_name ILIKE 'American Samoa' THEN 'AS'
        WHEN country_name ILIKE 'Austria' THEN 'AT'
        WHEN country_name ILIKE 'Australia' THEN 'AU'
        WHEN country_name ILIKE 'Aruba' THEN 'AW'
        WHEN country_name ILIKE 'Åland' THEN 'AX'
        WHEN country_name ILIKE 'Azerbaijan' THEN 'AZ'
        WHEN country_name ILIKE 'Bosnia and Herzegovina' THEN 'BA'
        WHEN country_name ILIKE 'Barbados' THEN 'BB'
        WHEN country_name ILIKE 'Bangladesh' THEN 'BD'
        WHEN country_name ILIKE 'Belgium' THEN 'BE'
        WHEN country_name ILIKE 'Burkina Faso' THEN 'BF'
        WHEN country_name ILIKE 'Bulgaria' THEN 'BG'
        WHEN country_name ILIKE 'Bahrain' THEN 'BH'
        WHEN country_name ILIKE 'Burundi' THEN 'BI'
        WHEN country_name ILIKE 'Benin' THEN 'BJ'
        WHEN country_name ILIKE 'Saint Barthélemy' THEN 'BL'
        WHEN country_name ILIKE 'Bermuda' THEN 'BM'
        WHEN country_name ILIKE 'Brunei Darussalam' THEN 'BN'
        WHEN country_name ILIKE 'Bolivia' THEN 'BO'
        WHEN country_name ILIKE 'Brazil' THEN 'BR'
        WHEN country_name ILIKE 'Bahamas' THEN 'BS'
        WHEN country_name ILIKE 'Bhutan' THEN 'BT'
        WHEN country_name ILIKE 'Bouvet Island' THEN 'BV'
        WHEN country_name ILIKE 'Botswana' THEN 'BW'
        WHEN country_name ILIKE 'Belarus' THEN 'BY'
        WHEN country_name ILIKE 'Belize' THEN 'BZ'
        WHEN country_name ILIKE 'Canada' THEN 'CA'
        WHEN country_name ILIKE 'Cocos (Keeling) Islands' THEN 'CC'
        WHEN country_name ILIKE 'Congo (Kinshasa)' THEN 'CD'
        WHEN country_name ILIKE 'Central African Republic' THEN 'CF'
        WHEN country_name ILIKE 'Congo (Brazzaville)' THEN 'CG'
        WHEN country_name ILIKE 'Switzerland' THEN 'CH'
        WHEN country_name ILIKE 'Côte d''Ivoire' THEN 'CI'
        WHEN country_name ILIKE 'Cook Islands' THEN 'CK'
        WHEN country_name ILIKE 'Chile' THEN 'CL'
        WHEN country_name ILIKE 'Cameroon' THEN 'CM'
        WHEN country_name ILIKE 'China' THEN 'CN'
        WHEN country_name ILIKE 'Colombia' THEN 'CO'
        WHEN country_name ILIKE 'Costa Rica' THEN 'CR'
        WHEN country_name ILIKE 'Cuba' THEN 'CU'
        WHEN country_name ILIKE 'Cape Verde' THEN 'CV'
        WHEN country_name ILIKE 'Christmas Island' THEN 'CX'
        WHEN country_name ILIKE 'Cyprus' THEN 'CY'
        WHEN country_name ILIKE 'Czech Republic' THEN 'CZ'
        WHEN country_name ILIKE 'Germany' THEN 'DE'
        WHEN country_name ILIKE 'Djibouti' THEN 'DJ'
        WHEN country_name ILIKE 'Denmark' THEN 'DK'
        WHEN country_name ILIKE 'Dominica' THEN 'DM'
        WHEN country_name ILIKE 'Dominican Republic' THEN 'DO'
        WHEN country_name ILIKE 'Algeria' THEN 'DZ'
        WHEN country_name ILIKE 'Ecuador' THEN 'EC'
        WHEN country_name ILIKE 'Estonia' THEN 'EE'
        WHEN country_name ILIKE 'Egypt' THEN 'EG'
        WHEN country_name ILIKE 'Western Sahara' THEN 'EH'
        WHEN country_name ILIKE 'Eritrea' THEN 'ER'
        WHEN country_name ILIKE 'Spain' THEN 'ES'
        WHEN country_name ILIKE 'Ethiopia' THEN 'ET'
        WHEN country_name ILIKE 'Finland' THEN 'FI'
        WHEN country_name ILIKE 'Fiji' THEN 'FJ'
        WHEN country_name ILIKE 'Falkland Islands' THEN 'FK'
        WHEN country_name ILIKE 'Micronesia' THEN 'FM'
        WHEN country_name ILIKE 'Faroe Islands' THEN 'FO'
        WHEN country_name ILIKE 'France' THEN 'FR'
        WHEN country_name ILIKE 'Gabon' THEN 'GA'
        WHEN country_name ILIKE 'United Kingdom' THEN 'GB'
        WHEN country_name ILIKE 'Grenada' THEN 'GD'
        WHEN country_name ILIKE 'Georgia' THEN 'GE'
        WHEN country_name ILIKE 'French Guiana' THEN 'GF'
        WHEN country_name ILIKE 'Guernsey' THEN 'GG'
        WHEN country_name ILIKE 'Ghana' THEN 'GH'
        WHEN country_name ILIKE 'Gibraltar' THEN 'GI'
        WHEN country_name ILIKE 'Greenland' THEN 'GL'
        WHEN country_name ILIKE 'Gambia' THEN 'GM'
        WHEN country_name ILIKE 'Guinea' THEN 'GN'
        WHEN country_name ILIKE 'Guadeloupe' THEN 'GP'
        WHEN country_name ILIKE 'Equatorial Guinea' THEN 'GQ'
        WHEN country_name ILIKE 'Greece' THEN 'GR'
        WHEN country_name ILIKE 'South Georgia and South Sandwich Islands' THEN 'GS'
        WHEN country_name ILIKE 'Guatemala' THEN 'GT'
        WHEN country_name ILIKE 'Guam' THEN 'GU'
        WHEN country_name ILIKE 'Guinea-Bissau' THEN 'GW'
        WHEN country_name ILIKE 'Guyana' THEN 'GY'
        WHEN country_name ILIKE 'Hong Kong' THEN 'HK'
        WHEN country_name ILIKE 'Heard and McDonald Islands' THEN 'HM'
        WHEN country_name ILIKE 'Honduras' THEN 'HN'
        WHEN country_name ILIKE 'Croatia' THEN 'HR'
        WHEN country_name ILIKE 'Haiti' THEN 'HT'
        WHEN country_name ILIKE 'Hungary' THEN 'HU'
        WHEN country_name ILIKE 'Indonesia' THEN 'ID'
        WHEN country_name ILIKE 'Ireland' THEN 'IE'
        WHEN country_name ILIKE 'Israel' THEN 'IL'
        WHEN country_name ILIKE 'Isle of Man' THEN 'IM'
        WHEN country_name ILIKE 'India' THEN 'IN'
        WHEN country_name ILIKE 'British Indian Ocean Territory' THEN 'IO'
        WHEN country_name ILIKE 'Iraq' THEN 'IQ'
        WHEN country_name ILIKE 'Iran' THEN 'IR'
        WHEN country_name ILIKE 'Iceland' THEN 'IS'
        WHEN country_name ILIKE 'Italy' THEN 'IT'
        WHEN country_name ILIKE 'Jersey' THEN 'JE'
        WHEN country_name ILIKE 'Jamaica' THEN 'JM'
        WHEN country_name ILIKE 'Jordan' THEN 'JO'
        WHEN country_name ILIKE 'Japan' THEN 'JP'
        WHEN country_name ILIKE 'Kenya' THEN 'KE'
        WHEN country_name ILIKE 'Kyrgyzstan' THEN 'KG'
        WHEN country_name ILIKE 'Cambodia' THEN 'KH'
        WHEN country_name ILIKE 'Kiribati' THEN 'KI'
        WHEN country_name ILIKE 'Comoros' THEN 'KM'
        WHEN country_name ILIKE 'Saint Kitts and Nevis' THEN 'KN'
        WHEN country_name ILIKE 'Korea, North' THEN 'KP'
        WHEN country_name ILIKE 'Korea, South' THEN 'KR'
        WHEN country_name ILIKE 'Kuwait' THEN 'KW'
        WHEN country_name ILIKE 'Cayman Islands' THEN 'KY'
        WHEN country_name ILIKE 'Kazakhstan' THEN 'KZ'
        WHEN country_name ILIKE 'Laos' THEN 'LA'
        WHEN country_name ILIKE 'Lebanon' THEN 'LB'
        WHEN country_name ILIKE 'Saint Lucia' THEN 'LC'
        WHEN country_name ILIKE 'Liechtenstein' THEN 'LI'
        WHEN country_name ILIKE 'Sri Lanka' THEN 'LK'
        WHEN country_name ILIKE 'Liberia' THEN 'LR'
        WHEN country_name ILIKE 'Lesotho' THEN 'LS'
        WHEN country_name ILIKE 'Lithuania' THEN 'LT'
        WHEN country_name ILIKE 'Luxembourg' THEN 'LU'
        WHEN country_name ILIKE 'Latvia' THEN 'LV'
        WHEN country_name ILIKE 'Libya' THEN 'LY'
        WHEN country_name ILIKE 'Morocco' THEN 'MA'
        WHEN country_name ILIKE 'Monaco' THEN 'MC'
        WHEN country_name ILIKE 'Moldova' THEN 'MD'
        WHEN country_name ILIKE 'Montenegro' THEN 'ME'
        WHEN country_name ILIKE 'Saint Martin (French part)' THEN 'MF'
        WHEN country_name ILIKE 'Madagascar' THEN 'MG'
        WHEN country_name ILIKE 'Marshall Islands' THEN 'MH'
        WHEN country_name ILIKE 'Macedonia' THEN 'MK'
        WHEN country_name ILIKE 'Mali' THEN 'ML'
        WHEN country_name ILIKE 'Myanmar' THEN 'MM'
        WHEN country_name ILIKE 'Mongolia' THEN 'MN'
        WHEN country_name ILIKE 'Macau' THEN 'MO'
        WHEN country_name ILIKE 'Northern Mariana Islands' THEN 'MP'
        WHEN country_name ILIKE 'Martinique' THEN 'MQ'
        WHEN country_name ILIKE 'Mauritania' THEN 'MR'
        WHEN country_name ILIKE 'Montserrat' THEN 'MS'
        WHEN country_name ILIKE 'Malta' THEN 'MT'
        WHEN country_name ILIKE 'Mauritius' THEN 'MU'
        WHEN country_name ILIKE 'Maldives' THEN 'MV'
        WHEN country_name ILIKE 'Malawi' THEN 'MW'
        WHEN country_name ILIKE 'Mexico' THEN 'MX'
        WHEN country_name ILIKE 'Malaysia' THEN 'MY'
        WHEN country_name ILIKE 'Mozambique' THEN 'MZ'
        WHEN country_name ILIKE 'Namibia' THEN 'NA'
        WHEN country_name ILIKE 'New Caledonia' THEN 'NC'
        WHEN country_name ILIKE 'Niger' THEN 'NE'
        WHEN country_name ILIKE 'Norfolk Island' THEN 'NF'
        WHEN country_name ILIKE 'Nigeria' THEN 'NG'
        WHEN country_name ILIKE 'Nicaragua' THEN 'NI'
        WHEN country_name ILIKE 'Netherlands' THEN 'NL'
        WHEN country_name ILIKE 'Norway' THEN 'NO'
        WHEN country_name ILIKE 'Nepal' THEN 'NP'
        WHEN country_name ILIKE 'Nauru' THEN 'NR'
        WHEN country_name ILIKE 'Niue' THEN 'NU'
        WHEN country_name ILIKE 'New Zealand' THEN 'NZ'
        WHEN country_name ILIKE 'Oman' THEN 'OM'
        WHEN country_name ILIKE 'Panama' THEN 'PA'
        WHEN country_name ILIKE 'Peru' THEN 'PE'
        WHEN country_name ILIKE 'French Polynesia' THEN 'PF'
        WHEN country_name ILIKE 'Papua New Guinea' THEN 'PG'
        WHEN country_name ILIKE 'Philippines' THEN 'PH'
        WHEN country_name ILIKE 'Pakistan' THEN 'PK'
        WHEN country_name ILIKE 'Poland' THEN 'PL'
        WHEN country_name ILIKE 'Saint Pierre and Miquelon' THEN 'PM'
        WHEN country_name ILIKE 'Pitcairn' THEN 'PN'
        WHEN country_name ILIKE 'Puerto Rico' THEN 'PR'
        WHEN country_name ILIKE 'Palestine' THEN 'PS'
        WHEN country_name ILIKE 'Portugal' THEN 'PT'
        WHEN country_name ILIKE 'Palau' THEN 'PW'
        WHEN country_name ILIKE 'Paraguay' THEN 'PY'
        WHEN country_name ILIKE 'Qatar' THEN 'QA'
        WHEN country_name ILIKE 'Reunion' THEN 'RE'
        WHEN country_name ILIKE 'Romania' THEN 'RO'
        WHEN country_name ILIKE 'Serbia' THEN 'RS'
        WHEN country_name ILIKE 'Russian Federation' THEN 'RU'
        WHEN country_name ILIKE 'Rwanda' THEN 'RW'
        WHEN country_name ILIKE 'Saudi Arabia' THEN 'SA'
        WHEN country_name ILIKE 'Solomon Islands' THEN 'SB'
        WHEN country_name ILIKE 'Seychelles' THEN 'SC'
        WHEN country_name ILIKE 'Sudan' THEN 'SD'
        WHEN country_name ILIKE 'Sweden' THEN 'SE'
        WHEN country_name ILIKE 'Singapore' THEN 'SG'
        WHEN country_name ILIKE 'Saint Helena' THEN 'SH'
        WHEN country_name ILIKE 'Slovenia' THEN 'SI'
        WHEN country_name ILIKE 'Svalbard and Jan Mayen Islands' THEN 'SJ'
        WHEN country_name ILIKE 'Slovakia' THEN 'SK'
        WHEN country_name ILIKE 'Sierra Leone' THEN 'SL'
        WHEN country_name ILIKE 'San Marino' THEN 'SM'
        WHEN country_name ILIKE 'Senegal' THEN 'SN'
        WHEN country_name ILIKE 'Somalia' THEN 'SO'
        WHEN country_name ILIKE 'Suriname' THEN 'SR'
        WHEN country_name ILIKE 'Sao Tome and Principe' THEN 'ST'
        WHEN country_name ILIKE 'El Salvador' THEN 'SV'
        WHEN country_name ILIKE 'Syria' THEN 'SY'
        WHEN country_name ILIKE 'Swaziland' THEN 'SZ'
        WHEN country_name ILIKE 'Turks and Caicos Islands' THEN 'TC'
        WHEN country_name ILIKE 'Chad' THEN 'TD'
        WHEN country_name ILIKE 'French Southern Lands' THEN 'TF'
        WHEN country_name ILIKE 'Togo' THEN 'TG'
        WHEN country_name ILIKE 'Thailand' THEN 'TH'
        WHEN country_name ILIKE 'Tajikistan' THEN 'TJ'
        WHEN country_name ILIKE 'Tokelau' THEN 'TK'
        WHEN country_name ILIKE 'Timor-Leste' THEN 'TL'
        WHEN country_name ILIKE 'Turkmenistan' THEN 'TM'
        WHEN country_name ILIKE 'Tunisia' THEN 'TN'
        WHEN country_name ILIKE 'Tonga' THEN 'TO'
        WHEN country_name ILIKE 'Turkey' THEN 'TR'
        WHEN country_name ILIKE 'Trinidad and Tobago' THEN 'TT'
        WHEN country_name ILIKE 'Tuvalu' THEN 'TV'
        WHEN country_name ILIKE 'Taiwan' THEN 'TW'
        WHEN country_name ILIKE 'Tanzania' THEN 'TZ'
        WHEN country_name ILIKE 'Ukraine' THEN 'UA'
        WHEN country_name ILIKE 'Uganda' THEN 'UG'
        WHEN country_name ILIKE 'United States Minor Outlying Islands' THEN 'UM'
        WHEN country_name ILIKE 'United States of America' THEN 'US'
        WHEN country_name ILIKE 'Uruguay' THEN 'UY'
        WHEN country_name ILIKE 'Uzbekistan' THEN 'UZ'
        WHEN country_name ILIKE 'Vatican City' THEN 'VA'
        WHEN country_name ILIKE 'Saint Vincent and the Grenadines' THEN 'VC'
        WHEN country_name ILIKE 'Venezuela' THEN 'VE'
        WHEN country_name ILIKE 'Virgin Islands, British' THEN 'VG'
        WHEN country_name ILIKE 'Virgin Islands, U.S.' THEN 'VI'
        WHEN country_name ILIKE 'Vietnam' THEN 'VN'
        WHEN country_name ILIKE 'Vanuatu' THEN 'VU'
        WHEN country_name ILIKE 'Wallis and Futuna Islands' THEN 'WF'
        WHEN country_name ILIKE 'Samoa' THEN 'WS'
        WHEN country_name ILIKE 'Yemen' THEN 'YE'
        WHEN country_name ILIKE 'Mayotte' THEN 'YT'
        WHEN country_name ILIKE 'South Africa' THEN 'ZA'
        WHEN country_name ILIKE 'Zambia' THEN 'ZM'
        WHEN country_name ILIKE 'Zimbabwe' THEN 'ZW'
        WHEN country_name ILIKE 'Bonaire' THEN 'BQ'
        WHEN country_name ILIKE 'Curaçao' THEN 'CW'
        WHEN country_name ILIKE 'South Sudan' THEN 'SS'
        WHEN country_name ILIKE 'Sint Maarten' THEN 'SX'
        WHEN country_name ILIKE 'Kosovo' THEN 'XK'
        -- audius exceptions
        WHEN country_name ILIKE 'Aland Islands' THEN 'AX'
        WHEN country_name ILIKE 'Bonaire, Saint Eustatius and Saba ' THEN 'BQ'
        WHEN country_name ILIKE 'British Virgin Islands' THEN 'VG'
        WHEN country_name ILIKE 'Brunei' THEN 'BN'
        WHEN country_name ILIKE 'Cabo Verde' THEN 'CV'
        WHEN country_name ILIKE 'Cocos Islands' THEN 'CC'
        WHEN country_name ILIKE 'Curacao' THEN 'CW'
        WHEN country_name ILIKE 'Czechia' THEN 'CZ'
        WHEN country_name ILIKE 'Democratic Republic of the Congo' THEN 'CD'
        WHEN country_name ILIKE 'Eswatini' THEN 'SZ'
        WHEN country_name ILIKE 'Ivory Coast' THEN 'CI'
        WHEN country_name ILIKE 'Macao' THEN 'MO'
        WHEN country_name ILIKE 'North Macedonia' THEN 'MK'
        WHEN country_name ILIKE 'Palestinian Territory' THEN 'PS'
        WHEN country_name ILIKE 'Republic of the Congo' THEN 'CG'
        WHEN country_name ILIKE 'Russia' THEN 'RU'
        WHEN country_name ILIKE 'Saint Barthelemy' THEN 'BL'
        WHEN country_name ILIKE 'Saint Martin' THEN 'MF'
        WHEN country_name ILIKE 'South Korea' THEN 'KR'
        WHEN country_name ILIKE 'The Netherlands' THEN 'NL'
        WHEN country_name ILIKE 'Timor Leste' THEN 'TL'
        WHEN country_name ILIKE 'U.S. Virgin Islands' THEN 'VI'
        WHEN country_name ILIKE 'Wallis and Futuna' THEN 'WF'
        WHEN country_name ILIKE 'United States' THEN 'US'
        ELSE NULL
    END;

    RETURN iso2_code;
END;
$$;


--
-- Name: delete_constraints_referencing_blocks(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.delete_constraints_referencing_blocks() RETURNS void
    LANGUAGE plpgsql
    AS $$
DECLARE
    constraint_record RECORD;
BEGIN
    FOR constraint_record IN (
        SELECT
            c.conname AS constraint_name,
            conrelid::regclass AS referencing_table
        FROM
            pg_constraint c
        JOIN
            pg_attribute a ON a.attnum = ANY(c.conkey)
        WHERE
            confrelid = 'blocks'::regclass
            AND contype = 'f'
            AND pg_get_constraintdef(c.oid) NOT LIKE '%ON DELETE CASCADE%'
        GROUP BY
            c.conname, conrelid::regclass
    )
    LOOP
        -- Drop the foreign key constraint
        EXECUTE 'ALTER TABLE ' || constraint_record.referencing_table || ' DROP CONSTRAINT ' || constraint_record.constraint_name;
    END LOOP;
END;
$$;


--
-- Name: delete_is_current_false_rows(text[]); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.delete_is_current_false_rows(_table_names text[]) RETURNS void
    LANGUAGE plpgsql
    AS $$
DECLARE
   _table_name text;
BEGIN
   FOREACH _table_name IN ARRAY _table_names
   LOOP
      -- Logging the deletion
      RAISE NOTICE 'Deleting rows from table % where is_current is false', _table_name;

      EXECUTE format('DELETE FROM %s WHERE is_current = false', 
                     quote_ident(_table_name));
                     
   END LOOP;
END
$$;


--
-- Name: delete_rows(text[]); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.delete_rows(_table_names text[]) RETURNS void
    LANGUAGE plpgsql
    AS $$
DECLARE
   _table_name text;
BEGIN
   FOREACH _table_name IN ARRAY _table_names
   LOOP
      RAISE NOTICE 'Deleting rows from table % where is_current is false', _table_name;

      EXECUTE format('DELETE FROM %s WHERE is_current = false', 
                     quote_ident(_table_name));

   END LOOP;
END
$$;


--
-- Name: drop_fk_constraints(text[]); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.drop_fk_constraints(_table_names text[]) RETURNS void
    LANGUAGE plpgsql
    AS $$
DECLARE
   _table_name text;
BEGIN
   FOREACH _table_name IN ARRAY _table_names
   LOOP
      RAISE NOTICE 'Dropping foreign key constraint to table %', _table_name;
      EXECUTE format('LOCK TABLE %s IN ACCESS EXCLUSIVE MODE', 
                     quote_ident(_table_name));

      EXECUTE format('ALTER TABLE %s DROP CONSTRAINT IF EXISTS %s', 
                     quote_ident(_table_name), 
                     quote_ident(_table_name || '_blocknumber_fkey'));

   END LOOP;
END
$$;


--
-- Name: find_track(text); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.find_track(url text) RETURNS TABLE(user_id integer, track_id integer)
    LANGUAGE plpgsql
    AS $$
DECLARE
    segments text[];
    v_handle text;
    v_slug text;
BEGIN
    -- Split the URL into path segments
    segments := string_to_array(regexp_replace(url, '^.+://[^/]+', ''), '/');

    -- Remove empty segments
    segments := segments[array_length(segments, 1) - array_upper(segments, 1) + 1:];

    -- Retrieve the last two segments
    v_slug := segments[array_upper(segments, 1)];
    v_handle := segments[array_upper(segments, 1) - 1];

    select u.user_id into user_id from users u where handle_lc = lower(v_handle);

    select r.track_id
    into track_id
    from track_routes r
    where r.slug = v_slug
      and owner_id = user_id
    order by is_current desc
    limit 1;

    return next;
END;
$$;


--
-- Name: get_shadowbanned_users(integer[]); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.get_shadowbanned_users(user_ids integer[]) RETURNS TABLE(user_id integer)
    LANGUAGE plpgsql
    AS $$ begin return query with scoped_users as (
        select users.user_id
        from users
        where users.user_id = any (user_ids)
    ),
    play_activity as (
        select plays.user_id,
            count(distinct date_trunc('minute', plays.created_at)) as play_count
        from plays
        where plays.user_id is not null
            and plays.user_id in (
                select scoped_users.user_id
                from scoped_users
            )
        group by plays.user_id
    ),
    fast_challenge_completion as (
        select users.user_id,
            handle_lc,
            users.created_at,
            count(*) as challenge_count,
            array_agg(user_challenges.challenge_id) as challenge_ids
        from users
            left join user_challenges on users.user_id = user_challenges.user_id
        where user_challenges.is_complete
            and user_challenges.completed_at - users.created_at <= interval '3 minutes'
            and user_challenges.challenge_id not in ('m', 'b')
            and users.user_id in (
                select scoped_users.user_id
                from scoped_users
            )
        group by users.user_id,
            users.handle_lc,
            users.created_at
        order by users.created_at desc
    ),
    aggregate_scores as (
        select users.user_id,
            users.handle_lc,
            users.created_at,
            coalesce(play_activity.play_count, 0) as play_count,
            coalesce(fast_challenge_completion.challenge_count, 0) as challenge_count,
            coalesce(aggregate_user.following_count, 0) as following_count,
            coalesce(aggregate_user.follower_count, 0) as follower_count
        from users
            left join play_activity on users.user_id = play_activity.user_id
            left join fast_challenge_completion on users.user_id = fast_challenge_completion.user_id
            left join aggregate_user on aggregate_user.user_id = users.user_id
        where users.handle_lc is not null
            and users.user_id in (
                select scoped_users.user_id
                from scoped_users
            )
        order by users.created_at desc
    ),
    computed_scores as (
        select a.user_id,
            a.handle_lc,
            a.play_count,
            a.follower_count,
            a.challenge_count,
            a.following_count,
            (
                a.play_count + a.follower_count - a.challenge_count + case
                    when a.following_count < 5 then -1
                    else 0
                end
            ) as overall_score
        from aggregate_scores a
    )
select computed_scores.user_id
from computed_scores
where overall_score < 0;
-- filter based on threshold
end;
$$;


--
-- Name: get_user_score(integer); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.get_user_score(target_user_id integer) RETURNS TABLE(user_id integer, handle_lc text, play_count bigint, distinct_tracks_played bigint, challenge_count bigint, following_count bigint, follower_count bigint, chat_block_count bigint, is_audius_impersonator boolean, karma bigint, score bigint)
    LANGUAGE sql
    AS $$ with play_activity as (
        select p.user_id,
            count(distinct date_trunc('day', p.created_at)) as play_count,
            count(distinct p.play_item_id) as distinct_tracks_played
        from plays p
        where p.user_id = target_user_id
        group by p.user_id
    ),
    fast_challenge_completion as (
        select u.user_id,
            u.handle_lc,
            u.created_at,
            count(*) as challenge_count,
            array_agg(uc.challenge_id) as challenge_ids
        from users u
            left join user_challenges uc on u.user_id = uc.user_id
        where u.user_id = target_user_id
            and uc.is_complete
            and uc.completed_at - u.created_at <= interval '3 minutes'
            and uc.challenge_id not in ('m', 'b')
        group by u.user_id,
            u.handle_lc,
            u.created_at
    ),
    chat_blocks as (
        select c.blockee_user_id as user_id,
            count(*) as block_count
        from chat_blocked_users c
        where c.blockee_user_id = target_user_id
        group by c.blockee_user_id
    ),
    aggregate_scores as (
        select u.user_id,
            u.handle_lc,
            coalesce(p.play_count, 0) as play_count,
            coalesce(p.distinct_tracks_played, 0) as distinct_tracks_played,
            coalesce(c.challenge_count, 0) as challenge_count,
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
            left join play_activity p on u.user_id = p.user_id
            left join fast_challenge_completion c on u.user_id = c.user_id
            left join chat_blocks cb on u.user_id = cb.user_id
            left join aggregate_user au on u.user_id = au.user_id
        where u.user_id = target_user_id
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
        a.distinct_tracks_played,
        a.karma
    ) as score
from aggregate_scores a;
$$;


--
-- Name: get_user_scores(integer[]); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.get_user_scores(target_user_ids integer[] DEFAULT NULL::integer[]) RETURNS TABLE(user_id integer, handle_lc text, play_count bigint, distinct_tracks_played bigint, follower_count bigint, following_count bigint, challenge_count bigint, chat_block_count bigint, is_audius_impersonator boolean, karma bigint, score bigint)
    LANGUAGE sql
    AS $$ with play_activity as (
        select plays.user_id,
            count(distinct (date_trunc('hour', plays.created_at))) as play_count,
            count(distinct(plays.play_item_id)) as distinct_tracks_played
        from plays
            join users on plays.user_id = users.user_id
        where target_user_ids is null
            or plays.user_id = any(target_user_ids)
        group by plays.user_id
    ),
    fast_challenge_completion as (
        select users.user_id,
            handle_lc,
            users.created_at,
            count(*) as challenge_count,
            array_agg(user_challenges.challenge_id) as challenge_ids
        from users
            left join user_challenges on users.user_id = user_challenges.user_id
        where user_challenges.is_complete
            and user_challenges.completed_at - users.created_at <= interval '3 minutes'
            and user_challenges.challenge_id not in ('m', 'b')
            and (
                target_user_ids is null
                or users.user_id = any(target_user_ids)
            )
        group by users.user_id,
            users.handle_lc,
            users.created_at
    ),
    chat_blocks as (
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
            coalesce(play_activity.play_count, 0) as play_count,
            coalesce(play_activity.distinct_tracks_played, 0) as distinct_tracks_played,
            coalesce(aggregate_user.following_count, 0) as following_count,
            coalesce(aggregate_user.follower_count, 0) as follower_count,
            coalesce(fast_challenge_completion.challenge_count, 0) as challenge_count,
            coalesce(chat_blocks.block_count, 0) as chat_block_count,
            case
                when (
                    users.handle_lc ilike '%audius%'
                    or lower(users.name) ilike '%audius%'
                )
                and users.is_verified = false then true
                else false
            end as is_audius_impersonator,
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
            left join play_activity on users.user_id = play_activity.user_id
            left join fast_challenge_completion on users.user_id = fast_challenge_completion.user_id
            left join chat_blocks on users.user_id = chat_blocks.user_id
            left join aggregate_user on aggregate_user.user_id = users.user_id
        where users.handle_lc is not null
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
        a.distinct_tracks_played,
        a.karma
    ) as score
from aggregate_scores a;
$$;


--
-- Name: handle_artist_coins_change(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.handle_artist_coins_change() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    PERFORM pg_notify('artist_coins_changed', json_build_object('operation', TG_OP, 'new_mint', NEW.mint, 'old_mint', OLD.mint)::text);
    RETURN NEW;
    EXCEPTION
        WHEN OTHERS THEN
            RAISE WARNING 'An error occurred in %: %', TG_NAME, SQLERRM;
            RETURN NULL;
END;
$$;


--
-- Name: handle_associated_wallets(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.handle_associated_wallets() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE
    v_mint varchar;
BEGIN
    -- For INSERT, always run
    IF TG_OP = 'INSERT' THEN
        FOR v_mint IN
            SELECT DISTINCT mint FROM sol_token_account_balances WHERE owner = NEW.wallet
        LOOP
            PERFORM update_sol_user_balance_mint(NEW.user_id, v_mint);
        END LOOP;
    END IF;

    -- For UPDATE, only run if is_delete changed
    IF TG_OP = 'UPDATE' AND (NEW.is_delete IS DISTINCT FROM OLD.is_delete) THEN
        FOR v_mint IN
            SELECT DISTINCT mint FROM sol_token_account_balances WHERE owner = NEW.wallet
        LOOP
            PERFORM update_sol_user_balance_mint(NEW.user_id, v_mint);
        END LOOP;
    END IF;

    -- For DELETE, always run
    IF TG_OP = 'DELETE' THEN
        FOR v_mint IN
            SELECT DISTINCT mint FROM sol_token_account_balances WHERE owner = OLD.wallet
        LOOP
            PERFORM update_sol_user_balance_mint(OLD.user_id, v_mint);
        END LOOP;
    END IF;

    RETURN NULL;
EXCEPTION
    WHEN OTHERS THEN
        RAISE WARNING 'An error occurred in %: %', TG_NAME, SQLERRM;
        RETURN NULL;
END;
$$;


--
-- Name: handle_challenge_disbursement(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.handle_challenge_disbursement() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
declare
  reward_manager_tx reward_manager_txs%ROWTYPE;
	existing_notification integer;
begin

  select * into reward_manager_tx from reward_manager_txs where reward_manager_txs.signature = new.signature limit 1;

  if reward_manager_tx is not null then
		select id into existing_notification 
		from notification
		where
		type = 'challenge_reward' and
		new.user_id = any(user_ids) and
		timestamp >= (new.created_at - interval '1 hour')
		limit 1;
		
		if existing_notification is null then
			-- create a notification for the challenge disbursement
			insert into notification
			(slot, user_ids, timestamp, type, group_id, specifier, data)
			values
			(
				new.slot,
				ARRAY [new.user_id],
				new.created_at,
				'challenge_reward',
				'challenge_reward:' || new.user_id || ':challenge:' || new.challenge_id || ':specifier:' || new.specifier,
				new.user_id,
				json_build_object('specifier', new.specifier, 'challenge_id', new.challenge_id, 'amount', new.amount)
			)
			on conflict do nothing;
		end if;
  end if;
  return null;

exception
  when others then
    raise warning 'An error occurred in %: %', tg_name, sqlerrm;
    return null;

end;
$$;


--
-- Name: handle_comment(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.handle_comment() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
begin
  if new.entity_type = 'Track' then
    insert into aggregate_track (track_id) 
    values (new.entity_id) 
    on conflict do nothing;
  end if;

  -- update agg track
  if new.entity_type = 'Track' then
    update aggregate_track 
    set comment_count = (
      select count(*)
      from comments c
      where
          c.is_delete is false
          and c.is_visible is true
          and c.entity_type = new.entity_type
          and c.entity_id = new.entity_id
    )
    where track_id = new.entity_id;
  end if;

  return null;

exception
    when others then
      raise warning 'An error occurred in %: %', tg_name, sqlerrm;
      return null;
end;
$$;


--
-- Name: handle_comment_mention(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.handle_comment_mention() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
declare
  comment_user_id int;
  entity_user_id int;
  entity_id int;
  entity_type text;
begin
  select comments.user_id, comments.entity_id, comments.entity_type
  into comment_user_id , entity_id, entity_type
  from comments 
  where comment_id = new.comment_id;

  select tracks.owner_id 
  into entity_user_id 
  from tracks 
  where track_id = entity_id;

  begin
    if new.user_id != entity_user_id then
      insert into notification
        (blocknumber, user_ids, timestamp, type, specifier, group_id, data)
        values
        ( 
          new.blocknumber,
          ARRAY [new.user_id], 
          new.created_at, 
          'comment_mention',
          comment_user_id,
          'comment_mention:' || entity_id || ':type:' || entity_type,
          json_build_object
          (
            'type', entity_type,
            'entity_id', entity_id,
            'entity_user_id', entity_user_id,
            'comment_user_id', comment_user_id
          )
        )
      on conflict do nothing;
    end if;
  end;

  return null;

exception
    when others then
      raise warning 'An error occurred in %: %', tg_name, sqlerrm;
      return null;
end;
$$;


--
-- Name: handle_comment_thread(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.handle_comment_thread() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
declare
  parent_comment_user_id int;
  comment_user_id int;
  entity_user_id int;
  entity_id int;
  entity_type text;
  blocknumber int;
  created_at timestamp without time zone;
  notification_muted boolean;
begin
  select comments.user_id, comments.entity_id, comments.entity_type 
  into parent_comment_user_id, entity_id, entity_type 
  from comments 
  where comment_id = new.parent_comment_id;

  select comments.user_id, comments.blocknumber, comments.created_at
  into comment_user_id, blocknumber, created_at
  from comments 
  where comment_id = new.comment_id;

  select tracks.owner_id 
  into entity_user_id 
  from tracks 
  where track_id = entity_id;

  select comment_notification_settings.is_muted
  into notification_muted
  from comment_notification_settings
  where user_id = parent_comment_user_id 
  and comment_notification_settings.entity_id = new.parent_comment_id
  and comment_notification_settings.entity_type = 'Comment';

  begin
    if notification_muted is not true and comment_user_id != parent_comment_user_id then
      insert into notification
        (blocknumber, user_ids, timestamp, type, specifier, group_id, data)
        values
        ( 
          blocknumber,
          ARRAY [parent_comment_user_id],
          created_at, 
          'comment_thread',
          comment_user_id,
          'comment_thread:' || new.parent_comment_id,
          json_build_object
          (
            'type', entity_type,
            'entity_id', entity_id,
            'entity_user_id', entity_user_id,
            'comment_user_id', comment_user_id
          )
        )
      on conflict do nothing;
    end if;
  end;

  return null;

exception
    when others then
      raise warning 'An error occurred in %: %', tg_name, sqlerrm;
      return null;
end;
$$;


--
-- Name: handle_event(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.handle_event() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
declare
  notified_user_id int;
  owner_user_id int;
  track_is_public boolean;
begin
  -- Only proceed if this is a remix contest event
  if new.event_type = 'remix_contest' and new.is_deleted = false then
    -- Get the owner of the track and check if it's public
    select owner_id, not is_unlisted into owner_user_id, track_is_public 
    from tracks 
    where is_current and track_id = new.entity_id 
    limit 1;

    -- Only create notifications if the track is public
    if track_is_public then
      -- For each follower of the event creator and each user who favorited the track
      -- Using UNION to ensure we don't get duplicate user_ids
      for notified_user_id in
        select distinct user_id
        from (
          -- Get followers
          select f.follower_user_id as user_id
          from follows f
          where f.followee_user_id = new.user_id
            and f.is_current = true
            and f.is_delete = false
          union
          -- Get users who favorited the track
          select s.user_id
          from saves s
          where s.save_item_id = new.entity_id
            and s.save_type = 'track'
            and s.is_current = true
            and s.is_delete = false
        ) as users_to_notify
      loop
        -- Create a notification for this user
        insert into notification
          (blocknumber, user_ids, timestamp, type, specifier, group_id, data)
        values
          (
            new.blocknumber,
            ARRAY[notified_user_id],
            new.created_at,
            'fan_remix_contest_started',
            notified_user_id,
            'fan_remix_contest_started:' || new.entity_id || ':user:' || new.user_id,
            json_build_object(
              'entity_user_id', owner_user_id,
              'entity_id', new.entity_id
            )
          )
        on conflict do nothing;
      end loop;
    end if;
  end if;

  return null;

exception
  when others then
    raise warning 'An error occurred in %: %', tg_name, sqlerrm;
    return null;
end;
$$;


--
-- Name: handle_follow(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.handle_follow() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
declare
  new_follower_count int;
  milestone integer;
  delta int;
  is_shadowbanned boolean;
begin
  insert into aggregate_user (user_id) values (new.followee_user_id) on conflict do nothing;
  insert into aggregate_user (user_id) values (new.follower_user_id) on conflict do nothing;

  -- increment or decrement?
  if new.is_delete then
    delta := -1;
  else
    delta := 1;
  end if;

  update aggregate_user 
  set following_count = following_count + delta 
  where user_id = new.follower_user_id;

  update aggregate_user 
  set follower_count = follower_count + delta
  where user_id = new.followee_user_id
  returning follower_count into new_follower_count;

  -- create a milestone if applicable
  select new_follower_count into milestone where new_follower_count in (10, 25, 50, 100, 250, 500, 1000, 5000, 10000, 20000, 50000, 100000, 1000000);
  select score < 0 into is_shadowbanned from aggregate_user where user_id = new.follower_user_id;
  if milestone is not null and new.is_delete is false and is_shadowbanned = false then
      insert into milestones 
        (id, name, threshold, blocknumber, slot, timestamp)
      values
        (new.followee_user_id, 'FOLLOWER_COUNT', milestone, new.blocknumber, new.slot, new.created_at)
      on conflict do nothing;
      insert into notification
        (user_ids, type, group_id, specifier, blocknumber, timestamp, data)
        values
        (
          ARRAY [new.followee_user_id],
          'milestone_follower_count',
          'milestone:FOLLOWER_COUNT:id:' || new.followee_user_id || ':threshold:' || milestone,
          new.followee_user_id,
          new.blocknumber,
          new.created_at,
          json_build_object('type', 'FOLLOWER_COUNT', 'user_id', new.followee_user_id, 'threshold', milestone)
        )
    on conflict do nothing;
  end if;

  begin
    -- create a notification for the followee
    if new.is_delete is false and is_shadowbanned = false then
      insert into notification
      (blocknumber, user_ids, timestamp, type, specifier, group_id, data)
      values
      (
        new.blocknumber,
        ARRAY [new.followee_user_id],
        new.created_at,
        'follow',
        new.follower_user_id,
        'follow:' || new.followee_user_id,
        json_build_object('followee_user_id', new.followee_user_id, 'follower_user_id', new.follower_user_id)
      )
      on conflict do nothing;
    end if;
	exception
    when others then
      raise warning 'An error occurred in %: %', tg_name, sqlerrm;
  end;

  return null;

exception
  when others then
    raise warning 'An error occurred in %: %', tg_name, sqlerrm;
    raise;

end; 
$$;


--
-- Name: handle_on_user_challenge(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.handle_on_user_challenge() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
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
    end if;

    return new;

end;
$$;


--
-- Name: handle_play(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.handle_play() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
declare
    new_listen_count int;
    milestone int;
    owner_user_id int;
begin

    -- update aggregate_plays
    insert into aggregate_plays (play_item_id, count) values (new.play_item_id, 0) on conflict do nothing;

    update aggregate_plays
        set count = count + 1 
        where play_item_id = new.play_item_id
        returning count into new_listen_count;

    -- update aggregate_monthly_plays
    insert into aggregate_monthly_plays (play_item_id, timestamp, country, count)
    values (new.play_item_id, date_trunc('month', new.created_at), coalesce(new.country, ''), 0)
    on conflict do nothing;

    update aggregate_monthly_plays
        set count = count + 1
        where play_item_id = new.play_item_id
        and timestamp = date_trunc('month', new.created_at)
        and country = coalesce(new.country, '');

    select new_listen_count 
        into milestone 
        where new_listen_count in (10,25,50,100,250,500,1000,2500,5000,10000,25000,50000,100000,250000,500000,1000000);

    if milestone is not null then
        insert into milestones
            (id, name, threshold, slot, timestamp)
        values
            (new.play_item_id, 'LISTEN_COUNT', milestone, new.slot, new.created_at)
        on conflict do nothing;
        select tracks.owner_id into owner_user_id from tracks where is_current and track_id = new.play_item_id;
        if owner_user_id is not null then
            insert into notification
                (user_ids, specifier, group_id, type, slot, timestamp, data)
                values
                (
                    array[owner_user_id],
                    owner_user_id,
                    'milestone:LISTEN_COUNT:id:' || new.play_item_id || ':threshold:' || milestone,
                    'milestone',
                    new.slot,
                    new.created_at,
                    json_build_object('type', 'LISTEN_COUNT', 'track_id', new.play_item_id, 'threshold', milestone)
                )
            on conflict do nothing;
        end if;
    end if;
    return null;

exception
  when others then
    raise warning 'An error occurred in %: %', tg_name, sqlerrm;
    raise;

end;
$$;


--
-- Name: handle_playlist(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.handle_playlist() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
declare
  track_owner_id int := 0;
  track_item json;
  subscriber_user_ids integer[];
  old_row RECORD;
  delta int := 0;
begin

  insert into aggregate_playlist (playlist_id, is_album) values (new.playlist_id, new.is_album) on conflict do nothing;

  with expanded as (
      select
          jsonb_array_elements(prev_records->'playlists') as playlist
      from
          revert_blocks
      where blocknumber = new.blocknumber
  )
  select
      (playlist->>'is_private')::boolean as is_private,
      (playlist->>'is_delete')::boolean as is_delete
  into old_row
  from
      expanded
  where
      (playlist->>'playlist_id')::int = new.playlist_id
  limit 1;

  delta := 0;
  if (new.is_delete = true and new.is_current = true) and (old_row.is_delete = false and old_row.is_private = false) then
    delta := -1;
  end if;

  if (old_row is null and new.is_private = false) or (old_row.is_private = true and new.is_private = false) then
    delta := 1;
  end if;

  if delta != 0 then
    if new.is_album then
      update aggregate_user
      set album_count = album_count + delta
      where user_id = new.playlist_owner_id;
    else
      update aggregate_user
      set playlist_count = playlist_count + delta
      where user_id = new.playlist_owner_id;
    end if;
  end if;
  -- Create playlist notification
  begin
    if new.is_private = FALSE AND
    new.is_delete = FALSE AND
    (
      new.created_at = new.updated_at OR
      old_row.is_private = TRUE
    )
    then
      select array(
        select subscriber_id
          from subscriptions
          where is_current and
          not is_delete and
          user_id=new.playlist_owner_id
      ) into subscriber_user_ids;
      if array_length(subscriber_user_ids, 1)	> 0 then
        insert into notification
        (blocknumber, user_ids, timestamp, type, specifier, group_id, data)
        values
        (
          new.blocknumber,
          subscriber_user_ids,
          new.updated_at,
          'create',
          new.playlist_owner_id,
          'create:playlist_id:' || new.playlist_id,
          json_build_object('playlist_id', new.playlist_id, 'is_album', new.is_album)
        )
        on conflict do nothing;
      end if;
    end if;
	exception
    when others then
      raise warning 'An error occurred in %: %', tg_name, sqlerrm;
  end;

  begin
    if new.is_delete IS FALSE and new.is_private IS FALSE then
      for track_item IN select jsonb_array_elements from jsonb_array_elements(new.playlist_contents->'track_ids')
      loop
        -- Add notification for each track owner
        if (track_item->>'time')::double precision::int >= extract(epoch from new.updated_at)::int then
          select owner_id into track_owner_id from tracks where is_current and track_id=(track_item->>'track')::int;
          if track_owner_id != new.playlist_owner_id then
            insert into notification
              (blocknumber, user_ids, timestamp, type, specifier, group_id, data)
              values
              (
                new.blocknumber,
                ARRAY [track_owner_id],
                new.updated_at,
                'track_added_to_playlist',
                track_owner_id,
                'track_added_to_playlist:playlist_id:' || new.playlist_id || ':track_id:' || (track_item->>'track')::int,
                json_build_object('track_id', (track_item->>'track')::int, 'playlist_id', new.playlist_id, 'playlist_owner_id', new.playlist_owner_id)
              )
            on conflict do nothing;
          end if;
        end if;
      end loop;
    end if;
  exception
    when others then
      raise warning 'An error occurred in %: %', tg_name, sqlerrm;
  end;

  return null;

exception
  when others then
    raise warning 'An error occurred in %: %', tg_name, sqlerrm;
    raise;
end;
$$;


--
-- Name: handle_playlist_track(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.handle_playlist_track() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
declare
  playlist_record RECORD;
begin
  select * into playlist_record from playlists where playlist_id = new.playlist_id limit 1;

  -- Add notification for each purchaser
  if playlist_record.is_stream_gated = true and jsonb_exists(playlist_record.stream_conditions, 'usdc_purchase') then
    with album_purchasers as (
      select distinct buyer_user_id
        from usdc_purchases
        where content_id = new.playlist_id
        and content_type = 'album'
    )
      insert into notification
        (blocknumber, user_ids, timestamp, type, specifier, group_id, data)
        select
          playlist_record.blocknumber,
          array [album_purchaser.buyer_user_id],
          new.updated_at,
          'track_added_to_purchased_album',
          album_purchaser.buyer_user_id,
          'track_added_to_purchased_album:playlist_id:' || new.playlist_id || ':track_id:' || new.track_id,
          json_build_object('track_id', new.track_id, 'playlist_id', new.playlist_id, 'playlist_owner_id', playlist_record.playlist_owner_id)
        from album_purchasers as album_purchaser;
  end if;
  
  return null;

exception
  when others then
    raise warning 'An error occurred in %: %', tg_name, sqlerrm;
    raise;

end;
$$;


--
-- Name: handle_reaction(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.handle_reaction() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
declare
  tip_row notification%ROWTYPE;
  tip_sender_user_id int;
  tip_receiver_user_id int;
  tip_amount bigint;
begin

  raise NOTICE 'start';
  
  if new.reaction_type = 'tip' then

    raise NOTICE 'is tip';

    SELECT amount, sender_user_id, receiver_user_id 
    INTO tip_amount, tip_sender_user_id, tip_receiver_user_id 
    FROM user_tips ut 
    WHERE ut.signature = new.reacted_to;
    
    raise NOTICE 'did select % %', tip_sender_user_id, tip_receiver_user_id;
    raise NOTICE 'did select %', new.reacted_to;

    IF tip_sender_user_id IS NOT NULL AND tip_receiver_user_id IS NOT NULL THEN
      raise NOTICE 'have ids';

      if new.reaction_value != 0 then
        INSERT INTO notification
          (user_ids, timestamp, type, specifier, group_id, data)
        VALUES
          (
          ARRAY [tip_sender_user_id],
          new.timestamp,
          'reaction',
          tip_receiver_user_id,
          'reaction:' || 'reaction_to:' || new.reacted_to || ':reaction_type:' || new.reaction_type || ':reaction_value:' || new.reaction_value,
          json_build_object(
            'sender_wallet', new.sender_wallet,
            'reaction_type', new.reaction_type,
            'reacted_to', new.reacted_to,
            'reaction_value', new.reaction_value,
            'receiver_user_id', tip_receiver_user_id,
            'sender_user_id', tip_sender_user_id,
            'tip_amount', tip_amount::varchar(255)
          )
        )
        on conflict do nothing;
      end if;

      -- find the notification for tip send - update the data to include reaction value
      UPDATE notification
      SET data = jsonb_set(data, '{reaction_value}', new.reaction_value::text::jsonb)
      WHERE notification.group_id = 'tip_receive:user_id:' || tip_receiver_user_id || ':signature:' || new.reacted_to;
    end if;
  end if;

  return null;

exception
  when others then
    raise warning 'An error occurred in %: %', tg_name, sqlerrm;
    return null;
end;
$$;


--
-- Name: handle_repost(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.handle_repost() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
declare
  new_val int;
  milestone_name text;
  milestone integer;
  owner_user_id int;
  track_remix_of json;
  is_remix_cosign boolean;
  is_album boolean;
  delta int;
  entity_type text;
  playlist_row record;
  is_shadowbanned boolean;
begin
  insert into aggregate_user (user_id) values (new.user_id) on conflict do nothing;
  if new.repost_type = 'track' then
    insert into aggregate_track (track_id) values (new.repost_item_id) on conflict do nothing;

    entity_type := 'track';
  else
    insert into aggregate_playlist (playlist_id, is_album)
    select p.playlist_id, p.is_album
    from playlists p
    where p.playlist_id = new.repost_item_id
    and p.is_current
    on conflict do nothing;

    entity_type := 'playlist';

    select ap.is_album into is_album
    from aggregate_playlist ap
    where ap.playlist_id = new.repost_item_id;
  end if;

  -- increment or decrement?
  if new.is_delete then
    delta := -1;
  else
    delta := 1;
  end if;

  -- update agg user
  update aggregate_user 
  set repost_count = (
    select count(*)
    from reposts r
    where r.is_current is true
      and r.is_delete is false
      and r.user_id = new.user_id
  )
  where user_id = new.user_id;

  -- update agg track or playlist
  if new.repost_type = 'track' then
    milestone_name := 'TRACK_REPOST_COUNT';
    update aggregate_track 
    set repost_count = (
      select count(*)
      from reposts r
      where
          r.is_current is true
          and r.is_delete is false
          and r.repost_type = new.repost_type
          and r.repost_item_id = new.repost_item_id
    )
    where track_id = new.repost_item_id
    returning repost_count into new_val;
  	if new.is_delete IS FALSE then
		  select tracks.owner_id, tracks.remix_of into owner_user_id, track_remix_of from tracks where is_current and track_id = new.repost_item_id;
	  end if;
  else
    milestone_name := 'PLAYLIST_REPOST_COUNT';
    update aggregate_playlist
    set repost_count = (
      select count(*)
      from reposts r
      where
          r.is_current is true
          and r.is_delete is false
          and r.repost_type = new.repost_type
          and r.repost_item_id = new.repost_item_id
    )    
    where playlist_id = new.repost_item_id
    returning repost_count into new_val;

  	if new.is_delete IS FALSE then
		  select playlist_owner_id into owner_user_id from playlists where is_current and playlist_id = new.repost_item_id;
	  end if;
  end if;

  -- create a milestone if applicable
  select new_val into milestone where new_val in (10,25,50,100,250,500,1000,2500,5000,10000,25000,50000,100000,250000,500000,1000000);
  select score < 0 into is_shadowbanned from aggregate_user where user_id = new.user_id;

  if new.is_delete = false and milestone is not null and owner_user_id is not null and is_shadowbanned = false then
    insert into milestones 
      (id, name, threshold, blocknumber, slot, timestamp)
    values
      (new.repost_item_id, milestone_name, milestone, new.blocknumber, new.slot, new.created_at)
    on conflict do nothing;


    if entity_type = 'track' then
      insert into notification
        (user_ids, type, specifier, group_id, blocknumber, timestamp, data)
        values
        (
          ARRAY [owner_user_id],
          'milestone',
          owner_user_id,
          'milestone:' || milestone_name  || ':id:' || new.repost_item_id || ':threshold:' || milestone,
          new.blocknumber,
          new.created_at,
          json_build_object('type', milestone_name, 'track_id', new.repost_item_id, 'threshold', milestone)
        )
        on conflict do nothing;
    else
      insert into notification
        (user_ids, type, specifier, group_id, blocknumber, timestamp, data)
        values
        (
          ARRAY [owner_user_id],
          'milestone',
          owner_user_id,
          'milestone:' || milestone_name  || ':id:' || new.repost_item_id || ':threshold:' || milestone,
          new.blocknumber,
          new.created_at,
          json_build_object('type', milestone_name, 'playlist_id', new.repost_item_id, 'threshold', milestone, 'is_album', is_album)
        )
        on conflict do nothing;
    end if;
  end if;

  begin
    -- create a notification for the reposted content's owner
    if new.is_delete is false and is_shadowbanned = false then
    insert into notification
      (blocknumber, user_ids, timestamp, type, specifier, group_id, data)
      values
      (
        new.blocknumber,
        ARRAY [owner_user_id],
        new.created_at,
        'repost',
        new.user_id,
        'repost:' || new.repost_item_id || ':type:'|| new.repost_type,
        json_build_object('repost_item_id', new.repost_item_id, 'user_id', new.user_id, 'type', new.repost_type)
      )
      on conflict do nothing;
    end if;

	-- notify followees of the reposter who have reposted the same content
	-- within the last month
	if new.is_delete is false
	and new.is_repost_of_repost is true
  and is_shadowbanned = false then
	with
	    followee_repost_of_repost_ids as (
	        select user_id
	        from reposts r
	        where
	            r.repost_item_id = new.repost_item_id
	            and new.created_at - INTERVAL '1 month' < r.created_at
	            and new.created_at > r.created_at
              and r.is_delete is false
              and r.is_current is true
	            and r.user_id in (
	                select
	                    followee_user_id
	                from follows
	                where
	                    follower_user_id = new.user_id
                      and is_delete is false
                      and is_current is true
	            )
	    )
	insert into notification
		(blocknumber, user_ids, timestamp, type, specifier, group_id, data)
		SELECT blocknumber_val, user_ids_val, timestamp_val, type_val, specifier_val, group_id_val, data_val
		FROM (
			SELECT new.blocknumber AS blocknumber_val,
			ARRAY(
				SELECT user_id
				FROM
					followee_repost_of_repost_ids
			) AS user_ids_val,
			new.created_at AS timestamp_val,
			'repost_of_repost' AS type_val,
			new.user_id AS specifier_val,
			'repost_of_repost:' || new.repost_item_id || ':type:' || new.repost_type AS group_id_val,
			json_build_object(
				'repost_of_repost_item_id',
				new.repost_item_id,
				'user_id',
				new.user_id,
				'type',
        case 
          when is_album then 'album'
          else new.repost_type
        end
			) AS data_val
		) sub
		WHERE user_ids_val IS NOT NULL AND array_length(user_ids_val, 1) > 0
		on conflict do nothing;
	end if;

    -- create a notification for remix cosign
    if new.is_delete is false and new.repost_type = 'track' and track_remix_of is not null and is_shadowbanned = false then
      select
        case when tracks.owner_id = new.user_id then TRUE else FALSE end as boolean into is_remix_cosign
        from tracks
        where is_current and track_id = (track_remix_of->'tracks'->0->>'parent_track_id')::int;
      if is_remix_cosign then
        insert into notification
          (blocknumber, user_ids, timestamp, type, specifier, group_id, data)
          values
          (
            new.blocknumber,
            ARRAY [owner_user_id],
            new.created_at,
            'cosign',
            new.user_id,
            'cosign:parent_track' || (track_remix_of->'tracks'->0->>'parent_track_id')::int || ':original_track:'|| new.repost_item_id,
            json_build_object('parent_track_id', (track_remix_of->'tracks'->0->>'parent_track_id')::int, 'track_id', new.repost_item_id, 'track_owner_id', owner_user_id)
          )
        on conflict do nothing;
      end if;
    end if;

	exception
    when others then
      raise warning 'An error occurred in %: %', tg_name, sqlerrm;
  end;

  return null;

exception
    when others then
      raise warning 'An error occurred in %: %', tg_name, sqlerrm;
      return null;
end;
$$;


--
-- Name: handle_save(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.handle_save() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
declare
  new_val int;
  milestone_name text;
  milestone integer;
  owner_user_id int;
  track_remix_of json;
  is_remix_cosign boolean;
  is_album boolean;
  delta int;
  entity_type text;
  is_purchased boolean default false;
  is_containing_album_purchased boolean default false;
  is_shadowbanned boolean;
begin

  insert into aggregate_user (user_id) values (new.user_id) on conflict do nothing;
  if new.save_type = 'track' then
    insert into aggregate_track (track_id) values (new.save_item_id) on conflict do nothing;

    entity_type := 'track';

    -- check if the track has been purchased
    select exists (
        select 1
        from usdc_purchases
        where content_type = 'track'
        and content_id = new.save_item_id
        and buyer_user_id = new.user_id
    ) into is_purchased;

    -- check if the track is part of an album that has been purchased
    select exists (
      select 1
      from
        usdc_purchases
        join playlist_tracks as pt
        on content_id = pt.playlist_id
      where track_id = new.save_item_id
      and buyer_user_id = new.user_id
    ) into is_containing_album_purchased;

    is_purchased := is_purchased or is_containing_album_purchased;
  else
    insert into aggregate_playlist (playlist_id, is_album)
    select p.playlist_id, p.is_album
    from playlists p
    where p.playlist_id = new.save_item_id
    and p.is_current
    on conflict do nothing;
    
    select ap.is_album into is_album
    from aggregate_playlist ap
    where ap.playlist_id = new.save_item_id;

    select exists (
      select 1
      from usdc_purchases
      where content_type = 'album'
      and content_id = new.save_item_id
      and buyer_user_id = new.user_id
    ) into is_purchased;
  end if;

  -- increment or decrement?
  if new.is_delete then
    delta := -1;
  else
    delta := 1;
  end if;

  -- update agg track or playlist
  if new.save_type = 'track' then
    milestone_name := 'TRACK_SAVE_COUNT';

    update aggregate_track 
    set save_count = (
      select count(*)
      from saves r
      where
          r.is_current is true
          and r.is_delete is false
          and r.save_type = new.save_type
          and r.save_item_id = new.save_item_id
    )
    where track_id = new.save_item_id
    returning save_count into new_val;

    -- update agg user
    update aggregate_user 
    set track_save_count = (
      select count(*)
      from saves r
      where r.is_current is true
        and r.is_delete is false
        and r.user_id = new.user_id
        and r.save_type = new.save_type
    )
    where user_id = new.user_id;
    
  	if new.is_delete IS FALSE then
		  select tracks.owner_id, tracks.remix_of into owner_user_id, track_remix_of from tracks where is_current and track_id = new.save_item_id;
	  end if;
  else
    milestone_name := 'PLAYLIST_SAVE_COUNT';

    update aggregate_playlist
    set save_count = (
      select count(*)
      from saves r
      where
          r.is_current is true
          and r.is_delete is false
          and r.save_type = new.save_type
          and r.save_item_id = new.save_item_id
    )
    where playlist_id = new.save_item_id
    returning save_count into new_val;

    if new.is_delete IS FALSE then
		  select playlists.playlist_owner_id into owner_user_id from playlists where is_current and playlist_id = new.save_item_id;
	  end if;

  end if;

  -- create a milestone if applicable
  select new_val into milestone where new_val in (10,25,50,100,250,500,1000,2500,5000,10000,25000,50000,100000,250000,500000,1000000);
  select score < 0 into is_shadowbanned from aggregate_user where user_id = new.user_id;

  if new.is_delete = false and milestone is not null and is_shadowbanned = false then
    insert into milestones 
      (id, name, threshold, blocknumber, slot, timestamp)
    values
      (new.save_item_id, milestone_name, milestone, new.blocknumber, new.slot, new.created_at)
    on conflict do nothing;

    if entity_type = 'track' then
      insert into notification
      (user_ids, type, specifier, group_id, blocknumber, timestamp, data)
      values
      (
        ARRAY [owner_user_id],
        'milestone',
        owner_user_id,
        'milestone:' || milestone_name  || ':id:' || new.save_item_id || ':threshold:' || milestone,
        new.blocknumber,
        new.created_at,
        json_build_object('type', milestone_name, 'track_id', new.save_item_id, 'threshold', milestone)
      )
      on conflict do nothing;
    else
      insert into notification
        (user_ids, type, specifier, group_id, blocknumber, timestamp, data)
        values
        (
          ARRAY [owner_user_id],
          'milestone',
          owner_user_id,
          'milestone:' || milestone_name  || ':id:' || new.save_item_id || ':threshold:' || milestone,
          new.blocknumber,
          new.created_at,
          json_build_object('type', milestone_name, 'playlist_id', new.save_item_id, 'threshold', milestone, 'is_album', is_album)
        )
        on conflict do nothing;
    end if;
  end if;

  begin
    -- create a notification for the saved content's owner
    -- skip notification for purchased content as the purchase triggers its own notification
    if new.is_delete is false and is_purchased is false and is_shadowbanned = false then
      insert into notification
        (blocknumber, user_ids, timestamp, type, specifier, group_id, data)
        values
        ( 
          new.blocknumber,
          ARRAY [owner_user_id], 
          new.created_at, 
          'save',
          new.user_id,
          'save:' || new.save_item_id || ':type:'|| new.save_type,
          json_build_object('save_item_id', new.save_item_id, 'user_id', new.user_id, 'type', new.save_type)
        )
      on conflict do nothing;
    end if;

    -- notify followees of the favoriter who have reposted the same content
    -- within the last month
    if new.is_delete is false
    and new.is_save_of_repost is true
    -- skip notification for tracks contained within a purchased album
    -- the favorite of the album itself will still trigger this notification
    and is_shadowbanned = false
    and is_containing_album_purchased is false then
    with
        followee_save_repost_ids as (
            select user_id
            from reposts r
            where
                r.repost_item_id = new.save_item_id
                and new.created_at - INTERVAL '1 month' < r.created_at
                and new.created_at > r.created_at
                and r.is_delete is false
                and r.is_current is true
                and r.repost_type::text = new.save_type::text
                and r.user_id in
                (
                    select
                        followee_user_id
                    from follows
                    where
                        follower_user_id = new.user_id
                        and is_delete is false
                        and is_current is true
                )
        )
    insert into notification
      (blocknumber, user_ids, timestamp, type, specifier, group_id, data)
      SELECT blocknumber_val, user_ids_val, timestamp_val, type_val, specifier_val, group_id_val, data_val
      FROM (
        SELECT new.blocknumber AS blocknumber_val,
        ARRAY(
          SELECT user_id
          FROM
            followee_save_repost_ids
        ) AS user_ids_val,
        new.created_at AS timestamp_val,
        'save_of_repost' AS type_val,
        new.user_id AS specifier_val,
        'save_of_repost:' || new.save_item_id || ':type:' || new.save_type AS group_id_val,
        json_build_object(
          'save_of_repost_item_id',
          new.save_item_id,
          'user_id',
          new.user_id,
          'type',
          case 
            when is_album then 'album'
            else new.save_type
          end
        ) AS data_val
      ) sub
      WHERE user_ids_val IS NOT NULL AND array_length(user_ids_val, 1) > 0
      on conflict do nothing;
    end if;

    -- create a notification for remix cosign
    if new.is_delete is false and new.save_type = 'track' and track_remix_of is not null and is_shadowbanned = false then
      select
        case when tracks.owner_id = new.user_id then TRUE else FALSE end as boolean into is_remix_cosign
        from tracks 
        where is_current and track_id = (track_remix_of->'tracks'->0->>'parent_track_id')::int;
      if is_remix_cosign then
        insert into notification
          (blocknumber, user_ids, timestamp, type, specifier, group_id, data)
          values
          ( 
            new.blocknumber,
            ARRAY [owner_user_id], 
            new.created_at, 
            'cosign',
            new.user_id,
            'cosign:parent_track' || (track_remix_of->'tracks'->0->>'parent_track_id')::int || ':original_track:'|| new.save_item_id,
            json_build_object('parent_track_id', (track_remix_of->'tracks'->0->>'parent_track_id')::int, 'track_id', new.save_item_id, 'track_owner_id', owner_user_id)
          )
        on conflict do nothing;
      end if;
    end if;
  exception
    when others then
      raise warning 'An error occurred in %: %', tg_name, sqlerrm;
      return null;
  end;

  return null;

exception
    when others then
      raise warning 'An error occurred in %: %', tg_name, sqlerrm;
      raise;

end; 
$$;


--
-- Name: handle_share(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.handle_share() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
begin
  -- Ensure aggregate_user exists for this user
  insert into aggregate_user (user_id) values (new.user_id) on conflict do nothing;

  if new.share_type = 'track' then
    -- Ensure aggregate_track exists for this track
    insert into aggregate_track (track_id) values (new.share_item_id) on conflict do nothing;
  else
    -- Ensure aggregate_playlist exists for this playlist
    insert into aggregate_playlist (playlist_id, is_album)
    select p.playlist_id, p.is_album
    from playlists p
    where p.playlist_id = new.share_item_id
    and p.is_current
    on conflict do nothing;
  end if;

  -- Update aggregate statistics for tracks
  if new.share_type = 'track' then
    update aggregate_track
    set share_count = (
      select count(*)
      from shares s
      where s.share_type = new.share_type
        and s.share_item_id = new.share_item_id
    )
    where track_id = new.share_item_id;

    -- Update user's track share count
    update aggregate_user
    set track_share_count = (
      select count(*)
      from shares s
      where s.user_id = new.user_id
        and s.share_type = new.share_type
    )
    where user_id = new.user_id;
  else
    -- Update aggregate statistics for playlists/albums
    update aggregate_playlist
    set share_count = (
      select count(*)
      from shares s
      where s.share_type = new.share_type
        and s.share_item_id = new.share_item_id
    )
    where playlist_id = new.share_item_id;
  end if;

  return null;

exception
    when others then
      raise warning 'An error occurred in %: %', tg_name, sqlerrm;
      return null;
end;
$$;


--
-- Name: handle_sol_claimable_accounts(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.handle_sol_claimable_accounts() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE
    v_user_id int;
BEGIN
    FOR v_user_id IN
        SELECT user_id
        FROM users
        WHERE users.wallet = NEW.ethereum_address
    LOOP
        PERFORM update_sol_user_balance_mint(v_user_id, NEW.mint);
    END LOOP;

    RETURN NULL;
EXCEPTION
    WHEN OTHERS THEN
        RAISE WARNING 'An error occurred in %: %', TG_NAME, SQLERRM;
        RETURN NULL;
END;
$$;


--
-- Name: handle_sol_token_balance_change(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.handle_sol_token_balance_change() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE
    v_user_id int;
BEGIN
    INSERT INTO sol_token_account_balances (account, mint, owner, balance, slot, updated_at)
    VALUES (NEW.account, NEW.mint, NEW.owner, NEW.balance, NEW.slot, NOW())
    ON CONFLICT (account)
    DO UPDATE SET
        balance = EXCLUDED.balance,
        slot = EXCLUDED.slot,
        updated_at = NOW()
        WHERE sol_token_account_balances.slot < EXCLUDED.slot;
    
    FOR v_user_id IN
        SELECT user_id
        FROM associated_wallets
        WHERE wallet = NEW.owner
          AND chain = 'sol'
        UNION ALL
        SELECT user_id
        FROM users
        JOIN sol_claimable_accounts ON sol_claimable_accounts.ethereum_address = users.wallet
        WHERE sol_claimable_accounts.account = NEW.account
          AND sol_claimable_accounts.mint = NEW.mint
    LOOP
        PERFORM update_sol_user_balance_mint(v_user_id, NEW.mint);
    END LOOP;

    RETURN NULL;
EXCEPTION
    WHEN OTHERS THEN
        RAISE WARNING 'An error occurred in %: %', TG_NAME, SQLERRM;
        RETURN NULL;
END;
$$;


--
-- Name: handle_supporter_rank_up(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.handle_supporter_rank_up() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
declare
  user_bank_tx user_bank_txs%ROWTYPE;
  dethroned_user_id int;
begin
  select * into user_bank_tx from user_bank_txs where user_bank_txs.slot = new.slot limit 1;

  if user_bank_tx is not null then
    -- create a notification for the sender and receiver
    insert into notification
      (slot, user_ids, timestamp, type, specifier, group_id, data, type_v2)
    values
      (
      -- supporting_rank_up notifs are sent to the sender of the tip
        new.slot,
        ARRAY [new.sender_user_id],
        user_bank_tx.created_at,
        'supporting_rank_up',
        new.sender_user_id,
        'supporting_rank_up:' || new.rank || ':slot:' || new.slot,
        json_build_object('sender_user_id', new.sender_user_id, 'receiver_user_id', new.receiver_user_id, 'rank', new.rank),
        'supporting_rank_up'
      ),
      (
      -- supporter_rank_up notifs are sent to the receiver of the tip
        new.slot,
        ARRAY [new.receiver_user_id],
        user_bank_tx.created_at,
        'supporter_rank_up',
        new.receiver_user_id,
        'supporter_rank_up:' || new.rank || ':slot:' || new.slot,
        json_build_object('sender_user_id', new.sender_user_id, 'receiver_user_id', new.receiver_user_id, 'rank', new.rank),
        'supporter_rank_up'
      )
    on conflict do nothing;

    if new.rank = 1 then
      select sender_user_id into dethroned_user_id from supporter_rank_ups where rank=1 and receiver_user_id=new.receiver_user_id and slot < new.slot order by slot desc limit 1;
      if dethroned_user_id is not NULL then
        -- create a notification for the sender and receiver
        insert into notification
          (slot, user_ids, timestamp, type, specifier, group_id, data, type_v2)
        values
          (
            new.slot,
            ARRAY [dethroned_user_id],
            user_bank_tx.created_at,
            'supporter_dethroned',
            new.sender_user_id,
            'supporter_dethroned:receiver_user_id:' || new.receiver_user_id || ':slot:' || new.slot,
            json_build_object('sender_user_id', new.sender_user_id, 'receiver_user_id', new.receiver_user_id, 'dethroned_user_id', dethroned_user_id),
            'supporter_dethroned'
          ) on conflict do nothing;

      end if;
    end if;

  end if;
  return null;

exception
  when others then
    raise warning 'An error occurred in %: %', tg_name, sqlerrm;
    return null;
end;
$$;


--
-- Name: handle_track(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.handle_track() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
declare
  parent_track_owner_id int;
  subscriber_user_ids int[];
begin
  insert into aggregate_track (track_id) values (new.track_id) on conflict do nothing;
  insert into aggregate_user (user_id) values (new.owner_id) on conflict do nothing;

  update aggregate_user
  set (track_count, total_track_count) = (
    select
      count(*) filter (where t.is_unlisted = false),
      count(*)
    from tracks t
    where t.is_current is true
      and t.is_delete is false
      and t.is_available is true
      and t.stem_of is null
      and t.owner_id = new.owner_id
  )
  where user_id = new.owner_id
  ;

  -- If new track or newly unlisted track, create notification
  begin
    if track_should_notify(OLD, new, TG_OP) AND new.is_playlist_upload = FALSE THEN
      select array(
        select subscriber_id
          from subscriptions
          where is_current and
          not is_delete and
          user_id=new.owner_id
      ) into subscriber_user_ids;

      if array_length(subscriber_user_ids, 1)	> 0 then
        insert into notification
          (blocknumber, user_ids, timestamp, type, specifier, group_id, data)
        values
        (
          new.blocknumber,
          subscriber_user_ids,
          new.updated_at,
          'create',
          new.track_id,
          'create:track:user_id:' || new.owner_id,
          json_build_object('track_id', new.track_id)
        )
        on conflict do nothing;
      end if;
    end if;
	exception
    when others then
      raise warning 'An error occurred in %: %', tg_name, sqlerrm;
  end;

  -- If new remix or newly unlisted remix, create notification
  begin
    if track_should_notify(OLD, new, TG_OP) AND new.remix_of is not null THEN
      select owner_id into parent_track_owner_id from tracks where is_current and track_id = (new.remix_of->'tracks'->0->>'parent_track_id')::int limit 1;
      if parent_track_owner_id is not null then
        insert into notification
        (blocknumber, user_ids, timestamp, type, specifier, group_id, data)
        values
        (
          new.blocknumber,
          ARRAY [parent_track_owner_id],
          new.updated_at,
          'remix',
          new.owner_id,
          'remix:track:' || new.track_id || ':parent_track:' || (new.remix_of->'tracks'->0->>'parent_track_id')::int,
          json_build_object('track_id', new.track_id, 'parent_track_id', (new.remix_of->'tracks'->0->>'parent_track_id')::int)
        )
        on conflict do nothing;
      end if;
    end if;
	exception
    when others then
      raise warning 'An error occurred in %: %', tg_name, sqlerrm;
  end;

  -- If new remix is a submission to an active remix contest, check for milestone notifications
  begin
    if track_should_notify(OLD, new, TG_OP) AND new.remix_of is not null THEN
      declare
        contest_event_id int;
        contest_creator_id int;
        submission_count int;
        milestone int;
        parent_track_id int := (new.remix_of->'tracks'->0->>'parent_track_id')::int;
      begin
        select event_id, user_id
        into contest_event_id, contest_creator_id
        from events
        where event_type = 'remix_contest'
          and is_deleted = false
          and end_date > now()
          and entity_id = parent_track_id
        limit 1;

        if contest_event_id is not null then
          -- Count submissions for this contest (only those after contest start)
          select count(*) into submission_count
          from tracks t
          join events e on e.event_type = 'remix_contest'
            and e.is_deleted = false
            and e.entity_id = parent_track_id
          where t.is_current = true
            and t.is_delete = false
            and t.remix_of is not null
            and (t.remix_of->'tracks'->0->>'parent_track_id')::int = parent_track_id
            and t.created_at >= e.created_at;

          -- For each milestone, insert notification if this is the Nth submission
          FOREACH milestone IN ARRAY ARRAY[1, 10, 50] LOOP
            IF submission_count = milestone THEN
              insert into notification
                (blocknumber, user_ids, timestamp, type, specifier, group_id, data)
              values
                (
                  new.blocknumber,
                  ARRAY [contest_creator_id],
                  new.updated_at,
                  'artist_remix_contest_submissions',
                  milestone || ':' || contest_event_id,
                  'artist_remix_contest_submissions:' || contest_event_id || ':' || milestone,
                  json_build_object(
                    'event_id', contest_event_id,
                    'milestone', milestone,
                    'entity_id', parent_track_id
                  )
                )
              on conflict do nothing;
            END IF;
          END LOOP;
        end if;
      end;
    end if;
    exception
      when others then
        raise warning 'An error occurred in %: %', tg_name, sqlerrm;
  end;

  return null;

exception
    when others then
      raise warning 'An error occurred in %: %', tg_name, sqlerrm;
      raise;

end;
$$;


--
-- Name: handle_usdc_purchase(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.handle_usdc_purchase() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
begin

  -- insert seller/artist notification
  INSERT INTO notification
          (slot, user_ids, timestamp, type, specifier, group_id, data)
        VALUES
          (
            new.slot,
            ARRAY [new.seller_user_id],
            new.created_at,
            'usdc_purchase_seller',
            new.buyer_user_id,
            'usdc_purchase_seller:' || 'seller_user_id:' || new.seller_user_id || ':buyer_user_id:' || new.buyer_user_id || ':content_id:' || new.content_id || ':content_type:' || new.content_type,
            json_build_object(
                'content_type', new.content_type,
                'buyer_user_id', new.buyer_user_id,
                'seller_user_id', new.seller_user_id,
                'amount', new.amount,
                'extra_amount', new.extra_amount,
                'content_id', new.content_id,
                'vendor', new.vendor
              )
          ),
          (
            new.slot,
            ARRAY [new.buyer_user_id],
            new.created_at,
            'usdc_purchase_buyer',
            new.buyer_user_id,
            'usdc_purchase_buyer:' || 'seller_user_id:' || new.seller_user_id || ':buyer_user_id:' || new.buyer_user_id || ':content_id:' || new.content_id || ':content_type:' || new.content_type,
            json_build_object(
                'content_type', new.content_type,
                'buyer_user_id', new.buyer_user_id,
                'seller_user_id', new.seller_user_id,
                'amount', new.amount,
                'extra_amount', new.extra_amount,
                'content_id', new.content_id,
                'vendor', new.vendor
            )
          )
        on conflict do nothing;

  return null;
  exception
    when others then
        raise warning 'An error occurred in %: %', tg_name, sqlerrm;
        return null;
end; 
$$;


--
-- Name: handle_usdc_withdrawal(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.handle_usdc_withdrawal() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE
    users_row users%ROWTYPE;
    notification_type varchar;
begin

  if new.transaction_type in ('transfer', 'withdrawal') and new.method = 'send' then
    notification_type := 'usdc_' || new.transaction_type;
    -- Fetch the corresponding user based on the wallet
    select into users_row users.*
    from users
    join usdc_user_bank_accounts
      on users.wallet = usdc_user_bank_accounts.ethereum_address
    where usdc_user_bank_accounts.bank_account = new.user_bank;

    -- Insert the new notification
    insert into notification
      (slot, user_ids, timestamp, type, specifier, group_id, data)
    values
      (
        new.slot,
        ARRAY [users_row.user_id],
        new.created_at,
        notification_type,
        users_row.user_id,
        notification_type || ':' || users_row.user_id || ':' || 'signature:' || new.signature,
        json_build_object(
          'user_id', users_row.user_id,
          'user_bank', new.user_bank,
          'signature', new.signature,
          'change', new.change,
          'balance', new.balance,
          'receiver_account', new.tx_metadata
        )
      )
      on conflict do nothing;
  end if;

  return null;
  exception
    when others then
        raise warning 'An error occurred in %: %', tg_name, sqlerrm;
        return null;

end;
$$;


--
-- Name: handle_user(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.handle_user() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
declare
begin
  insert into aggregate_user (user_id) values (new.user_id) on conflict do nothing;
  return null;

exception
  when others then
    raise warning 'An error occurred in %: %', tg_name, sqlerrm;
    raise;

end;
$$;


--
-- Name: handle_user_balance_change(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.handle_user_balance_change() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
declare
  new_val int;
  new_tier text;
  new_tier_value int;
  previous_tier text;
  previous_tier_value int;
begin
  SELECT label, val into new_tier, new_tier_value
  FROM (
    VALUES ('bronze', 10::bigint), ('silver', 100::bigint), ('gold', 1000::bigint), ('platinum', 10000::bigint)
  ) as tier (label, val)
  WHERE
    substr(new.current_balance, 1, GREATEST(1, length(new.current_balance) - 18))::bigint >= tier.val
  ORDER BY 
    tier.val DESC
  limit 1;

  SELECT label, val into previous_tier, previous_tier_value
  FROM (
    VALUES ('bronze', 10::bigint), ('silver', 100::bigint), ('gold', 1000::bigint), ('platinum', 10000::bigint)
  ) as tier (label, val)
  WHERE
    substr(new.previous_balance, 1, GREATEST(1, length(new.previous_balance) - 18))::bigint >= tier.val
  ORDER BY 
    tier.val DESC
  limit 1;

  -- create a notification if the tier changes
  if new_tier_value > previous_tier_value or (new_tier_value is not NULL and previous_tier_value is NULL) then
    insert into notification
      (blocknumber, user_ids, timestamp, type, specifier, group_id, data)
    values
      ( 
        new.blocknumber,
        ARRAY [new.user_id], 
        new.updated_at, 
        'tier_change',
        new.user_id,
        'tier_change:user_id:' || new.user_id ||  ':tier:' || new_tier || ':blocknumber:' || new.blocknumber,
        json_build_object(
          'new_tier', new_tier,
          'new_tier_value', new_tier_value,
          'current_value', new.current_balance
        )
      )
    on conflict do nothing;
    return null;
  end if;

  return null;
exception
  when others then
    raise warning 'An error occurred in %: %', tg_name, sqlerrm;
    return null;
end;
$$;


--
-- Name: handle_user_tip(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.handle_user_tip() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
begin

  -- create a notification for the sender and receiver
  insert into notification
    (slot, user_ids, timestamp, type, specifier, group_id, data)
  values
    ( 
      new.slot,
      ARRAY [new.receiver_user_id], 
      new.created_at, 
      'tip_receive',
      new.receiver_user_id,
      'tip_receive:user_id:' || new.receiver_user_id || ':signature:' || new.signature,
      json_build_object(
        'sender_user_id', new.sender_user_id,
        'receiver_user_id', new.receiver_user_id,
        'amount', new.amount,
        'tx_signature', new.signature
      )
    ),
    ( 
      new.slot,
      ARRAY [new.sender_user_id], 
      new.created_at, 
      'tip_send',
      new.sender_user_id,
      'tip_send:user_id:' || new.sender_user_id || ':signature:' || new.signature,
      json_build_object(
        'sender_user_id', new.sender_user_id,
        'receiver_user_id', new.receiver_user_id,
        'amount', new.amount,
        'tx_signature', new.signature
      )
    )
    on conflict do nothing;

return null;

exception
  when others then
    raise warning 'An error occurred in %: %', tg_name, sqlerrm;
    return null;

end;
$$;


--
-- Name: id_decode(text); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.id_decode(p_id text) RETURNS integer
    LANGUAGE plpgsql IMMUTABLE COST 300
    AS $$
begin
  return (hashids.decode(p_id, 'azowernasdfoia', 5))[1]::integer;
end;
$$;


--
-- Name: id_encode(integer); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.id_encode(p_id integer) RETURNS text
    LANGUAGE plpgsql IMMUTABLE COST 300
    AS $$
begin
  return hashids.encode(p_id, 'azowernasdfoia', 5);
end;
$$;


--
-- Name: is_country_eur(text); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.is_country_eur(country text) RETURNS boolean
    LANGUAGE plpgsql
    AS $$
BEGIN
    RETURN
        country = 'Afghanistan' OR
        country = 'Albania' OR
        country = 'Algeria' OR
        country = 'American Samoa' OR
        country = 'Andorra' OR
        country = 'Angola' OR
        country = 'Antigua and Barbuda' OR
        country = 'Arab Emirates' OR
        country = 'Armenia' OR
        country = 'Aruba' OR
        country = 'Australia' OR
        country = 'Austria' OR
        country = 'Azerbaijan' OR
        country = 'Bahamas' OR
        country = 'Bahrain' OR
        country = 'Bangladesh' OR
        country = 'Barbados' OR
        country = 'Belarus' OR
        country = 'Belgium' OR
        country = 'Belize' OR
        country = 'Benin' OR
        country = 'Bermuda' OR
        country = 'Bhutan' OR
        country = 'Bolivia' OR
        country = 'Bosnia and Herzegovina' OR
        country = 'Botswana' OR
        country = 'Brunei' OR
        country = 'Bulgaria' OR
        country = 'Burkina Faso' OR
        country = 'Burma' OR
        country = 'Burundi' OR
        country = 'Cambodia' OR
        country = 'Cameroon' OR
        country = 'Cape Verde' OR
        country = 'Cayman Islands' OR
        country = 'Central African Republic' OR
        country = 'Chad' OR
        country = 'Channel Islands' OR
        country = 'Chile' OR
        country = 'China' OR
        country = 'Colombia' OR
        country = 'Comoros' OR
        country = 'Congo' OR
        country = 'Costa Rica' OR
        country = 'Cote d''Ivoire' OR
        country = 'Côte d''Ivoire' OR
        country = 'Croatia' OR
        country = 'Cuba' OR
        country = 'Curacao' OR
        country = 'Cyprus' OR
        country = 'Czech Republic' OR
        country = 'Czechia' OR
        country = 'Democratic People''s Republic of Korea' OR
        country = 'Democratic Republic of the Congo' OR
        country = 'Denmark' OR
        country = 'Djibouti' OR
        country = 'Dominica' OR
        country = 'Dominican Republic' OR
        country = 'East Timor' OR
        country = 'Ecuador' OR
        country = 'Egypt' OR
        country = 'El Salvador' OR
        country = 'Equatorial Guinea' OR
        country = 'Eritrea' OR
        country = 'Estonia' OR
        country = 'Eswatini' OR
        country = 'Ethiopia' OR
        country = 'Faroe Islands' OR
        country = 'Fiji' OR
        country = 'Finland' OR
        country = 'France' OR
        country = 'French Polynesia' OR
        country = 'Gabon' OR
        country = 'Gambia' OR
        country = 'Gambia' OR
        country = 'Georgia' OR
        country = 'Germany' OR
        country = 'Ghana' OR
        country = 'Gibraltar' OR
        country = 'Greece' OR
        country = 'Guernsey' OR
        country = 'Guinea-Bissau' OR
        country = 'Guinea' OR
        country = 'Holy See' OR
        country = 'Hong Kong' OR
        country = 'Hungary' OR
        country = 'Iceland' OR
        country = 'India' OR
        country = 'Indonesia' OR
        country = 'Iran' OR
        country = 'Iraq' OR
        country = 'Ireland' OR
        country = 'Israel' OR
        country = 'Italy' OR
        country = 'Ivory Coast' OR
        country = 'Japan' OR
        country = 'Jersey' OR
        country = 'Jordan' OR
        country = 'Kazakhstan' OR
        country = 'Kenya' OR
        country = 'Kiribati' OR
        country = 'Kosovo' OR
        country = 'Kuwait' OR
        country = 'Kyrgyzstan' OR
        country = 'Laos' OR
        country = 'Latvia' OR
        country = 'Lebanon' OR
        country = 'Lesotho' OR
        country = 'Liberia' OR
        country = 'Libya' OR
        country = 'Liechtenstein' OR
        country = 'Lithuania' OR
        country = 'Lituania' OR
        country = 'Luxembourg' OR
        country = 'Macau' OR
        country = 'Macedonia' OR
        country = 'Madagascar' OR
        country = 'Malawi' OR
        country = 'Malaysia' OR
        country = 'Maldives' OR
        country = 'Mali' OR
        country = 'Malta' OR
        country = 'Marshall Islands' OR
        country = 'Mauritania' OR
        country = 'Mauritius' OR
        country = 'Micronesia' OR
        country = 'Monaco' OR
        country = 'Mongolia' OR
        country = 'Montenegro' OR
        country = 'Morocco' OR
        country = 'Mozambique' OR
        country = 'Myanmar' OR
        country = 'Namibia' OR
        country = 'Nauru' OR
        country = 'Nepal' OR
        country = 'Netherlands' OR
        country = 'New Zealand' OR
        country = 'Niger' OR
        country = 'Nigeria' OR
        country = 'Niue' OR
        country = 'North Korea' OR
        country = 'North Macedonia' OR
        country = 'Norway' OR
        country = 'Oman' OR
        country = 'Pakistan' OR
        country = 'Palau' OR
        country = 'Palestine' OR
        country = 'Palestinian Territory' OR
        country = 'Papua New Guinea' OR
        country = 'Paraguay' OR
        country = 'Peru' OR
        country = 'Philippines' OR
        country = 'Poland' OR
        country = 'Portugal' OR
        country = 'Qatar' OR
        country = 'Reunion' OR
        country = 'Romania' OR
        country = 'Rwanda' OR
        country = 'Samoa' OR
        country = 'San Marino' OR
        country = 'Sao Tome and Principe' OR
        country = 'Saudi Arabia' OR
        country = 'Senegal' OR
        country = 'Serbia' OR
        country = 'Seychelles' OR
        country = 'Sierra Leone' OR
        country = 'Singapore' OR
        country = 'Slovakia' OR
        country = 'Slovenia' OR
        country = 'Solomon Islands' OR
        country = 'Somalia' OR
        country = 'South Africa' OR
        country = 'South Korea' OR
        country = 'South Sudan' OR
        country = 'Spain' OR
        country = 'Sri Lanka' OR
        country = 'Sudan' OR
        country = 'Suriname' OR
        country = 'Swaziland' OR
        country = 'Sweden' OR
        country = 'Switzerland' OR
        country = 'Syria' OR
        country = 'Taiwan' OR
        country = 'Tajikistan' OR
        country = 'Tanzania' OR
        country = 'Thailand' OR
        country = 'Timor Leste' OR
        country = 'Togo' OR
        country = 'Tonga' OR
        country = 'Trinidad and Tobago' OR
        country = 'Tunisia' OR
        country = 'Turkey' OR
        country = 'Turkmenistan' OR
        country = 'Turks and Caicos Islands' OR
        country = 'Tuvalu' OR
        country = 'Uganda' OR
        country = 'Ukraine' OR
        country = 'United Arab Emirates' OR
        country = 'United Kingdom' OR
        country = 'United States' OR
        country = 'Uruguay' OR
        country = 'Uzbekistan' OR
        country = 'Vanuatu' OR
        country = 'Vatican City' OR
        country = 'Venezuela' OR
        country = 'Vietnam' OR
        country = 'Virgin Islands' OR
        country = 'Western Sahara' OR
        country = 'Yemen' OR
        country = 'Zambia' OR
        country = 'Zimbabwe' OR

        -- country codes
        country = 'AD' OR
        country = 'AE' OR
        country = 'AF' OR
        country = 'AG' OR
        country = 'AL' OR
        country = 'AM' OR
        country = 'AO' OR
        country = 'AS' OR
        country = 'AT' OR
        country = 'AU' OR
        country = 'AW' OR
        country = 'AZ' OR
        country = 'BA' OR
        country = 'BB' OR
        country = 'BD' OR
        country = 'BE' OR
        country = 'BF' OR
        country = 'BG' OR
        country = 'BH' OR
        country = 'BI' OR
        country = 'BJ' OR
        country = 'BM' OR
        country = 'BN' OR
        country = 'BO' OR
        country = 'BS' OR
        country = 'BT' OR
        country = 'BW' OR
        country = 'BY' OR
        country = 'BZ' OR
        country = 'CF' OR
        country = 'CG' OR
        country = 'CH' OR
        country = 'CI' OR
        country = 'CL' OR
        country = 'CM' OR
        country = 'CN' OR
        country = 'CO' OR
        country = 'CR' OR
        country = 'CU' OR
        country = 'CV' OR
        country = 'CW' OR
        country = 'CY' OR
        country = 'CZ' OR
        country = 'DE' OR
        country = 'DJ' OR
        country = 'DK' OR
        country = 'DM' OR
        country = 'DO' OR
        country = 'DZ' OR
        country = 'EC' OR
        country = 'EE' OR
        country = 'EG' OR
        country = 'EH' OR
        country = 'ER' OR
        country = 'ES' OR
        country = 'ET' OR
        country = 'FI' OR
        country = 'FJ' OR
        country = 'FO' OR
        country = 'FR' OR
        country = 'GA' OR
        country = 'GB' OR
        country = 'GD' OR
        country = 'GE' OR
        country = 'GG' OR
        country = 'GH' OR
        country = 'GI' OR
        country = 'GM' OR
        country = 'GN' OR
        country = 'GQ' OR
        country = 'GR' OR
        country = 'GT' OR
        country = 'GU' OR
        country = 'GW' OR
        country = 'GY' OR
        country = 'HK' OR
        country = 'HN' OR
        country = 'HR' OR
        country = 'HT' OR
        country = 'HU' OR
        country = 'ID' OR
        country = 'IE' OR
        country = 'IL' OR
        country = 'IM' OR
        country = 'IN' OR
        country = 'IQ' OR
        country = 'IS' OR
        country = 'IT' OR
        country = 'JE' OR
        country = 'JM' OR
        country = 'JO' OR
        country = 'JP' OR
        country = 'KE' OR
        country = 'KG' OR
        country = 'KH' OR
        country = 'KI' OR
        country = 'KM' OR
        country = 'KN' OR
        country = 'KP' OR
        country = 'KR' OR
        country = 'KW' OR
        country = 'KY' OR
        country = 'KZ' OR
        country = 'LA' OR
        country = 'LB' OR
        country = 'LC' OR
        country = 'LI' OR
        country = 'LK' OR
        country = 'LR' OR
        country = 'LS' OR
        country = 'LT' OR
        country = 'LU' OR
        country = 'LV' OR
        country = 'LY' OR
        country = 'MA' OR
        country = 'MC' OR
        country = 'MD' OR
        country = 'ME' OR
        country = 'MG' OR
        country = 'MH' OR
        country = 'MK' OR
        country = 'ML' OR
        country = 'MM' OR
        country = 'MN' OR
        country = 'MO' OR
        country = 'MR' OR
        country = 'MS' OR
        country = 'MT' OR
        country = 'MU' OR
        country = 'MV' OR
        country = 'MW' OR
        country = 'MX' OR
        country = 'MY' OR
        country = 'MZ' OR
        country = 'NA' OR
        country = 'NE' OR
        country = 'NG' OR
        country = 'NI' OR
        country = 'NL' OR
        country = 'NO' OR
        country = 'NP' OR
        country = 'NR' OR
        country = 'NU' OR
        country = 'NZ' OR
        country = 'OM' OR
        country = 'PA' OR
        country = 'PE' OR
        country = 'PF' OR
        country = 'PG' OR
        country = 'PH' OR
        country = 'PK' OR
        country = 'PL' OR
        country = 'PM' OR
        country = 'PN' OR
        country = 'PR' OR
        country = 'PS' OR
        country = 'PT' OR
        country = 'PW' OR
        country = 'PY' OR
        country = 'QA' OR
        country = 'RE' OR
        country = 'RO' OR
        country = 'RS' OR
        country = 'RW' OR
        country = 'SA' OR
        country = 'SB' OR
        country = 'SC' OR
        country = 'SD' OR
        country = 'SE' OR
        country = 'SG' OR
        country = 'SI' OR
        country = 'SK' OR
        country = 'SL' OR
        country = 'SM' OR
        country = 'SN' OR
        country = 'SO' OR
        country = 'SR' OR
        country = 'SS' OR
        country = 'ST' OR
        country = 'SV' OR
        country = 'SX' OR
        country = 'SY' OR
        country = 'SZ' OR
        country = 'TC' OR
        country = 'TD' OR
        country = 'TG' OR
        country = 'TH' OR
        country = 'TJ' OR
        country = 'TK' OR
        country = 'TL' OR
        country = 'TM' OR
        country = 'TN' OR
        country = 'TO' OR
        country = 'TR' OR
        country = 'TT' OR
        country = 'TV' OR
        country = 'TW' OR
        country = 'TZ' OR
        country = 'UA' OR
        country = 'UG' OR
        country = 'US' OR
        country = 'UY' OR
        country = 'UZ' OR
        country = 'VA' OR
        country = 'VC' OR
        country = 'VE' OR
        country = 'VG' OR
        country = 'VI' OR
        country = 'VN' OR
        country = 'VU' OR
        country = 'WS' OR
        country = 'XK' OR
        country = 'YE' OR
        country = 'ZA' OR
        country = 'ZM' OR
        country = 'ZW'
        ;
END;
$$;


--
-- Name: log_message(text); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.log_message(message_text text) RETURNS void
    LANGUAGE plpgsql
    AS $$
BEGIN
    RAISE NOTICE '% %', pg_backend_pid(), message_text;
END;
$$;


--
-- Name: on_new_notification_row(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.on_new_notification_row() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
begin
  PERFORM pg_notify(TG_TABLE_NAME, json_build_object('notification_id', new.id)::text);
  return null;
end; 
$$;


--
-- Name: on_new_notification_seen_row(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.on_new_notification_seen_row() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
begin
  PERFORM pg_notify(TG_TABLE_NAME, json_build_object('user_id', new.user_id)::text);
  return null;
end; 
$$;


--
-- Name: on_new_row(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.on_new_row() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
begin
  case TG_TABLE_NAME
    when 'tracks' then
      PERFORM pg_notify(TG_TABLE_NAME, json_build_object('track_id', new.track_id, 'updated_at', new.updated_at, 'created_at', new.created_at, 'blocknumber', new.blocknumber)::text);
    when 'users' then
      PERFORM pg_notify(TG_TABLE_NAME, json_build_object('user_id', new.user_id, 'blocknumber', new.blocknumber)::text);
    when 'playlists' then
      PERFORM pg_notify(TG_TABLE_NAME, json_build_object('playlist_id', new.playlist_id)::text);
    else
      PERFORM pg_notify(TG_TABLE_NAME, to_json(new)::text);
  end case;
  return null;
end;
$$;


--
-- Name: process_grant_change(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.process_grant_change() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
declare
    matched_user_id integer;
begin
    -- fetch the user_id where wallet matches grantee_address
    select user_id into matched_user_id from users where lower(wallet) = lower(NEW.grantee_address);
    
    if matched_user_id is not null then
        -- if the grant is newly created (i.e. the grant is not deleted, is not approved yet, and was just created indicated by created timestamp = last updated timestamp) OR grant went from deleted (revoked) to not deleted and is not approved yet...
        if (TG_OP = 'INSERT' and NEW.is_revoked = FALSE and NEW.is_approved is null and NEW.created_at = NEW.updated_at or
            (TG_OP = 'UPDATE' and NEW.is_revoked = FALSE and OLD.is_revoked = TRUE and NEW.is_approved is null))
        then
            -- ... create a "request_manager" notification
            insert into notification
                    (blocknumber, user_ids, timestamp, type, specifier, group_id, data)
                  values
                    (
                      new.blocknumber,
                      array [matched_user_id],
                      new.updated_at,
                      'request_manager',
                      new.user_id,
                      'request_manager:' || 'grantee_user_id:' || matched_user_id || ':grantee_address:' || new.grantee_address || ':user_id:' || new.user_id || ':updated_at:' || new.updated_at ||
                      ':created_at:' || new.created_at,
                      json_build_object(
                          'grantee_user_id', matched_user_id,
                          'grantee_address', new.grantee_address,
                          'user_id', new.user_id
                        )
                    )
                  on conflict do nothing;
        -- otherwise, if the grant is approved and not deleted (revoked)...
        elsif (TG_OP = 'INSERT' and NEW.is_approved = TRUE and NEW.is_revoked = FALSE) or
            (TG_OP = 'UPDATE' and NEW.is_approved = TRUE and (OLD.is_approved != TRUE) and NEW.is_revoked = FALSE)
        then
            -- ... create a "approve_manager_request" notification
            insert into notification
                    (blocknumber, user_ids, timestamp, type, specifier, group_id, data)
                  values
                    (
                      new.blocknumber,
                      array [new.user_id],
                      new.updated_at,
                      'approve_manager_request',
                      matched_user_id,
                      'approve_manager_request:' || 'grantee_user_id:' || matched_user_id || ':grantee_address:' || new.grantee_address || ':user_id:' || new.user_id || ':created_at:' || new.created_at,
                      json_build_object(
                          'grantee_user_id', matched_user_id,
                          'grantee_address', new.grantee_address,
                          'user_id', new.user_id
                        )
                    )
                  on conflict do nothing;
        end if;
    end if;
    return null;
exception
  when others then
      raise warning 'An error occurred in %: %', tg_name, sqlerrm;
      return null;
end; 
$$;


--
-- Name: recreate_trending_params(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.recreate_trending_params() RETURNS void
    LANGUAGE plpgsql
    AS $$
BEGIN
    create materialized view public.trending_params as
    select
    t.track_id,
    t.release_date,
    t.genre,
    t.owner_id,
    ap.play_count,
    au.follower_count as owner_follower_count,
    coalesce(aggregate_track.repost_count, 0) as repost_count,
    coalesce(aggregate_track.save_count, 0) as save_count,
    coalesce(repost_week.repost_count, (0) :: bigint) as repost_week_count,
    coalesce(repost_month.repost_count, (0) :: bigint) as repost_month_count,
    coalesce(repost_year.repost_count, (0) :: bigint) as repost_year_count,
    coalesce(save_week.repost_count, (0) :: bigint) as save_week_count,
    coalesce(save_month.repost_count, (0) :: bigint) as save_month_count,
    coalesce(save_year.repost_count, (0) :: bigint) as save_year_count,
    coalesce(karma.karma, (0) :: numeric) as karma
    from
    (
        (
            (
                (
                (
                    (
                        (
                            (
                            (
                                (
                                    public.tracks t
                                    left join (
                                        select
                                        ap_1.count as play_count,
                                        ap_1.play_item_id
                                        from
                                        public.aggregate_plays ap_1
                                    ) ap on ((ap.play_item_id = t.track_id))
                                )
                                left join (
                                    select
                                        au_1.user_id,
                                        au_1.follower_count
                                    from
                                        public.aggregate_user au_1
                                ) au on ((au.user_id = t.owner_id))
                            )
                            left join (
                                select
                                    aggregate_track_1.track_id,
                                    aggregate_track_1.repost_count,
                                    aggregate_track_1.save_count
                                from
                                    public.aggregate_track aggregate_track_1
                            ) aggregate_track on ((aggregate_track.track_id = t.track_id))
                            )
                            left join (
                            select
                                r.repost_item_id as track_id,
                                count(r.repost_item_id) as repost_count
                            from
                                public.reposts r
                            where
                                (
                                    (r.is_current is true)
                                    and (r.repost_type = 'track' :: public.reposttype)
                                    and (r.is_delete is false)
                                    and (r.created_at > (now() - '1 year' :: interval))
                                )
                            group by
                                r.repost_item_id
                            ) repost_year on ((repost_year.track_id = t.track_id))
                        )
                        left join (
                            select
                            r.repost_item_id as track_id,
                            count(r.repost_item_id) as repost_count
                            from
                            public.reposts r
                            where
                            (
                                (r.is_current is true)
                                and (r.repost_type = 'track' :: public.reposttype)
                                and (r.is_delete is false)
                                and (r.created_at > (now() - '1 mon' :: interval))
                            )
                            group by
                            r.repost_item_id
                        ) repost_month on ((repost_month.track_id = t.track_id))
                    )
                    left join (
                        select
                            r.repost_item_id as track_id,
                            count(r.repost_item_id) as repost_count
                        from
                            public.reposts r
                        where
                            (
                            (r.is_current is true)
                            and (r.repost_type = 'track' :: public.reposttype)
                            and (r.is_delete is false)
                            and (r.created_at > (now() - '7 days' :: interval))
                            )
                        group by
                            r.repost_item_id
                    ) repost_week on ((repost_week.track_id = t.track_id))
                )
                left join (
                    select
                        r.save_item_id as track_id,
                        count(r.save_item_id) as repost_count
                    from
                        public.saves r
                    where
                        (
                            (r.is_current is true)
                            and (r.save_type = 'track' :: public.savetype)
                            and (r.is_delete is false)
                            and (r.created_at > (now() - '1 year' :: interval))
                        )
                    group by
                        r.save_item_id
                ) save_year on ((save_year.track_id = t.track_id))
                )
                left join (
                select
                    r.save_item_id as track_id,
                    count(r.save_item_id) as repost_count
                from
                    public.saves r
                where
                    (
                        (r.is_current is true)
                        and (r.save_type = 'track' :: public.savetype)
                        and (r.is_delete is false)
                        and (r.created_at > (now() - '1 mon' :: interval))
                    )
                group by
                    r.save_item_id
                ) save_month on ((save_month.track_id = t.track_id))
            )
            left join (
                select
                r.save_item_id as track_id,
                count(r.save_item_id) as repost_count
                from
                public.saves r
                where
                (
                    (r.is_current is true)
                    and (r.save_type = 'track' :: public.savetype)
                    and (r.is_delete is false)
                    and (r.created_at > (now() - '7 days' :: interval))
                )
                group by
                r.save_item_id
            ) save_week on ((save_week.track_id = t.track_id))
        )
        left join (
            select
                save_and_reposts.item_id as track_id,
                sum(au_1.follower_count) as karma
            from
                (
                (
                    select
                        r_and_s.user_id,
                        r_and_s.item_id
                    from
                        (
                            (
                            select
                                reposts.user_id,
                                reposts.repost_item_id as item_id
                            from
                                public.reposts
                            where
                                (
                                    (reposts.is_delete is false)
                                    and (reposts.is_current is true)
                                    and (
                                        reposts.repost_type = 'track' :: public.reposttype
                                    )
                                )
                            union
                            all
                            select
                                saves.user_id,
                                saves.save_item_id as item_id
                            from
                                public.saves
                            where
                                (
                                    (saves.is_delete is false)
                                    and (saves.is_current is true)
                                    and (saves.save_type = 'track' :: public.savetype)
                                )
                            ) r_and_s
                            join public.users on ((r_and_s.user_id = users.user_id))
                        )
                    where
                        (
                            (
                            (users.cover_photo is not null)
                            or (users.cover_photo_sizes is not null)
                            )
                            and (
                            (users.profile_picture is not null)
                            or (users.profile_picture_sizes is not null)
                            )
                            and (users.bio is not null)
                        )
                ) save_and_reposts
                join public.aggregate_user au_1 on ((save_and_reposts.user_id = au_1.user_id))
                )
            group by
                save_and_reposts.item_id
        ) karma on ((karma.track_id = t.track_id))
    )
    where
    (
        (t.is_current is true)
        and (t.is_delete is false)
        and (t.is_unlisted is false)
        and (t.stem_of is null)
    ) with no data;

    create index trending_params_track_id_idx on public.trending_params using btree (track_id);
END;
$$;


--
-- Name: table_exists(text); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.table_exists(text) RETURNS boolean
    LANGUAGE sql
    AS $_$
  SELECT EXISTS (SELECT 1 FROM pg_tables WHERE tablename = $1)
$_$;


--
-- Name: table_has_column(text, text); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.table_has_column(text, text) RETURNS boolean
    LANGUAGE sql
    AS $_$
  SELECT EXISTS (SELECT column_name FROM information_schema.columns WHERE table_name = $1 AND column_name = $2)
$_$;


--
-- Name: table_has_constraint(text, text); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.table_has_constraint(text, text) RETURNS boolean
    LANGUAGE sql
    AS $_$
  SELECT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = $1::regclass AND conname = $2)
$_$;


--
-- Name: to_date_safe(character varying, character varying); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.to_date_safe(p_date character varying, p_format character varying) RETURNS date
    LANGUAGE plpgsql
    AS $$
        DECLARE
            ret_date DATE;
        BEGIN
            IF p_date = '' THEN
                RETURN NULL;
            END IF;
            RETURN to_date( p_date, p_format );
        EXCEPTION
        WHEN others THEN
            RETURN null;
        END;
        $$;


--
-- Name: to_timestamp_safe(character varying, character varying); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.to_timestamp_safe(p_timestamp character varying, p_format character varying) RETURNS timestamp without time zone
    LANGUAGE plpgsql
    AS $$
  DECLARE
      ret_timestamp TIMESTAMP;
  BEGIN
      IF p_timestamp = '' THEN
          RETURN NULL;
      END IF;
      RETURN to_timestamp( p_timestamp, p_format );
  EXCEPTION
  WHEN others THEN
      RETURN null;
  END;
  $$;


--
-- Name: track_is_public(record); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.track_is_public(track record) RETURNS boolean
    LANGUAGE plpgsql
    AS $$
begin
  return track.is_unlisted = false
     and track.is_available = true
     and track.is_delete = false
     and track.stem_of is null;
end
$$;


--
-- Name: validate_territory_codes(text[]); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.validate_territory_codes(codes text[]) RETURNS boolean
    LANGUAGE plpgsql
    AS $_$
begin
    -- null is valid
    if codes is null then
        return true;
    end if;
    
    -- array must have at least one element
    if array_length(codes, 1) is null then
        return false;
    end if;
    
    -- check each element to make sure it's a 2 letter ISO code
    for i in 1..array_length(codes, 1) loop
        if codes[i] !~ '^[A-Z]{2}$' then
            return false;
        end if;
    end loop;
    
    return true;
end;
$_$;


SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: tracks; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.tracks (
    blockhash character varying,
    track_id integer NOT NULL,
    is_current boolean NOT NULL,
    is_delete boolean NOT NULL,
    owner_id integer NOT NULL,
    title text,
    cover_art character varying,
    tags character varying,
    genre character varying,
    mood character varying,
    credits_splits character varying,
    create_date character varying,
    file_type character varying,
    metadata_multihash character varying,
    blocknumber integer,
    created_at timestamp without time zone NOT NULL,
    description character varying,
    isrc character varying,
    iswc character varying,
    license character varying,
    updated_at timestamp without time zone NOT NULL,
    cover_art_sizes character varying,
    is_unlisted boolean DEFAULT false NOT NULL,
    field_visibility jsonb,
    route_id character varying,
    stem_of jsonb,
    remix_of jsonb,
    txhash character varying DEFAULT ''::character varying NOT NULL,
    slot integer,
    is_available boolean DEFAULT true NOT NULL,
    stream_conditions jsonb,
    track_cid character varying,
    is_playlist_upload boolean DEFAULT false NOT NULL,
    duration integer DEFAULT 0,
    ai_attribution_user_id integer,
    preview_cid character varying,
    audio_upload_id character varying,
    preview_start_seconds double precision,
    release_date timestamp without time zone,
    track_segments jsonb DEFAULT '[]'::jsonb NOT NULL,
    is_scheduled_release boolean DEFAULT false NOT NULL,
    is_downloadable boolean DEFAULT false NOT NULL,
    download_conditions jsonb,
    is_original_available boolean DEFAULT false NOT NULL,
    orig_file_cid character varying,
    orig_filename character varying,
    playlists_containing_track integer[] DEFAULT '{}'::integer[] NOT NULL,
    placement_hosts text,
    ddex_app character varying,
    ddex_release_ids jsonb,
    artists jsonb,
    resource_contributors jsonb,
    indirect_resource_contributors jsonb,
    rights_controller jsonb,
    copyright_line jsonb,
    producer_copyright_line jsonb,
    parental_warning_type character varying,
    playlists_previously_containing_track jsonb DEFAULT jsonb_build_object() NOT NULL,
    allowed_api_keys text[],
    bpm double precision,
    musical_key character varying,
    audio_analysis_error_count integer DEFAULT 0 NOT NULL,
    is_custom_bpm boolean DEFAULT false,
    is_custom_musical_key boolean DEFAULT false,
    comments_disabled boolean DEFAULT false,
    pinned_comment_id integer,
    cover_original_song_title character varying,
    cover_original_artist character varying,
    is_owned_by_user boolean DEFAULT false NOT NULL,
    is_stream_gated boolean DEFAULT false,
    is_download_gated boolean DEFAULT false,
    no_ai_use boolean DEFAULT false,
    parental_warning public.parental_warning_type,
    territory_codes text[],
    CONSTRAINT check_territory_codes CHECK (public.validate_territory_codes(territory_codes))
);


--
-- Name: COLUMN tracks.cover_original_song_title; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.tracks.cover_original_song_title IS 'Title of the original song if this track is a cover';


--
-- Name: COLUMN tracks.cover_original_artist; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.tracks.cover_original_artist IS 'Artist of the original song if this track is a cover';


--
-- Name: COLUMN tracks.is_owned_by_user; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.tracks.is_owned_by_user IS 'Indicates whether the track is owned by the user for publishing payouts';


--
-- Name: track_should_notify(public.tracks, record, character varying); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.track_should_notify(old_track public.tracks, new_track record, tg_op character varying) RETURNS boolean
    LANGUAGE plpgsql
    AS $$
begin
  if tg_op = 'UPDATE' and old_track.track_id is not null then
    return not track_is_public(old_track) and track_is_public(new_track);
  else
    return tg_op = 'INSERT'
      and track_is_public(new_track)
    ;
  end if;
end
$$;


--
-- Name: update_sol_user_balance_mint(integer, character varying); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.update_sol_user_balance_mint(p_user_id integer, p_mint character varying) RETURNS void
    LANGUAGE plpgsql
    AS $$
BEGIN
    INSERT INTO sol_user_balances
        (user_id, mint, balance, updated_at, created_at)
    SELECT
        p_user_id,
        p_mint,
        SUM(balance),
        NOW(),
        NOW()
    FROM (
        SELECT 
            p_user_id AS user_id, 
            COALESCE(balance, 0) AS balance
        FROM associated_wallets 
        JOIN sol_token_account_balances AS associated_wallet_balances
            ON associated_wallet_balances.owner = associated_wallets.wallet
            AND associated_wallet_balances.mint = p_mint
        WHERE associated_wallets.user_id = p_user_id
            AND associated_wallets.chain = 'sol'
            AND associated_wallets.is_delete = FALSE

        UNION ALL

        SELECT 
            p_user_id AS user_id, 
            COALESCE(balance, 0) AS balance
        FROM users
        JOIN sol_claimable_accounts
            ON sol_claimable_accounts.ethereum_address = users.wallet
            AND sol_claimable_accounts.mint = p_mint
        JOIN sol_token_account_balances
            ON sol_token_account_balances.account = sol_claimable_accounts.account
        WHERE users.user_id = p_user_id
    ) AS balances
    GROUP BY user_id
    ON CONFLICT (user_id, mint)
    DO UPDATE SET
        balance = EXCLUDED.balance,
        updated_at = NOW();
END;
$$;


--
-- Name: audius_ts_dict; Type: TEXT SEARCH DICTIONARY; Schema: public; Owner: -
--

CREATE TEXT SEARCH DICTIONARY public.audius_ts_dict (
    TEMPLATE = pg_catalog.simple );


--
-- Name: audius_ts_config; Type: TEXT SEARCH CONFIGURATION; Schema: public; Owner: -
--

CREATE TEXT SEARCH CONFIGURATION public.audius_ts_config (
    PARSER = pg_catalog."default" );

ALTER TEXT SEARCH CONFIGURATION public.audius_ts_config
    ADD MAPPING FOR asciiword WITH public.audius_ts_dict;

ALTER TEXT SEARCH CONFIGURATION public.audius_ts_config
    ADD MAPPING FOR word WITH public.audius_ts_dict;

ALTER TEXT SEARCH CONFIGURATION public.audius_ts_config
    ADD MAPPING FOR numword WITH simple;

ALTER TEXT SEARCH CONFIGURATION public.audius_ts_config
    ADD MAPPING FOR email WITH simple;

ALTER TEXT SEARCH CONFIGURATION public.audius_ts_config
    ADD MAPPING FOR url WITH simple;

ALTER TEXT SEARCH CONFIGURATION public.audius_ts_config
    ADD MAPPING FOR host WITH simple;

ALTER TEXT SEARCH CONFIGURATION public.audius_ts_config
    ADD MAPPING FOR sfloat WITH simple;

ALTER TEXT SEARCH CONFIGURATION public.audius_ts_config
    ADD MAPPING FOR version WITH simple;

ALTER TEXT SEARCH CONFIGURATION public.audius_ts_config
    ADD MAPPING FOR hword_numpart WITH simple;

ALTER TEXT SEARCH CONFIGURATION public.audius_ts_config
    ADD MAPPING FOR hword_part WITH public.audius_ts_dict;

ALTER TEXT SEARCH CONFIGURATION public.audius_ts_config
    ADD MAPPING FOR hword_asciipart WITH public.audius_ts_dict;

ALTER TEXT SEARCH CONFIGURATION public.audius_ts_config
    ADD MAPPING FOR numhword WITH simple;

ALTER TEXT SEARCH CONFIGURATION public.audius_ts_config
    ADD MAPPING FOR asciihword WITH public.audius_ts_dict;

ALTER TEXT SEARCH CONFIGURATION public.audius_ts_config
    ADD MAPPING FOR hword WITH public.audius_ts_dict;

ALTER TEXT SEARCH CONFIGURATION public.audius_ts_config
    ADD MAPPING FOR url_path WITH simple;

ALTER TEXT SEARCH CONFIGURATION public.audius_ts_config
    ADD MAPPING FOR file WITH simple;

ALTER TEXT SEARCH CONFIGURATION public.audius_ts_config
    ADD MAPPING FOR "float" WITH simple;

ALTER TEXT SEARCH CONFIGURATION public.audius_ts_config
    ADD MAPPING FOR "int" WITH simple;

ALTER TEXT SEARCH CONFIGURATION public.audius_ts_config
    ADD MAPPING FOR uint WITH simple;


--
-- Name: access_keys; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.access_keys (
    id integer NOT NULL,
    track_id text NOT NULL,
    pub_key text NOT NULL
);


--
-- Name: access_keys_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.access_keys_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: access_keys_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.access_keys_id_seq OWNED BY public.access_keys.id;


--
-- Name: aggregate_daily_app_name_metrics; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aggregate_daily_app_name_metrics (
    id integer NOT NULL,
    application_name character varying NOT NULL,
    count integer NOT NULL,
    "timestamp" date NOT NULL,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);


--
-- Name: aggregate_daily_app_name_metrics_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aggregate_daily_app_name_metrics_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aggregate_daily_app_name_metrics_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aggregate_daily_app_name_metrics_id_seq OWNED BY public.aggregate_daily_app_name_metrics.id;


--
-- Name: aggregate_daily_total_users_metrics; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aggregate_daily_total_users_metrics (
    id integer NOT NULL,
    count integer NOT NULL,
    "timestamp" date NOT NULL,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    personal_count integer
);


--
-- Name: aggregate_daily_total_users_metrics_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aggregate_daily_total_users_metrics_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aggregate_daily_total_users_metrics_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aggregate_daily_total_users_metrics_id_seq OWNED BY public.aggregate_daily_total_users_metrics.id;


--
-- Name: aggregate_daily_unique_users_metrics; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aggregate_daily_unique_users_metrics (
    id integer NOT NULL,
    count integer NOT NULL,
    "timestamp" date NOT NULL,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    summed_count integer,
    personal_count integer
);


--
-- Name: aggregate_daily_unique_users_metrics_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aggregate_daily_unique_users_metrics_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aggregate_daily_unique_users_metrics_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aggregate_daily_unique_users_metrics_id_seq OWNED BY public.aggregate_daily_unique_users_metrics.id;


--
-- Name: plays; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.plays (
    id integer NOT NULL,
    user_id integer,
    source character varying,
    play_item_id integer NOT NULL,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    slot integer,
    signature character varying,
    city character varying,
    region character varying,
    country character varying
);


--
-- Name: aggregate_interval_plays; Type: MATERIALIZED VIEW; Schema: public; Owner: -
--

CREATE MATERIALIZED VIEW public.aggregate_interval_plays AS
 SELECT tracks.track_id,
    tracks.genre,
    tracks.created_at,
    COALESCE(week_listen_counts.count, (0)::bigint) AS week_listen_counts,
    COALESCE(month_listen_counts.count, (0)::bigint) AS month_listen_counts
   FROM ((public.tracks
     LEFT JOIN ( SELECT plays.play_item_id,
            count(plays.id) AS count
           FROM public.plays
          WHERE (plays.created_at > (now() - '7 days'::interval))
          GROUP BY plays.play_item_id) week_listen_counts ON ((week_listen_counts.play_item_id = tracks.track_id)))
     LEFT JOIN ( SELECT plays.play_item_id,
            count(plays.id) AS count
           FROM public.plays
          WHERE (plays.created_at > (now() - '1 mon'::interval))
          GROUP BY plays.play_item_id) month_listen_counts ON ((month_listen_counts.play_item_id = tracks.track_id)))
  WHERE ((tracks.is_current IS TRUE) AND (tracks.is_delete IS FALSE) AND (tracks.is_unlisted IS FALSE) AND (tracks.stem_of IS NULL))
  WITH NO DATA;


--
-- Name: aggregate_monthly_app_name_metrics; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aggregate_monthly_app_name_metrics (
    id integer NOT NULL,
    application_name character varying NOT NULL,
    count integer NOT NULL,
    "timestamp" date NOT NULL,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);


--
-- Name: aggregate_monthly_app_name_metrics_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aggregate_monthly_app_name_metrics_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aggregate_monthly_app_name_metrics_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aggregate_monthly_app_name_metrics_id_seq OWNED BY public.aggregate_monthly_app_name_metrics.id;


--
-- Name: aggregate_monthly_plays; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aggregate_monthly_plays (
    play_item_id integer NOT NULL,
    "timestamp" date DEFAULT CURRENT_TIMESTAMP NOT NULL,
    count integer NOT NULL,
    country text DEFAULT ''::text NOT NULL
);


--
-- Name: aggregate_monthly_total_users_metrics; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aggregate_monthly_total_users_metrics (
    id integer NOT NULL,
    count integer NOT NULL,
    "timestamp" date NOT NULL,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    personal_count integer
);


--
-- Name: aggregate_monthly_total_users_metrics_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aggregate_monthly_total_users_metrics_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aggregate_monthly_total_users_metrics_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aggregate_monthly_total_users_metrics_id_seq OWNED BY public.aggregate_monthly_total_users_metrics.id;


--
-- Name: aggregate_monthly_unique_users_metrics; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aggregate_monthly_unique_users_metrics (
    id integer NOT NULL,
    count integer NOT NULL,
    "timestamp" date NOT NULL,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    summed_count integer,
    personal_count integer
);


--
-- Name: aggregate_monthly_unique_users_metrics_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aggregate_monthly_unique_users_metrics_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aggregate_monthly_unique_users_metrics_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aggregate_monthly_unique_users_metrics_id_seq OWNED BY public.aggregate_monthly_unique_users_metrics.id;


--
-- Name: aggregate_playlist; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aggregate_playlist (
    playlist_id integer NOT NULL,
    is_album boolean,
    repost_count integer DEFAULT 0,
    save_count integer DEFAULT 0,
    share_count integer DEFAULT 0
);


--
-- Name: aggregate_plays; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aggregate_plays (
    play_item_id integer NOT NULL,
    count bigint
);


--
-- Name: aggregate_track; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aggregate_track (
    track_id integer NOT NULL,
    repost_count integer DEFAULT 0 NOT NULL,
    save_count integer DEFAULT 0 NOT NULL,
    comment_count integer DEFAULT 0,
    share_count integer DEFAULT 0
);


--
-- Name: aggregate_user; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aggregate_user (
    user_id integer NOT NULL,
    track_count bigint DEFAULT 0,
    playlist_count bigint DEFAULT 0,
    album_count bigint DEFAULT 0,
    follower_count bigint DEFAULT 0,
    following_count bigint DEFAULT 0,
    repost_count bigint DEFAULT 0,
    track_save_count bigint DEFAULT 0,
    supporter_count integer DEFAULT 0 NOT NULL,
    supporting_count integer DEFAULT 0 NOT NULL,
    dominant_genre character varying,
    dominant_genre_count integer DEFAULT 0,
    score integer DEFAULT 0,
    total_track_count bigint DEFAULT 0,
    track_share_count integer DEFAULT 0
);


--
-- Name: aggregate_user_tips; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aggregate_user_tips (
    sender_user_id integer NOT NULL,
    receiver_user_id integer NOT NULL,
    amount bigint NOT NULL
);


--
-- Name: album_price_history; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.album_price_history (
    playlist_id integer NOT NULL,
    splits jsonb NOT NULL,
    total_price_cents bigint NOT NULL,
    blocknumber integer NOT NULL,
    block_timestamp timestamp without time zone NOT NULL,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);


--
-- Name: alembic_version; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.alembic_version (
    version_num character varying(32) NOT NULL
);


--
-- Name: anti_abuse_blocked_users; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.anti_abuse_blocked_users (
    handle_lc character varying(255) NOT NULL,
    is_blocked boolean DEFAULT false NOT NULL,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);


--
-- Name: api_metrics_apps; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.api_metrics_apps (
    date date NOT NULL,
    api_key character varying(255) NOT NULL,
    app_name character varying(255) NOT NULL,
    request_count bigint DEFAULT 0 NOT NULL,
    created_at timestamp without time zone DEFAULT now() NOT NULL,
    updated_at timestamp without time zone DEFAULT now() NOT NULL
);


--
-- Name: api_metrics_counts; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.api_metrics_counts (
    date date NOT NULL,
    hll_sketch bytea NOT NULL,
    total_count bigint DEFAULT 0 NOT NULL,
    unique_count bigint DEFAULT 0 NOT NULL,
    created_at timestamp without time zone DEFAULT now(),
    updated_at timestamp without time zone DEFAULT now()
);


--
-- Name: api_metrics_routes; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.api_metrics_routes (
    date date NOT NULL,
    route_pattern character varying(512) NOT NULL,
    method character varying(10) NOT NULL,
    request_count bigint DEFAULT 0 NOT NULL,
    created_at timestamp without time zone DEFAULT now() NOT NULL,
    updated_at timestamp without time zone DEFAULT now() NOT NULL
);


--
-- Name: app_name_metrics; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.app_name_metrics (
    application_name character varying NOT NULL,
    count integer NOT NULL,
    "timestamp" timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    id bigint NOT NULL,
    ip character varying
);


--
-- Name: app_name_metrics_all_time; Type: MATERIALIZED VIEW; Schema: public; Owner: -
--

CREATE MATERIALIZED VIEW public.app_name_metrics_all_time AS
 SELECT application_name AS name,
    sum(count) AS count
   FROM public.app_name_metrics
  GROUP BY application_name
  WITH NO DATA;


--
-- Name: app_name_metrics_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.app_name_metrics ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.app_name_metrics_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: app_name_metrics_trailing_month; Type: MATERIALIZED VIEW; Schema: public; Owner: -
--

CREATE MATERIALIZED VIEW public.app_name_metrics_trailing_month AS
 SELECT application_name AS name,
    sum(count) AS count
   FROM public.app_name_metrics
  WHERE ("timestamp" > (now() - '1 mon'::interval))
  GROUP BY application_name
  WITH NO DATA;


--
-- Name: app_name_metrics_trailing_week; Type: MATERIALIZED VIEW; Schema: public; Owner: -
--

CREATE MATERIALIZED VIEW public.app_name_metrics_trailing_week AS
 SELECT application_name AS name,
    sum(count) AS count
   FROM public.app_name_metrics
  WHERE ("timestamp" > (now() - '7 days'::interval))
  GROUP BY application_name
  WITH NO DATA;


--
-- Name: artist_coins; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.artist_coins (
    mint character varying NOT NULL,
    ticker character varying NOT NULL,
    user_id integer NOT NULL,
    decimals integer NOT NULL,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    logo_uri text,
    description text,
    website text
);


--
-- Name: TABLE artist_coins; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.artist_coins IS 'Stores the token mints for artist coins that the indexer is tracking and their tickers.';


--
-- Name: associated_wallets; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.associated_wallets (
    id integer NOT NULL,
    user_id integer NOT NULL,
    wallet character varying NOT NULL,
    blockhash character varying NOT NULL,
    blocknumber integer NOT NULL,
    is_current boolean NOT NULL,
    is_delete boolean NOT NULL,
    chain public.wallet_chain NOT NULL
);


--
-- Name: associated_wallets_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.associated_wallets_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: associated_wallets_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.associated_wallets_id_seq OWNED BY public.associated_wallets.id;


--
-- Name: audio_transactions_history; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.audio_transactions_history (
    user_bank character varying NOT NULL,
    slot integer NOT NULL,
    signature character varying NOT NULL,
    transaction_type character varying NOT NULL,
    method character varying NOT NULL,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    transaction_created_at timestamp without time zone NOT NULL,
    change numeric NOT NULL,
    balance numeric NOT NULL,
    tx_metadata character varying
);


--
-- Name: audius_data_txs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.audius_data_txs (
    signature character varying NOT NULL,
    slot integer NOT NULL
);


--
-- Name: blocks; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.blocks (
    blockhash character varying NOT NULL,
    parenthash character varying,
    is_current boolean,
    number integer
);


--
-- Name: challenge_disbursements; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.challenge_disbursements (
    challenge_id character varying NOT NULL,
    user_id integer NOT NULL,
    specifier character varying NOT NULL,
    signature character varying NOT NULL,
    slot integer NOT NULL,
    amount character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now()
);


--
-- Name: challenge_listen_streak; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.challenge_listen_streak (
    user_id integer NOT NULL,
    last_listen_date timestamp without time zone,
    listen_streak integer NOT NULL
);


--
-- Name: challenge_listen_streak_user_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.challenge_listen_streak_user_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: challenge_listen_streak_user_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.challenge_listen_streak_user_id_seq OWNED BY public.challenge_listen_streak.user_id;


--
-- Name: challenge_profile_completion; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.challenge_profile_completion (
    user_id integer NOT NULL,
    profile_description boolean NOT NULL,
    profile_name boolean NOT NULL,
    profile_picture boolean NOT NULL,
    profile_cover_photo boolean NOT NULL,
    follows boolean NOT NULL,
    favorites boolean NOT NULL,
    reposts boolean NOT NULL
);


--
-- Name: challenge_profile_completion_user_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.challenge_profile_completion_user_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: challenge_profile_completion_user_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.challenge_profile_completion_user_id_seq OWNED BY public.challenge_profile_completion.user_id;


--
-- Name: challenges; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.challenges (
    id character varying NOT NULL,
    type public.challengetype NOT NULL,
    amount character varying NOT NULL,
    active boolean NOT NULL,
    step_count integer,
    starting_block integer,
    weekly_pool integer,
    cooldown_days integer
);


--
-- Name: chat; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.chat (
    chat_id text NOT NULL,
    created_at timestamp without time zone NOT NULL,
    last_message_at timestamp without time zone NOT NULL,
    last_message text,
    last_message_is_plaintext boolean DEFAULT false
);


--
-- Name: chat_ban; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.chat_ban (
    user_id integer NOT NULL,
    is_banned boolean NOT NULL,
    updated_at timestamp without time zone NOT NULL
);


--
-- Name: chat_blast; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.chat_blast (
    blast_id text NOT NULL,
    from_user_id integer NOT NULL,
    audience text NOT NULL,
    audience_content_id integer,
    plaintext text NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    audience_content_type text
);


--
-- Name: chat_blocked_users; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.chat_blocked_users (
    blocker_user_id integer NOT NULL,
    blockee_user_id integer NOT NULL,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);


--
-- Name: chat_member; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.chat_member (
    chat_id text NOT NULL,
    user_id integer NOT NULL,
    cleared_history_at timestamp without time zone,
    invited_by_user_id integer NOT NULL,
    invite_code text NOT NULL,
    last_active_at timestamp without time zone,
    unread_count integer DEFAULT 0 NOT NULL,
    created_at timestamp without time zone NOT NULL,
    is_hidden boolean DEFAULT false NOT NULL
);


--
-- Name: chat_message; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.chat_message (
    message_id text NOT NULL,
    chat_id text NOT NULL,
    user_id integer NOT NULL,
    created_at timestamp without time zone NOT NULL,
    ciphertext text,
    blast_id text
);


--
-- Name: chat_message_reactions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.chat_message_reactions (
    user_id integer NOT NULL,
    message_id text NOT NULL,
    reaction text NOT NULL,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);


--
-- Name: chat_permissions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.chat_permissions (
    user_id integer NOT NULL,
    permits text DEFAULT 'all'::text NOT NULL,
    updated_at timestamp without time zone DEFAULT to_timestamp((0)::double precision) NOT NULL,
    allowed boolean DEFAULT true NOT NULL
);


--
-- Name: cid_data; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.cid_data (
    cid character varying NOT NULL,
    type character varying,
    data jsonb
);


--
-- Name: collectibles; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.collectibles (
    user_id integer NOT NULL,
    data jsonb NOT NULL,
    blockhash text NOT NULL,
    blocknumber integer NOT NULL,
    created_at timestamp with time zone DEFAULT now(),
    updated_at timestamp with time zone DEFAULT now()
);


--
-- Name: TABLE collectibles; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.collectibles IS 'Stores collectibles data for users';


--
-- Name: COLUMN collectibles.user_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.collectibles.user_id IS 'User ID of the person who owns the collectibles';


--
-- Name: COLUMN collectibles.data; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.collectibles.data IS 'Data about the collectibles';


--
-- Name: COLUMN collectibles.blockhash; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.collectibles.blockhash IS 'Blockhash of the most recent block that changed the collectibles data';


--
-- Name: COLUMN collectibles.blocknumber; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.collectibles.blocknumber IS 'Block number of the most recent block that changed the collectibles data';


--
-- Name: comment_mentions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.comment_mentions (
    comment_id integer NOT NULL,
    user_id integer NOT NULL,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    is_delete boolean DEFAULT false,
    txhash text NOT NULL,
    blockhash text NOT NULL,
    blocknumber integer
);


--
-- Name: comment_notification_settings; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.comment_notification_settings (
    user_id integer NOT NULL,
    entity_id integer NOT NULL,
    entity_type text NOT NULL,
    is_muted boolean DEFAULT false,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);


--
-- Name: comment_reactions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.comment_reactions (
    comment_id integer NOT NULL,
    user_id integer NOT NULL,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    is_delete boolean DEFAULT false,
    txhash text NOT NULL,
    blockhash text NOT NULL,
    blocknumber integer
);


--
-- Name: comment_reports; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.comment_reports (
    comment_id integer NOT NULL,
    user_id integer NOT NULL,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    is_delete boolean DEFAULT false,
    txhash text NOT NULL,
    blockhash text NOT NULL,
    blocknumber integer
);


--
-- Name: comment_threads; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.comment_threads (
    comment_id integer NOT NULL,
    parent_comment_id integer NOT NULL
);


--
-- Name: comments; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.comments (
    comment_id integer NOT NULL,
    text text NOT NULL,
    user_id integer NOT NULL,
    entity_id integer NOT NULL,
    entity_type text NOT NULL,
    track_timestamp_s bigint,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    is_delete boolean DEFAULT false,
    is_visible boolean DEFAULT true,
    is_edited boolean DEFAULT false,
    txhash text NOT NULL,
    blockhash text NOT NULL,
    blocknumber integer
);


--
-- Name: core_app_state; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.core_app_state (
    block_height bigint NOT NULL,
    app_hash bytea NOT NULL,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);


--
-- Name: core_blocks; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.core_blocks (
    rowid bigint NOT NULL,
    height bigint NOT NULL,
    chain_id text NOT NULL,
    hash text NOT NULL,
    proposer text NOT NULL,
    created_at timestamp without time zone NOT NULL
);


--
-- Name: core_blocks_rowid_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.core_blocks_rowid_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: core_blocks_rowid_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.core_blocks_rowid_seq OWNED BY public.core_blocks.rowid;


--
-- Name: core_db_migrations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.core_db_migrations (
    id text NOT NULL,
    applied_at timestamp with time zone
);


--
-- Name: core_indexed_blocks; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.core_indexed_blocks (
    blockhash character varying NOT NULL,
    parenthash character varying,
    chain_id text NOT NULL,
    height integer NOT NULL,
    plays_slot integer DEFAULT 0,
    em_block integer
);


--
-- Name: core_transactions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.core_transactions (
    rowid bigint NOT NULL,
    block_id bigint NOT NULL,
    index integer NOT NULL,
    tx_hash text NOT NULL,
    transaction bytea NOT NULL,
    created_at timestamp without time zone NOT NULL
);


--
-- Name: core_transactions_rowid_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.core_transactions_rowid_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: core_transactions_rowid_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.core_transactions_rowid_seq OWNED BY public.core_transactions.rowid;


--
-- Name: core_tx_stats; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.core_tx_stats (
    id integer NOT NULL,
    tx_type text NOT NULL,
    tx_hash text NOT NULL,
    block_height bigint NOT NULL,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);


--
-- Name: core_tx_stats_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.core_tx_stats_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: core_tx_stats_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.core_tx_stats_id_seq OWNED BY public.core_tx_stats.id;


--
-- Name: core_validators; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.core_validators (
    rowid integer NOT NULL,
    pub_key text NOT NULL,
    endpoint text NOT NULL,
    eth_address text NOT NULL,
    comet_address text NOT NULL,
    eth_block text NOT NULL,
    node_type text NOT NULL,
    sp_id text NOT NULL,
    comet_pub_key text NOT NULL
);


--
-- Name: core_validators_rowid_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.core_validators_rowid_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: core_validators_rowid_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.core_validators_rowid_seq OWNED BY public.core_validators.rowid;


--
-- Name: countries; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.countries (
    iso character(2) NOT NULL,
    name character varying(80) NOT NULL,
    nicename character varying(80) NOT NULL,
    iso3 character(3) DEFAULT NULL::bpchar,
    numcode smallint,
    phonecode integer NOT NULL
);


--
-- Name: dashboard_wallet_users; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.dashboard_wallet_users (
    wallet character varying NOT NULL,
    user_id integer NOT NULL,
    is_delete boolean DEFAULT false NOT NULL,
    updated_at timestamp without time zone NOT NULL,
    created_at timestamp without time zone NOT NULL,
    blockhash character varying,
    blocknumber integer,
    txhash character varying NOT NULL
);


--
-- Name: delist_status_cursor; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.delist_status_cursor (
    host text NOT NULL,
    entity public.delist_entity NOT NULL,
    created_at timestamp with time zone NOT NULL
);


--
-- Name: developer_apps; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.developer_apps (
    address character varying NOT NULL,
    blockhash character varying,
    blocknumber integer,
    user_id integer,
    name character varying NOT NULL,
    is_personal_access boolean DEFAULT false NOT NULL,
    is_delete boolean DEFAULT false NOT NULL,
    created_at timestamp without time zone NOT NULL,
    txhash character varying NOT NULL,
    is_current boolean NOT NULL,
    updated_at timestamp without time zone NOT NULL,
    description character varying(255),
    image_url character varying
);


--
-- Name: email_access; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.email_access (
    id integer NOT NULL,
    email_owner_user_id integer NOT NULL,
    receiving_user_id integer NOT NULL,
    grantor_user_id integer NOT NULL,
    encrypted_key text NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    is_initial boolean DEFAULT false NOT NULL
);


--
-- Name: TABLE email_access; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.email_access IS 'Tracks who has access to encrypted emails';


--
-- Name: COLUMN email_access.email_owner_user_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.email_access.email_owner_user_id IS 'The user ID of the email owner';


--
-- Name: COLUMN email_access.receiving_user_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.email_access.receiving_user_id IS 'The user ID of the person granted access';


--
-- Name: COLUMN email_access.grantor_user_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.email_access.grantor_user_id IS 'The user ID of the person who granted access';


--
-- Name: COLUMN email_access.encrypted_key; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.email_access.encrypted_key IS 'The symmetric key (SK) encrypted for the receiving user';


--
-- Name: email_access_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.email_access_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: email_access_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.email_access_id_seq OWNED BY public.email_access.id;


--
-- Name: encrypted_emails; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.encrypted_emails (
    id integer NOT NULL,
    email_owner_user_id integer NOT NULL,
    encrypted_email text NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP
);


--
-- Name: TABLE encrypted_emails; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.encrypted_emails IS 'Stores encrypted email addresses';


--
-- Name: COLUMN encrypted_emails.email_owner_user_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.encrypted_emails.email_owner_user_id IS 'The user ID of the email owner';


--
-- Name: COLUMN encrypted_emails.encrypted_email; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.encrypted_emails.encrypted_email IS 'The encrypted email address (base64 encoded)';


--
-- Name: encrypted_emails_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.encrypted_emails_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: encrypted_emails_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.encrypted_emails_id_seq OWNED BY public.encrypted_emails.id;


--
-- Name: eth_blocks; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.eth_blocks (
    last_scanned_block integer NOT NULL,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);


--
-- Name: eth_blocks_last_scanned_block_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.eth_blocks_last_scanned_block_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: eth_blocks_last_scanned_block_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.eth_blocks_last_scanned_block_seq OWNED BY public.eth_blocks.last_scanned_block;


--
-- Name: eth_db_migrations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.eth_db_migrations (
    version bigint NOT NULL,
    dirty boolean NOT NULL
);


--
-- Name: eth_funding_rounds; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.eth_funding_rounds (
    round_num integer NOT NULL,
    blocknumber bigint NOT NULL,
    creation_time timestamp without time zone NOT NULL
);


--
-- Name: eth_registered_endpoints; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.eth_registered_endpoints (
    id integer NOT NULL,
    service_type text NOT NULL,
    owner text NOT NULL,
    delegate_wallet text NOT NULL,
    endpoint text NOT NULL,
    blocknumber bigint NOT NULL
);


--
-- Name: eth_service_providers; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.eth_service_providers (
    address text NOT NULL,
    deployer_stake bigint NOT NULL,
    deployer_cut bigint NOT NULL,
    valid_bounds boolean NOT NULL,
    number_of_endpoints integer NOT NULL,
    min_account_stake bigint NOT NULL,
    max_account_stake bigint NOT NULL
);


--
-- Name: events; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.events (
    event_id integer NOT NULL,
    event_type public.event_type NOT NULL,
    user_id integer NOT NULL,
    entity_type public.event_entity_type,
    entity_id integer,
    end_date timestamp without time zone,
    is_deleted boolean DEFAULT false,
    event_data jsonb,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    txhash text NOT NULL,
    blockhash text NOT NULL,
    blocknumber integer
);


--
-- Name: follows; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.follows (
    blockhash character varying,
    blocknumber integer,
    follower_user_id integer NOT NULL,
    followee_user_id integer NOT NULL,
    is_current boolean NOT NULL,
    is_delete boolean NOT NULL,
    created_at timestamp without time zone NOT NULL,
    txhash character varying DEFAULT ''::character varying NOT NULL,
    slot integer
);


--
-- Name: grants; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.grants (
    blockhash character varying,
    blocknumber integer,
    grantee_address character varying NOT NULL,
    user_id integer NOT NULL,
    is_revoked boolean DEFAULT false NOT NULL,
    is_current boolean NOT NULL,
    is_approved boolean,
    updated_at timestamp without time zone NOT NULL,
    created_at timestamp without time zone NOT NULL,
    txhash character varying NOT NULL
);


--
-- Name: hourly_play_counts; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.hourly_play_counts (
    hourly_timestamp timestamp without time zone NOT NULL,
    play_count integer NOT NULL
);


--
-- Name: indexing_checkpoints; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.indexing_checkpoints (
    tablename character varying NOT NULL,
    last_checkpoint integer NOT NULL,
    signature character varying
);


--
-- Name: management_keys; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.management_keys (
    id integer NOT NULL,
    track_id text NOT NULL,
    address text NOT NULL
);


--
-- Name: management_keys_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.management_keys_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: management_keys_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.management_keys_id_seq OWNED BY public.management_keys.id;


--
-- Name: milestones; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.milestones (
    id integer NOT NULL,
    name character varying NOT NULL,
    threshold integer NOT NULL,
    blocknumber integer,
    slot integer,
    "timestamp" timestamp without time zone NOT NULL
);


--
-- Name: muted_users; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.muted_users (
    muted_user_id integer NOT NULL,
    user_id integer NOT NULL,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    is_delete boolean DEFAULT false,
    txhash text NOT NULL,
    blockhash text NOT NULL,
    blocknumber integer
);


--
-- Name: notification; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.notification (
    id integer NOT NULL,
    specifier character varying NOT NULL,
    group_id character varying NOT NULL,
    type character varying NOT NULL,
    slot integer,
    blocknumber integer,
    "timestamp" timestamp without time zone NOT NULL,
    data jsonb,
    user_ids integer[],
    type_v2 character varying
);


--
-- Name: notification_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.notification_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: notification_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.notification_id_seq OWNED BY public.notification.id;


--
-- Name: notification_seen; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.notification_seen (
    user_id integer NOT NULL,
    seen_at timestamp without time zone NOT NULL,
    blocknumber integer,
    blockhash character varying,
    txhash character varying
);


--
-- Name: payment_router_txs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.payment_router_txs (
    signature character varying NOT NULL,
    slot integer NOT NULL,
    created_at timestamp without time zone NOT NULL
);


--
-- Name: playlist_routes; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.playlist_routes (
    slug character varying NOT NULL,
    title_slug character varying NOT NULL,
    collision_id integer NOT NULL,
    owner_id integer NOT NULL,
    playlist_id integer NOT NULL,
    is_current boolean NOT NULL,
    blockhash character varying NOT NULL,
    blocknumber integer NOT NULL,
    txhash character varying NOT NULL
);


--
-- Name: playlist_seen; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.playlist_seen (
    user_id integer NOT NULL,
    playlist_id integer NOT NULL,
    seen_at timestamp without time zone NOT NULL,
    is_current boolean NOT NULL,
    blocknumber integer,
    blockhash character varying,
    txhash character varying
);


--
-- Name: playlist_tracks; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.playlist_tracks (
    playlist_id integer NOT NULL,
    track_id integer NOT NULL,
    is_removed boolean NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);


--
-- Name: playlist_trending_scores; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.playlist_trending_scores (
    playlist_id integer NOT NULL,
    type character varying NOT NULL,
    version character varying NOT NULL,
    time_range character varying NOT NULL,
    score double precision NOT NULL,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);


--
-- Name: playlists; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.playlists (
    blockhash character varying,
    blocknumber integer,
    playlist_id integer NOT NULL,
    playlist_owner_id integer NOT NULL,
    is_album boolean NOT NULL,
    is_private boolean NOT NULL,
    playlist_name character varying,
    playlist_contents jsonb NOT NULL,
    playlist_image_multihash character varying,
    is_current boolean NOT NULL,
    is_delete boolean NOT NULL,
    description character varying,
    created_at timestamp without time zone NOT NULL,
    upc character varying,
    updated_at timestamp without time zone NOT NULL,
    playlist_image_sizes_multihash character varying,
    txhash character varying DEFAULT ''::character varying NOT NULL,
    last_added_to timestamp without time zone,
    slot integer,
    metadata_multihash character varying,
    is_image_autogenerated boolean DEFAULT false NOT NULL,
    stream_conditions jsonb,
    ddex_app character varying,
    ddex_release_ids jsonb,
    artists jsonb,
    copyright_line jsonb,
    producer_copyright_line jsonb,
    parental_warning_type character varying,
    is_scheduled_release boolean DEFAULT false NOT NULL,
    release_date timestamp without time zone,
    is_stream_gated boolean DEFAULT false
);


--
-- Name: plays_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.plays_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: plays_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.plays_id_seq OWNED BY public.plays.id;


--
-- Name: reactions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.reactions (
    id integer NOT NULL,
    reaction_value integer NOT NULL,
    sender_wallet character varying NOT NULL,
    reaction_type character varying NOT NULL,
    reacted_to character varying NOT NULL,
    "timestamp" timestamp without time zone NOT NULL,
    blocknumber integer
);


--
-- Name: reactions_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.reactions_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: reactions_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.reactions_id_seq OWNED BY public.reactions.id;


--
-- Name: related_artists; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.related_artists (
    user_id integer NOT NULL,
    related_artist_user_id integer NOT NULL,
    score double precision NOT NULL,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);


--
-- Name: remixes; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.remixes (
    parent_track_id integer NOT NULL,
    child_track_id integer NOT NULL
);


--
-- Name: reported_comments; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.reported_comments (
    reported_comment_id integer NOT NULL,
    user_id integer NOT NULL,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    txhash text NOT NULL,
    blockhash text NOT NULL,
    blocknumber integer
);


--
-- Name: reposts; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.reposts (
    blockhash character varying,
    blocknumber integer,
    user_id integer NOT NULL,
    repost_item_id integer NOT NULL,
    repost_type public.reposttype NOT NULL,
    is_current boolean NOT NULL,
    is_delete boolean NOT NULL,
    created_at timestamp without time zone NOT NULL,
    txhash character varying DEFAULT ''::character varying NOT NULL,
    slot integer,
    is_repost_of_repost boolean DEFAULT false NOT NULL
);


--
-- Name: revert_blocks; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.revert_blocks (
    blocknumber integer NOT NULL,
    prev_records jsonb NOT NULL
);


--
-- Name: reward_manager_txs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.reward_manager_txs (
    signature character varying NOT NULL,
    slot integer NOT NULL,
    created_at timestamp without time zone NOT NULL
);


--
-- Name: route_metrics; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.route_metrics (
    route_path character varying NOT NULL,
    version character varying NOT NULL,
    query_string character varying DEFAULT ''::character varying NOT NULL,
    count integer NOT NULL,
    "timestamp" timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    id bigint NOT NULL,
    ip character varying
);


--
-- Name: route_metrics_all_time; Type: MATERIALIZED VIEW; Schema: public; Owner: -
--

CREATE MATERIALIZED VIEW public.route_metrics_all_time AS
 SELECT count(DISTINCT ip) AS unique_count,
    sum(count) AS count
   FROM public.route_metrics
  WITH NO DATA;


--
-- Name: route_metrics_day_bucket; Type: MATERIALIZED VIEW; Schema: public; Owner: -
--

CREATE MATERIALIZED VIEW public.route_metrics_day_bucket AS
 SELECT count(DISTINCT ip) AS unique_count,
    sum(count) AS count,
    date_trunc('day'::text, "timestamp") AS "time"
   FROM public.route_metrics
  GROUP BY (date_trunc('day'::text, "timestamp"))
  WITH NO DATA;


--
-- Name: route_metrics_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.route_metrics ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.route_metrics_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: route_metrics_month_bucket; Type: MATERIALIZED VIEW; Schema: public; Owner: -
--

CREATE MATERIALIZED VIEW public.route_metrics_month_bucket AS
 SELECT count(DISTINCT ip) AS unique_count,
    sum(count) AS count,
    date_trunc('month'::text, "timestamp") AS "time"
   FROM public.route_metrics
  GROUP BY (date_trunc('month'::text, "timestamp"))
  WITH NO DATA;


--
-- Name: route_metrics_trailing_month; Type: MATERIALIZED VIEW; Schema: public; Owner: -
--

CREATE MATERIALIZED VIEW public.route_metrics_trailing_month AS
 SELECT count(DISTINCT ip) AS unique_count,
    sum(count) AS count
   FROM public.route_metrics
  WHERE ("timestamp" > (now() - '1 mon'::interval))
  WITH NO DATA;


--
-- Name: route_metrics_trailing_week; Type: MATERIALIZED VIEW; Schema: public; Owner: -
--

CREATE MATERIALIZED VIEW public.route_metrics_trailing_week AS
 SELECT count(DISTINCT ip) AS unique_count,
    sum(count) AS count
   FROM public.route_metrics
  WHERE ("timestamp" > (now() - '7 days'::interval))
  WITH NO DATA;


--
-- Name: rpc_cursor; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.rpc_cursor (
    relayed_by text NOT NULL,
    relayed_at timestamp without time zone NOT NULL
);


--
-- Name: rpc_error; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.rpc_error (
    sig text NOT NULL,
    rpc_log_json jsonb NOT NULL,
    error_text text NOT NULL,
    error_count integer DEFAULT 0 NOT NULL,
    last_attempt timestamp without time zone NOT NULL
);


--
-- Name: rpc_log; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.rpc_log (
    relayed_at timestamp without time zone NOT NULL,
    from_wallet text NOT NULL,
    rpc json NOT NULL,
    sig text NOT NULL,
    relayed_by text NOT NULL,
    applied_at timestamp without time zone NOT NULL
);


--
-- Name: saves; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.saves (
    blockhash character varying,
    blocknumber integer,
    user_id integer NOT NULL,
    save_item_id integer NOT NULL,
    save_type public.savetype NOT NULL,
    is_current boolean NOT NULL,
    is_delete boolean NOT NULL,
    created_at timestamp without time zone NOT NULL,
    txhash character varying DEFAULT ''::character varying NOT NULL,
    slot integer,
    is_save_of_repost boolean DEFAULT false NOT NULL
);


--
-- Name: schema_migrations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.schema_migrations (
    version character varying(255) NOT NULL
);


--
-- Name: schema_version; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.schema_version (
    file_name text NOT NULL,
    md5 text,
    applied_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: shares; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.shares (
    blockhash character varying,
    blocknumber integer,
    user_id integer NOT NULL,
    share_item_id integer NOT NULL,
    share_type public.sharetype NOT NULL,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    txhash character varying DEFAULT ''::character varying NOT NULL,
    slot integer
);


--
-- Name: skipped_transactions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.skipped_transactions (
    id integer NOT NULL,
    blocknumber integer NOT NULL,
    blockhash character varying NOT NULL,
    txhash character varying NOT NULL,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    level public.skippedtransactionlevel DEFAULT 'node'::public.skippedtransactionlevel NOT NULL
);


--
-- Name: skipped_transactions_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.skipped_transactions_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: skipped_transactions_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.skipped_transactions_id_seq OWNED BY public.skipped_transactions.id;


--
-- Name: sla_auditor_version_data; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.sla_auditor_version_data (
    id integer NOT NULL,
    "nodeEndpoint" character varying(255) NOT NULL,
    "nodeVersion" character varying(255) NOT NULL,
    "minVersion" character varying(255) NOT NULL,
    owner character varying(255) NOT NULL,
    ok boolean NOT NULL,
    "timestamp" timestamp with time zone DEFAULT CURRENT_TIMESTAMP
);


--
-- Name: sla_auditor_version_data_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.sla_auditor_version_data_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: sla_auditor_version_data_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.sla_auditor_version_data_id_seq OWNED BY public.sla_auditor_version_data.id;


--
-- Name: sla_node_reports; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.sla_node_reports (
    id integer NOT NULL,
    address character varying NOT NULL,
    blocks_proposed integer NOT NULL,
    sla_rollup_id integer
);


--
-- Name: sla_node_reports_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.sla_node_reports_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: sla_node_reports_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.sla_node_reports_id_seq OWNED BY public.sla_node_reports.id;


--
-- Name: sla_rollups; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.sla_rollups (
    id integer NOT NULL,
    tx_hash text NOT NULL,
    block_start bigint NOT NULL,
    block_end bigint NOT NULL,
    "time" timestamp without time zone NOT NULL
);


--
-- Name: sla_rollups_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.sla_rollups_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: sla_rollups_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.sla_rollups_id_seq OWNED BY public.sla_rollups.id;


--
-- Name: sol_claimable_account_transfers; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.sol_claimable_account_transfers (
    signature character varying NOT NULL,
    instruction_index integer NOT NULL,
    amount bigint NOT NULL,
    slot bigint NOT NULL,
    from_account character varying NOT NULL,
    to_account character varying NOT NULL,
    sender_eth_address character varying NOT NULL
);


--
-- Name: TABLE sol_claimable_account_transfers; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.sol_claimable_account_transfers IS 'Stores claimable tokens program Transfer instructions for tracked mints.';


--
-- Name: sol_claimable_accounts; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.sol_claimable_accounts (
    signature character varying NOT NULL,
    instruction_index integer NOT NULL,
    slot bigint NOT NULL,
    mint character varying NOT NULL,
    ethereum_address character varying NOT NULL,
    account character varying NOT NULL
);


--
-- Name: TABLE sol_claimable_accounts; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.sol_claimable_accounts IS 'Stores claimable tokens program Create instructions for tracked mints.';


--
-- Name: sol_payments; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.sol_payments (
    signature character varying NOT NULL,
    instruction_index integer NOT NULL,
    amount bigint NOT NULL,
    slot bigint NOT NULL,
    route_index integer NOT NULL,
    to_account character varying NOT NULL
);


--
-- Name: TABLE sol_payments; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.sol_payments IS 'Stores payment router program Route instruction recipients and amounts for tracked mints.';


--
-- Name: sol_purchases; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.sol_purchases (
    signature character varying NOT NULL,
    instruction_index integer NOT NULL,
    amount bigint NOT NULL,
    slot bigint NOT NULL,
    from_account character varying NOT NULL,
    content_type character varying NOT NULL,
    content_id integer NOT NULL,
    buyer_user_id integer NOT NULL,
    access_type character varying NOT NULL,
    valid_after_blocknumber bigint NOT NULL,
    is_valid boolean,
    city character varying,
    region character varying,
    country character varying
);


--
-- Name: TABLE sol_purchases; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.sol_purchases IS 'Stores payment router program Route instructions that are paired with purchase information for tracked mints.';


--
-- Name: COLUMN sol_purchases.valid_after_blocknumber; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.sol_purchases.valid_after_blocknumber IS 'Purchase transactions include the blocknumber that the content was most recently updated in order to ensure that the relevant pricing information has been indexed before evaluating whether the purchase is valid.';


--
-- Name: COLUMN sol_purchases.is_valid; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.sol_purchases.is_valid IS 'A purchase is valid if it meets the pricing information set by the artist. If the pricing information is not available yet (as indicated by the valid_after_blocknumber), then is_valid will be NULL which indicates a "pending" state.';


--
-- Name: sol_reward_disbursements; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.sol_reward_disbursements (
    signature character varying NOT NULL,
    instruction_index integer NOT NULL,
    amount bigint NOT NULL,
    slot bigint NOT NULL,
    user_bank character varying NOT NULL,
    challenge_id character varying NOT NULL,
    specifier character varying NOT NULL
);


--
-- Name: TABLE sol_reward_disbursements; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.sol_reward_disbursements IS 'Stores reward manager program Evaluate instructions for tracked mints.';


--
-- Name: sol_slot_checkpoints; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.sol_slot_checkpoints (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    from_slot bigint NOT NULL,
    to_slot bigint NOT NULL,
    subscription_hash text NOT NULL,
    subscription jsonb NOT NULL,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);


--
-- Name: TABLE sol_slot_checkpoints; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.sol_slot_checkpoints IS 'Stores checkpoints for Solana slots to track indexing progress.';


--
-- Name: sol_swaps; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.sol_swaps (
    signature character varying NOT NULL,
    instruction_index integer NOT NULL,
    slot bigint NOT NULL,
    from_mint character varying NOT NULL,
    from_account character varying NOT NULL,
    from_amount bigint NOT NULL,
    to_mint character varying NOT NULL,
    to_account character varying NOT NULL,
    to_amount bigint NOT NULL
);


--
-- Name: TABLE sol_swaps; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.sol_swaps IS 'Stores eg. Jupiter swaps for tracked mints.';


--
-- Name: sol_token_account_balance_changes; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.sol_token_account_balance_changes (
    signature character varying NOT NULL,
    mint character varying NOT NULL,
    owner character varying NOT NULL,
    account character varying NOT NULL,
    change bigint NOT NULL,
    balance bigint NOT NULL,
    slot bigint NOT NULL,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    block_timestamp timestamp without time zone NOT NULL
);


--
-- Name: TABLE sol_token_account_balance_changes; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.sol_token_account_balance_changes IS 'Stores token balance changes for all accounts of tracked mints.';


--
-- Name: sol_token_account_balances; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.sol_token_account_balances (
    account character varying NOT NULL,
    mint character varying NOT NULL,
    owner character varying NOT NULL,
    balance bigint NOT NULL,
    slot bigint NOT NULL,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);


--
-- Name: TABLE sol_token_account_balances; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.sol_token_account_balances IS 'Stores current token balances for all accounts of tracked mints.';


--
-- Name: sol_token_transfers; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.sol_token_transfers (
    signature character varying NOT NULL,
    instruction_index integer NOT NULL,
    amount bigint NOT NULL,
    slot bigint NOT NULL,
    from_account character varying NOT NULL,
    to_account character varying NOT NULL
);


--
-- Name: TABLE sol_token_transfers; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.sol_token_transfers IS 'Stores SPL token transfers for tracked mints.';


--
-- Name: sol_user_balances; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.sol_user_balances (
    user_id integer NOT NULL,
    mint text NOT NULL,
    balance bigint NOT NULL,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);


--
-- Name: TABLE sol_user_balances; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.sol_user_balances IS 'Stores the balances of Solana tokens for users.';


--
-- Name: sound_recordings; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.sound_recordings (
    id integer NOT NULL,
    sound_recording_id text NOT NULL,
    track_id text NOT NULL,
    cid text NOT NULL,
    encoding_details text
);


--
-- Name: sound_recordings_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.sound_recordings_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: sound_recordings_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.sound_recordings_id_seq OWNED BY public.sound_recordings.id;


--
-- Name: spl_token_tx; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.spl_token_tx (
    last_scanned_slot integer NOT NULL,
    signature character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: stems; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.stems (
    parent_track_id integer NOT NULL,
    child_track_id integer NOT NULL
);


--
-- Name: storage_proof_peers; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.storage_proof_peers (
    id integer NOT NULL,
    block_height bigint NOT NULL,
    prover_addresses text[] NOT NULL
);


--
-- Name: storage_proof_peers_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.storage_proof_peers_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: storage_proof_peers_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.storage_proof_peers_id_seq OWNED BY public.storage_proof_peers.id;


--
-- Name: storage_proofs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.storage_proofs (
    id integer NOT NULL,
    block_height bigint NOT NULL,
    address text NOT NULL,
    cid text,
    proof_signature text,
    proof text,
    prover_addresses text[] DEFAULT '{}'::text[] NOT NULL,
    status public.proof_status DEFAULT 'unresolved'::public.proof_status NOT NULL
);


--
-- Name: storage_proofs_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.storage_proofs_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: storage_proofs_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.storage_proofs_id_seq OWNED BY public.storage_proofs.id;


--
-- Name: subscriptions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.subscriptions (
    blockhash character varying,
    blocknumber integer,
    subscriber_id integer NOT NULL,
    user_id integer NOT NULL,
    is_current boolean NOT NULL,
    is_delete boolean NOT NULL,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    txhash character varying DEFAULT ''::character varying NOT NULL
);


--
-- Name: supporter_rank_ups; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.supporter_rank_ups (
    slot integer NOT NULL,
    sender_user_id integer NOT NULL,
    receiver_user_id integer NOT NULL,
    rank integer NOT NULL
);


--
-- Name: tag_track_user; Type: MATERIALIZED VIEW; Schema: public; Owner: -
--

CREATE MATERIALIZED VIEW public.tag_track_user AS
 SELECT unnest(tags) AS tag,
    track_id,
    owner_id
   FROM ( SELECT string_to_array(lower((tracks.tags)::text), ','::text) AS tags,
            tracks.track_id,
            tracks.owner_id
           FROM public.tracks
          WHERE (((tracks.tags)::text <> ''::text) AND (tracks.tags IS NOT NULL) AND (tracks.is_current IS TRUE) AND (tracks.is_unlisted IS FALSE) AND (tracks.stem_of IS NULL))
          ORDER BY tracks.updated_at DESC) t
  GROUP BY (unnest(tags)), track_id, owner_id
  WITH NO DATA;


--
-- Name: track_delist_statuses; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.track_delist_statuses (
    created_at timestamp with time zone NOT NULL,
    track_id integer NOT NULL,
    owner_id integer NOT NULL,
    track_cid character varying NOT NULL,
    delisted boolean NOT NULL,
    reason public.delist_track_reason NOT NULL
);


--
-- Name: track_downloads; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.track_downloads (
    txhash character varying NOT NULL,
    blocknumber integer NOT NULL,
    parent_track_id integer NOT NULL,
    track_id integer NOT NULL,
    user_id integer,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    city character varying,
    region character varying,
    country character varying
);


--
-- Name: track_price_history; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.track_price_history (
    track_id integer NOT NULL,
    splits jsonb NOT NULL,
    total_price_cents bigint NOT NULL,
    blocknumber integer NOT NULL,
    block_timestamp timestamp without time zone NOT NULL,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    access public.usdc_purchase_access_type DEFAULT 'stream'::public.usdc_purchase_access_type NOT NULL
);


--
-- Name: track_releases; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.track_releases (
    id integer NOT NULL,
    track_id text NOT NULL
);


--
-- Name: track_releases_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.track_releases_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: track_releases_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.track_releases_id_seq OWNED BY public.track_releases.id;


--
-- Name: track_routes; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.track_routes (
    slug character varying NOT NULL,
    title_slug character varying NOT NULL,
    collision_id integer NOT NULL,
    owner_id integer NOT NULL,
    track_id integer NOT NULL,
    is_current boolean NOT NULL,
    blockhash character varying NOT NULL,
    blocknumber integer NOT NULL,
    txhash character varying NOT NULL
);


--
-- Name: track_trending_scores; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.track_trending_scores (
    track_id integer NOT NULL,
    type character varying NOT NULL,
    genre character varying,
    version character varying NOT NULL,
    time_range character varying NOT NULL,
    score double precision NOT NULL,
    created_at timestamp without time zone NOT NULL
);


--
-- Name: users; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.users (
    blockhash character varying,
    user_id integer NOT NULL,
    is_current boolean NOT NULL,
    handle character varying,
    wallet character varying,
    name text,
    profile_picture character varying,
    cover_photo character varying,
    bio character varying,
    location character varying,
    metadata_multihash character varying,
    creator_node_endpoint character varying,
    blocknumber integer,
    is_verified boolean DEFAULT false NOT NULL,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    handle_lc character varying,
    cover_photo_sizes character varying,
    profile_picture_sizes character varying,
    primary_id integer,
    secondary_ids integer[],
    replica_set_update_signer character varying,
    has_collectibles boolean DEFAULT false NOT NULL,
    txhash character varying DEFAULT ''::character varying NOT NULL,
    playlist_library jsonb,
    is_deactivated boolean DEFAULT false NOT NULL,
    slot integer,
    user_storage_account character varying,
    user_authority_account character varying,
    artist_pick_track_id integer,
    is_available boolean DEFAULT true NOT NULL,
    is_storage_v2 boolean DEFAULT false NOT NULL,
    allow_ai_attribution boolean DEFAULT false NOT NULL,
    spl_usdc_payout_wallet character varying,
    twitter_handle character varying,
    instagram_handle character varying,
    tiktok_handle character varying,
    verified_with_twitter boolean DEFAULT false,
    verified_with_instagram boolean DEFAULT false,
    verified_with_tiktok boolean DEFAULT false,
    website character varying,
    donation character varying,
    profile_type public.profile_type_enum
);


--
-- Name: trending_params; Type: MATERIALIZED VIEW; Schema: public; Owner: -
--

CREATE MATERIALIZED VIEW public.trending_params AS
 SELECT t.track_id,
    t.release_date,
    t.genre,
    t.owner_id,
    ap.play_count,
    au.follower_count AS owner_follower_count,
    COALESCE(aggregate_track.repost_count, 0) AS repost_count,
    COALESCE(aggregate_track.save_count, 0) AS save_count,
    COALESCE(repost_week.repost_count, (0)::bigint) AS repost_week_count,
    COALESCE(repost_month.repost_count, (0)::bigint) AS repost_month_count,
    COALESCE(repost_year.repost_count, (0)::bigint) AS repost_year_count,
    COALESCE(save_week.repost_count, (0)::bigint) AS save_week_count,
    COALESCE(save_month.repost_count, (0)::bigint) AS save_month_count,
    COALESCE(save_year.repost_count, (0)::bigint) AS save_year_count,
    COALESCE(karma.karma, (0)::numeric) AS karma
   FROM ((((((((((public.tracks t
     LEFT JOIN ( SELECT ap_1.count AS play_count,
            ap_1.play_item_id
           FROM public.aggregate_plays ap_1) ap ON ((ap.play_item_id = t.track_id)))
     LEFT JOIN ( SELECT au_1.user_id,
            au_1.follower_count
           FROM public.aggregate_user au_1) au ON ((au.user_id = t.owner_id)))
     LEFT JOIN ( SELECT aggregate_track_1.track_id,
            aggregate_track_1.repost_count,
            aggregate_track_1.save_count
           FROM public.aggregate_track aggregate_track_1) aggregate_track ON ((aggregate_track.track_id = t.track_id)))
     LEFT JOIN ( SELECT r.repost_item_id AS track_id,
            count(r.repost_item_id) AS repost_count
           FROM public.reposts r
          WHERE ((r.is_current IS TRUE) AND (r.repost_type = 'track'::public.reposttype) AND (r.is_delete IS FALSE) AND (r.created_at > (now() - '1 year'::interval)))
          GROUP BY r.repost_item_id) repost_year ON ((repost_year.track_id = t.track_id)))
     LEFT JOIN ( SELECT r.repost_item_id AS track_id,
            count(r.repost_item_id) AS repost_count
           FROM public.reposts r
          WHERE ((r.is_current IS TRUE) AND (r.repost_type = 'track'::public.reposttype) AND (r.is_delete IS FALSE) AND (r.created_at > (now() - '1 mon'::interval)))
          GROUP BY r.repost_item_id) repost_month ON ((repost_month.track_id = t.track_id)))
     LEFT JOIN ( SELECT r.repost_item_id AS track_id,
            count(r.repost_item_id) AS repost_count
           FROM public.reposts r
          WHERE ((r.is_current IS TRUE) AND (r.repost_type = 'track'::public.reposttype) AND (r.is_delete IS FALSE) AND (r.created_at > (now() - '7 days'::interval)))
          GROUP BY r.repost_item_id) repost_week ON ((repost_week.track_id = t.track_id)))
     LEFT JOIN ( SELECT r.save_item_id AS track_id,
            count(r.save_item_id) AS repost_count
           FROM public.saves r
          WHERE ((r.is_current IS TRUE) AND (r.save_type = 'track'::public.savetype) AND (r.is_delete IS FALSE) AND (r.created_at > (now() - '1 year'::interval)))
          GROUP BY r.save_item_id) save_year ON ((save_year.track_id = t.track_id)))
     LEFT JOIN ( SELECT r.save_item_id AS track_id,
            count(r.save_item_id) AS repost_count
           FROM public.saves r
          WHERE ((r.is_current IS TRUE) AND (r.save_type = 'track'::public.savetype) AND (r.is_delete IS FALSE) AND (r.created_at > (now() - '1 mon'::interval)))
          GROUP BY r.save_item_id) save_month ON ((save_month.track_id = t.track_id)))
     LEFT JOIN ( SELECT r.save_item_id AS track_id,
            count(r.save_item_id) AS repost_count
           FROM public.saves r
          WHERE ((r.is_current IS TRUE) AND (r.save_type = 'track'::public.savetype) AND (r.is_delete IS FALSE) AND (r.created_at > (now() - '7 days'::interval)))
          GROUP BY r.save_item_id) save_week ON ((save_week.track_id = t.track_id)))
     LEFT JOIN ( SELECT save_and_reposts.item_id AS track_id,
            sum(au_1.follower_count) AS karma
           FROM (( SELECT r_and_s.user_id,
                    r_and_s.item_id
                   FROM (( SELECT reposts.user_id,
                            reposts.repost_item_id AS item_id
                           FROM public.reposts
                          WHERE ((reposts.is_delete IS FALSE) AND (reposts.is_current IS TRUE) AND (reposts.repost_type = 'track'::public.reposttype))
                        UNION ALL
                         SELECT saves.user_id,
                            saves.save_item_id AS item_id
                           FROM public.saves
                          WHERE ((saves.is_delete IS FALSE) AND (saves.is_current IS TRUE) AND (saves.save_type = 'track'::public.savetype))) r_and_s
                     JOIN public.users ON ((r_and_s.user_id = users.user_id)))
                  WHERE (((users.cover_photo IS NOT NULL) OR (users.cover_photo_sizes IS NOT NULL)) AND ((users.profile_picture IS NOT NULL) OR (users.profile_picture_sizes IS NOT NULL)) AND (users.bio IS NOT NULL))) save_and_reposts
             JOIN public.aggregate_user au_1 ON ((save_and_reposts.user_id = au_1.user_id)))
          GROUP BY save_and_reposts.item_id) karma ON ((karma.track_id = t.track_id)))
  WHERE ((t.is_current IS TRUE) AND (t.is_delete IS FALSE) AND (t.is_unlisted IS FALSE) AND (t.stem_of IS NULL))
  WITH NO DATA;


--
-- Name: trending_results; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.trending_results (
    user_id integer NOT NULL,
    id character varying,
    rank integer NOT NULL,
    type character varying NOT NULL,
    version character varying NOT NULL,
    week date NOT NULL
);


--
-- Name: usdc_purchases; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.usdc_purchases (
    slot integer NOT NULL,
    signature character varying NOT NULL,
    buyer_user_id integer NOT NULL,
    seller_user_id integer NOT NULL,
    amount bigint NOT NULL,
    content_type public.usdc_purchase_content_type NOT NULL,
    content_id integer NOT NULL,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    extra_amount bigint DEFAULT 0 NOT NULL,
    access public.usdc_purchase_access_type DEFAULT 'stream'::public.usdc_purchase_access_type NOT NULL,
    city character varying,
    region character varying,
    country character varying,
    vendor character varying,
    splits jsonb NOT NULL
);


--
-- Name: usdc_transactions_history; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.usdc_transactions_history (
    user_bank character varying NOT NULL,
    slot integer NOT NULL,
    signature character varying NOT NULL,
    transaction_type character varying NOT NULL,
    method character varying NOT NULL,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    transaction_created_at timestamp without time zone NOT NULL,
    change numeric NOT NULL,
    balance numeric NOT NULL,
    tx_metadata character varying
);


--
-- Name: usdc_user_bank_accounts; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.usdc_user_bank_accounts (
    signature character varying NOT NULL,
    ethereum_address character varying NOT NULL,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    bank_account character varying NOT NULL
);


--
-- Name: user_balance_changes; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_balance_changes (
    user_id integer NOT NULL,
    blocknumber integer NOT NULL,
    current_balance character varying NOT NULL,
    previous_balance character varying NOT NULL,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);


--
-- Name: user_balance_changes_user_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.user_balance_changes_user_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: user_balance_changes_user_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.user_balance_changes_user_id_seq OWNED BY public.user_balance_changes.user_id;


--
-- Name: user_balances; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_balances (
    user_id integer NOT NULL,
    balance character varying NOT NULL,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    associated_wallets_balance character varying DEFAULT '0'::character varying NOT NULL,
    waudio character varying DEFAULT '0'::character varying,
    associated_sol_wallets_balance character varying DEFAULT '0'::character varying NOT NULL
);


--
-- Name: user_balances_user_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.user_balances_user_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: user_balances_user_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.user_balances_user_id_seq OWNED BY public.user_balances.user_id;


--
-- Name: user_bank_accounts; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_bank_accounts (
    signature character varying NOT NULL,
    ethereum_address character varying NOT NULL,
    created_at timestamp without time zone NOT NULL,
    bank_account character varying NOT NULL
);


--
-- Name: user_bank_txs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_bank_txs (
    signature character varying NOT NULL,
    slot integer NOT NULL,
    created_at timestamp without time zone NOT NULL
);


--
-- Name: user_challenges; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_challenges (
    challenge_id character varying NOT NULL,
    user_id integer NOT NULL,
    specifier character varying NOT NULL,
    is_complete boolean NOT NULL,
    current_step_count integer,
    completed_blocknumber integer,
    amount integer DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    completed_at timestamp without time zone
);


--
-- Name: user_delist_statuses; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_delist_statuses (
    created_at timestamp with time zone NOT NULL,
    user_id integer NOT NULL,
    delisted boolean NOT NULL,
    reason public.delist_user_reason NOT NULL
);


--
-- Name: user_events; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_events (
    id integer NOT NULL,
    blockhash character varying,
    blocknumber integer,
    is_current boolean NOT NULL,
    user_id integer NOT NULL,
    referrer integer,
    is_mobile_user boolean DEFAULT false NOT NULL,
    slot integer
);


--
-- Name: user_events_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.user_events_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: user_events_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.user_events_id_seq OWNED BY public.user_events.id;


--
-- Name: user_listening_history; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_listening_history (
    user_id integer NOT NULL,
    listening_history jsonb NOT NULL
);


--
-- Name: user_listening_history_user_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.user_listening_history_user_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: user_listening_history_user_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.user_listening_history_user_id_seq OWNED BY public.user_listening_history.user_id;


--
-- Name: user_payout_wallet_history; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_payout_wallet_history (
    user_id integer NOT NULL,
    spl_usdc_payout_wallet character varying,
    blocknumber integer NOT NULL,
    block_timestamp timestamp without time zone NOT NULL,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);


--
-- Name: user_pubkeys; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_pubkeys (
    user_id integer NOT NULL,
    pubkey_base64 text NOT NULL
);


--
-- Name: user_tips; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_tips (
    slot integer NOT NULL,
    signature character varying NOT NULL,
    sender_user_id integer NOT NULL,
    receiver_user_id integer NOT NULL,
    amount bigint NOT NULL,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);


--
-- Name: access_keys id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.access_keys ALTER COLUMN id SET DEFAULT nextval('public.access_keys_id_seq'::regclass);


--
-- Name: aggregate_daily_app_name_metrics id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aggregate_daily_app_name_metrics ALTER COLUMN id SET DEFAULT nextval('public.aggregate_daily_app_name_metrics_id_seq'::regclass);


--
-- Name: aggregate_daily_total_users_metrics id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aggregate_daily_total_users_metrics ALTER COLUMN id SET DEFAULT nextval('public.aggregate_daily_total_users_metrics_id_seq'::regclass);


--
-- Name: aggregate_daily_unique_users_metrics id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aggregate_daily_unique_users_metrics ALTER COLUMN id SET DEFAULT nextval('public.aggregate_daily_unique_users_metrics_id_seq'::regclass);


--
-- Name: aggregate_monthly_app_name_metrics id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aggregate_monthly_app_name_metrics ALTER COLUMN id SET DEFAULT nextval('public.aggregate_monthly_app_name_metrics_id_seq'::regclass);


--
-- Name: aggregate_monthly_total_users_metrics id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aggregate_monthly_total_users_metrics ALTER COLUMN id SET DEFAULT nextval('public.aggregate_monthly_total_users_metrics_id_seq'::regclass);


--
-- Name: aggregate_monthly_unique_users_metrics id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aggregate_monthly_unique_users_metrics ALTER COLUMN id SET DEFAULT nextval('public.aggregate_monthly_unique_users_metrics_id_seq'::regclass);


--
-- Name: associated_wallets id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.associated_wallets ALTER COLUMN id SET DEFAULT nextval('public.associated_wallets_id_seq'::regclass);


--
-- Name: challenge_listen_streak user_id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.challenge_listen_streak ALTER COLUMN user_id SET DEFAULT nextval('public.challenge_listen_streak_user_id_seq'::regclass);


--
-- Name: challenge_profile_completion user_id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.challenge_profile_completion ALTER COLUMN user_id SET DEFAULT nextval('public.challenge_profile_completion_user_id_seq'::regclass);


--
-- Name: core_blocks rowid; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.core_blocks ALTER COLUMN rowid SET DEFAULT nextval('public.core_blocks_rowid_seq'::regclass);


--
-- Name: core_transactions rowid; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.core_transactions ALTER COLUMN rowid SET DEFAULT nextval('public.core_transactions_rowid_seq'::regclass);


--
-- Name: core_tx_stats id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.core_tx_stats ALTER COLUMN id SET DEFAULT nextval('public.core_tx_stats_id_seq'::regclass);


--
-- Name: core_validators rowid; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.core_validators ALTER COLUMN rowid SET DEFAULT nextval('public.core_validators_rowid_seq'::regclass);


--
-- Name: email_access id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.email_access ALTER COLUMN id SET DEFAULT nextval('public.email_access_id_seq'::regclass);


--
-- Name: encrypted_emails id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.encrypted_emails ALTER COLUMN id SET DEFAULT nextval('public.encrypted_emails_id_seq'::regclass);


--
-- Name: eth_blocks last_scanned_block; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.eth_blocks ALTER COLUMN last_scanned_block SET DEFAULT nextval('public.eth_blocks_last_scanned_block_seq'::regclass);


--
-- Name: management_keys id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.management_keys ALTER COLUMN id SET DEFAULT nextval('public.management_keys_id_seq'::regclass);


--
-- Name: notification id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.notification ALTER COLUMN id SET DEFAULT nextval('public.notification_id_seq'::regclass);


--
-- Name: plays id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.plays ALTER COLUMN id SET DEFAULT nextval('public.plays_id_seq'::regclass);


--
-- Name: reactions id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.reactions ALTER COLUMN id SET DEFAULT nextval('public.reactions_id_seq'::regclass);


--
-- Name: skipped_transactions id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.skipped_transactions ALTER COLUMN id SET DEFAULT nextval('public.skipped_transactions_id_seq'::regclass);


--
-- Name: sla_auditor_version_data id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sla_auditor_version_data ALTER COLUMN id SET DEFAULT nextval('public.sla_auditor_version_data_id_seq'::regclass);


--
-- Name: sla_node_reports id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sla_node_reports ALTER COLUMN id SET DEFAULT nextval('public.sla_node_reports_id_seq'::regclass);


--
-- Name: sla_rollups id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sla_rollups ALTER COLUMN id SET DEFAULT nextval('public.sla_rollups_id_seq'::regclass);


--
-- Name: sound_recordings id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sound_recordings ALTER COLUMN id SET DEFAULT nextval('public.sound_recordings_id_seq'::regclass);


--
-- Name: storage_proof_peers id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.storage_proof_peers ALTER COLUMN id SET DEFAULT nextval('public.storage_proof_peers_id_seq'::regclass);


--
-- Name: storage_proofs id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.storage_proofs ALTER COLUMN id SET DEFAULT nextval('public.storage_proofs_id_seq'::regclass);


--
-- Name: track_releases id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.track_releases ALTER COLUMN id SET DEFAULT nextval('public.track_releases_id_seq'::regclass);


--
-- Name: user_balance_changes user_id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_balance_changes ALTER COLUMN user_id SET DEFAULT nextval('public.user_balance_changes_user_id_seq'::regclass);


--
-- Name: user_balances user_id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_balances ALTER COLUMN user_id SET DEFAULT nextval('public.user_balances_user_id_seq'::regclass);


--
-- Name: user_events id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_events ALTER COLUMN id SET DEFAULT nextval('public.user_events_id_seq'::regclass);


--
-- Name: user_listening_history user_id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_listening_history ALTER COLUMN user_id SET DEFAULT nextval('public.user_listening_history_user_id_seq'::regclass);


--
-- Name: access_keys access_keys_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.access_keys
    ADD CONSTRAINT access_keys_pkey PRIMARY KEY (id);


--
-- Name: aggregate_daily_app_name_metrics aggregate_daily_app_name_metrics_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aggregate_daily_app_name_metrics
    ADD CONSTRAINT aggregate_daily_app_name_metrics_pkey PRIMARY KEY (id);


--
-- Name: aggregate_daily_total_users_metrics aggregate_daily_total_users_metrics_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aggregate_daily_total_users_metrics
    ADD CONSTRAINT aggregate_daily_total_users_metrics_pkey PRIMARY KEY (id);


--
-- Name: aggregate_daily_unique_users_metrics aggregate_daily_unique_users_metrics_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aggregate_daily_unique_users_metrics
    ADD CONSTRAINT aggregate_daily_unique_users_metrics_pkey PRIMARY KEY (id);


--
-- Name: aggregate_monthly_app_name_metrics aggregate_monthly_app_name_metrics_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aggregate_monthly_app_name_metrics
    ADD CONSTRAINT aggregate_monthly_app_name_metrics_pkey PRIMARY KEY (id);


--
-- Name: aggregate_monthly_plays aggregate_monthly_plays_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aggregate_monthly_plays
    ADD CONSTRAINT aggregate_monthly_plays_pkey PRIMARY KEY (play_item_id, "timestamp", country);


--
-- Name: aggregate_monthly_total_users_metrics aggregate_monthly_total_users_metrics_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aggregate_monthly_total_users_metrics
    ADD CONSTRAINT aggregate_monthly_total_users_metrics_pkey PRIMARY KEY (id);


--
-- Name: aggregate_monthly_unique_users_metrics aggregate_monthly_unique_users_metrics_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aggregate_monthly_unique_users_metrics
    ADD CONSTRAINT aggregate_monthly_unique_users_metrics_pkey PRIMARY KEY (id);


--
-- Name: aggregate_playlist aggregate_playlist_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aggregate_playlist
    ADD CONSTRAINT aggregate_playlist_pkey PRIMARY KEY (playlist_id);


--
-- Name: aggregate_track aggregate_track_table_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aggregate_track
    ADD CONSTRAINT aggregate_track_table_pkey PRIMARY KEY (track_id);


--
-- Name: aggregate_user aggregate_user_table_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aggregate_user
    ADD CONSTRAINT aggregate_user_table_pkey PRIMARY KEY (user_id);


--
-- Name: aggregate_user_tips aggregate_user_tips_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aggregate_user_tips
    ADD CONSTRAINT aggregate_user_tips_pkey PRIMARY KEY (sender_user_id, receiver_user_id);


--
-- Name: album_price_history album_price_history_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.album_price_history
    ADD CONSTRAINT album_price_history_pkey PRIMARY KEY (playlist_id, block_timestamp);


--
-- Name: alembic_version alembic_version_pkc; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.alembic_version
    ADD CONSTRAINT alembic_version_pkc PRIMARY KEY (version_num);


--
-- Name: anti_abuse_blocked_users anti_abuse_blocked_users_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.anti_abuse_blocked_users
    ADD CONSTRAINT anti_abuse_blocked_users_pkey PRIMARY KEY (handle_lc);


--
-- Name: api_metrics_apps api_metrics_apps_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.api_metrics_apps
    ADD CONSTRAINT api_metrics_apps_pkey PRIMARY KEY (date, api_key, app_name);


--
-- Name: api_metrics_counts api_metrics_counts_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.api_metrics_counts
    ADD CONSTRAINT api_metrics_counts_pkey PRIMARY KEY (date);


--
-- Name: api_metrics_routes api_metrics_routes_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.api_metrics_routes
    ADD CONSTRAINT api_metrics_routes_pkey PRIMARY KEY (date, route_pattern, method);


--
-- Name: app_name_metrics app_name_metrics_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.app_name_metrics
    ADD CONSTRAINT app_name_metrics_pkey PRIMARY KEY (id);


--
-- Name: artist_coins artist_coins_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.artist_coins
    ADD CONSTRAINT artist_coins_pkey PRIMARY KEY (mint);


--
-- Name: associated_wallets associated_wallets_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.associated_wallets
    ADD CONSTRAINT associated_wallets_pkey PRIMARY KEY (id);


--
-- Name: audio_transactions_history audio_transactions_history_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.audio_transactions_history
    ADD CONSTRAINT audio_transactions_history_pkey PRIMARY KEY (user_bank, signature);


--
-- Name: audius_data_txs audius_data_txs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.audius_data_txs
    ADD CONSTRAINT audius_data_txs_pkey PRIMARY KEY (signature);


--
-- Name: blocks blocks_number_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.blocks
    ADD CONSTRAINT blocks_number_key UNIQUE (number);


--
-- Name: blocks blocks_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.blocks
    ADD CONSTRAINT blocks_pkey PRIMARY KEY (blockhash);


--
-- Name: challenge_disbursements challenge_disbursements_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.challenge_disbursements
    ADD CONSTRAINT challenge_disbursements_pkey PRIMARY KEY (challenge_id, specifier);


--
-- Name: challenge_listen_streak challenge_listen_streak_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.challenge_listen_streak
    ADD CONSTRAINT challenge_listen_streak_pkey PRIMARY KEY (user_id);


--
-- Name: challenge_profile_completion challenge_profile_completion_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.challenge_profile_completion
    ADD CONSTRAINT challenge_profile_completion_pkey PRIMARY KEY (user_id);


--
-- Name: challenges challenges_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.challenges
    ADD CONSTRAINT challenges_pkey PRIMARY KEY (id);


--
-- Name: chat_ban chat_ban_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.chat_ban
    ADD CONSTRAINT chat_ban_pkey PRIMARY KEY (user_id);


--
-- Name: chat_blast chat_blast_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.chat_blast
    ADD CONSTRAINT chat_blast_pkey PRIMARY KEY (blast_id);


--
-- Name: chat_blocked_users chat_blocked_users_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.chat_blocked_users
    ADD CONSTRAINT chat_blocked_users_pkey PRIMARY KEY (blocker_user_id, blockee_user_id);


--
-- Name: chat_member chat_member_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.chat_member
    ADD CONSTRAINT chat_member_pkey PRIMARY KEY (chat_id, user_id);


--
-- Name: chat_message chat_message_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.chat_message
    ADD CONSTRAINT chat_message_pkey PRIMARY KEY (message_id);


--
-- Name: chat_message_reactions chat_message_reactions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.chat_message_reactions
    ADD CONSTRAINT chat_message_reactions_pkey PRIMARY KEY (user_id, message_id);


--
-- Name: chat_permissions chat_permissions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.chat_permissions
    ADD CONSTRAINT chat_permissions_pkey PRIMARY KEY (user_id, permits);


--
-- Name: chat chat_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.chat
    ADD CONSTRAINT chat_pkey PRIMARY KEY (chat_id);


--
-- Name: cid_data cid_data_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cid_data
    ADD CONSTRAINT cid_data_pkey PRIMARY KEY (cid);


--
-- Name: comment_mentions comment_mentions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.comment_mentions
    ADD CONSTRAINT comment_mentions_pkey PRIMARY KEY (comment_id, user_id);


--
-- Name: comment_notification_settings comment_notification_settings_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.comment_notification_settings
    ADD CONSTRAINT comment_notification_settings_pkey PRIMARY KEY (user_id, entity_id, entity_type);


--
-- Name: comment_reactions comment_reactions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.comment_reactions
    ADD CONSTRAINT comment_reactions_pkey PRIMARY KEY (comment_id, user_id);


--
-- Name: comment_reports comment_reports_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.comment_reports
    ADD CONSTRAINT comment_reports_pkey PRIMARY KEY (comment_id, user_id);


--
-- Name: comment_threads comment_threads_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.comment_threads
    ADD CONSTRAINT comment_threads_pkey PRIMARY KEY (parent_comment_id, comment_id);


--
-- Name: comments comments_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.comments
    ADD CONSTRAINT comments_pkey PRIMARY KEY (comment_id);


--
-- Name: core_app_state core_app_state_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.core_app_state
    ADD CONSTRAINT core_app_state_pkey PRIMARY KEY (block_height, app_hash);


--
-- Name: core_blocks core_blocks_height_chain_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.core_blocks
    ADD CONSTRAINT core_blocks_height_chain_id_key UNIQUE (height, chain_id);


--
-- Name: core_blocks core_blocks_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.core_blocks
    ADD CONSTRAINT core_blocks_pkey PRIMARY KEY (rowid);


--
-- Name: core_db_migrations core_db_migrations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.core_db_migrations
    ADD CONSTRAINT core_db_migrations_pkey PRIMARY KEY (id);


--
-- Name: core_transactions core_transactions_block_id_index_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.core_transactions
    ADD CONSTRAINT core_transactions_block_id_index_key UNIQUE (block_id, index);


--
-- Name: core_transactions core_transactions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.core_transactions
    ADD CONSTRAINT core_transactions_pkey PRIMARY KEY (rowid);


--
-- Name: core_tx_stats core_tx_stats_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.core_tx_stats
    ADD CONSTRAINT core_tx_stats_pkey PRIMARY KEY (id);


--
-- Name: core_tx_stats core_tx_stats_tx_hash_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.core_tx_stats
    ADD CONSTRAINT core_tx_stats_tx_hash_key UNIQUE (tx_hash);


--
-- Name: core_validators core_validators_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.core_validators
    ADD CONSTRAINT core_validators_pkey PRIMARY KEY (rowid);


--
-- Name: countries countries_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.countries
    ADD CONSTRAINT countries_pkey PRIMARY KEY (iso);


--
-- Name: dashboard_wallet_users dashboard_wallet_users_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dashboard_wallet_users
    ADD CONSTRAINT dashboard_wallet_users_pkey PRIMARY KEY (user_id, wallet);


--
-- Name: delist_status_cursor delist_status_cursor_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.delist_status_cursor
    ADD CONSTRAINT delist_status_cursor_pkey PRIMARY KEY (host, entity);


--
-- Name: developer_apps developer_apps_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.developer_apps
    ADD CONSTRAINT developer_apps_pkey PRIMARY KEY (address, txhash);


--
-- Name: email_access email_access_email_owner_user_id_receiving_user_id_grantor__key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.email_access
    ADD CONSTRAINT email_access_email_owner_user_id_receiving_user_id_grantor__key UNIQUE (email_owner_user_id, receiving_user_id, grantor_user_id);


--
-- Name: email_access email_access_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.email_access
    ADD CONSTRAINT email_access_pkey PRIMARY KEY (id);


--
-- Name: encrypted_emails encrypted_emails_email_owner_user_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.encrypted_emails
    ADD CONSTRAINT encrypted_emails_email_owner_user_id_key UNIQUE (email_owner_user_id);


--
-- Name: encrypted_emails encrypted_emails_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.encrypted_emails
    ADD CONSTRAINT encrypted_emails_pkey PRIMARY KEY (id);


--
-- Name: eth_blocks eth_blocks_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.eth_blocks
    ADD CONSTRAINT eth_blocks_pkey PRIMARY KEY (last_scanned_block);


--
-- Name: eth_db_migrations eth_db_migrations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.eth_db_migrations
    ADD CONSTRAINT eth_db_migrations_pkey PRIMARY KEY (version);


--
-- Name: eth_funding_rounds eth_funding_rounds_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.eth_funding_rounds
    ADD CONSTRAINT eth_funding_rounds_pkey PRIMARY KEY (round_num);


--
-- Name: eth_registered_endpoints eth_registered_endpoints_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.eth_registered_endpoints
    ADD CONSTRAINT eth_registered_endpoints_pkey PRIMARY KEY (id, service_type);


--
-- Name: eth_service_providers eth_service_providers_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.eth_service_providers
    ADD CONSTRAINT eth_service_providers_pkey PRIMARY KEY (address);


--
-- Name: events events_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.events
    ADD CONSTRAINT events_pkey PRIMARY KEY (event_id);


--
-- Name: follows follows_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.follows
    ADD CONSTRAINT follows_pkey PRIMARY KEY (follower_user_id, followee_user_id, txhash);


--
-- Name: grants grants_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.grants
    ADD CONSTRAINT grants_pkey PRIMARY KEY (grantee_address, user_id, txhash);


--
-- Name: hourly_play_counts hourly_play_counts_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.hourly_play_counts
    ADD CONSTRAINT hourly_play_counts_pkey PRIMARY KEY (hourly_timestamp);


--
-- Name: indexing_checkpoints indexing_checkpoints_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.indexing_checkpoints
    ADD CONSTRAINT indexing_checkpoints_pkey PRIMARY KEY (tablename);


--
-- Name: management_keys management_keys_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.management_keys
    ADD CONSTRAINT management_keys_pkey PRIMARY KEY (id);


--
-- Name: milestones milestones_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.milestones
    ADD CONSTRAINT milestones_pkey PRIMARY KEY (id, name, threshold);


--
-- Name: muted_users muted_users_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.muted_users
    ADD CONSTRAINT muted_users_pkey PRIMARY KEY (muted_user_id, user_id);


--
-- Name: notification notification_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.notification
    ADD CONSTRAINT notification_pkey PRIMARY KEY (id);


--
-- Name: notification_seen notification_seen_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.notification_seen
    ADD CONSTRAINT notification_seen_pkey PRIMARY KEY (user_id, seen_at);


--
-- Name: core_indexed_blocks pk_chain_id_height; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.core_indexed_blocks
    ADD CONSTRAINT pk_chain_id_height PRIMARY KEY (chain_id, height);


--
-- Name: collectibles pk_user_id; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.collectibles
    ADD CONSTRAINT pk_user_id PRIMARY KEY (user_id);


--
-- Name: aggregate_plays play_item_id_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aggregate_plays
    ADD CONSTRAINT play_item_id_pkey PRIMARY KEY (play_item_id);


--
-- Name: playlist_routes playlist_routes_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.playlist_routes
    ADD CONSTRAINT playlist_routes_pkey PRIMARY KEY (owner_id, slug);


--
-- Name: playlist_seen playlist_seen_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.playlist_seen
    ADD CONSTRAINT playlist_seen_pkey PRIMARY KEY (user_id, playlist_id, seen_at);


--
-- Name: playlist_tracks playlist_tracks_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.playlist_tracks
    ADD CONSTRAINT playlist_tracks_pkey PRIMARY KEY (playlist_id, track_id);


--
-- Name: playlist_trending_scores playlist_trending_scores_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.playlist_trending_scores
    ADD CONSTRAINT playlist_trending_scores_pkey PRIMARY KEY (playlist_id, type, version, time_range);


--
-- Name: playlists playlists_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.playlists
    ADD CONSTRAINT playlists_pkey PRIMARY KEY (playlist_id, txhash);


--
-- Name: plays plays_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.plays
    ADD CONSTRAINT plays_pkey PRIMARY KEY (id);


--
-- Name: reactions reactions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.reactions
    ADD CONSTRAINT reactions_pkey PRIMARY KEY (id);


--
-- Name: related_artists related_artists_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.related_artists
    ADD CONSTRAINT related_artists_pkey PRIMARY KEY (user_id, related_artist_user_id);


--
-- Name: remixes remixes_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.remixes
    ADD CONSTRAINT remixes_pkey PRIMARY KEY (parent_track_id, child_track_id);


--
-- Name: reported_comments reported_comments_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.reported_comments
    ADD CONSTRAINT reported_comments_pkey PRIMARY KEY (reported_comment_id, user_id);


--
-- Name: reposts reposts_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.reposts
    ADD CONSTRAINT reposts_pkey PRIMARY KEY (user_id, repost_item_id, repost_type, txhash);


--
-- Name: revert_blocks revert_blocks_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.revert_blocks
    ADD CONSTRAINT revert_blocks_pkey PRIMARY KEY (blocknumber);


--
-- Name: reward_manager_txs reward_manager_txs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.reward_manager_txs
    ADD CONSTRAINT reward_manager_txs_pkey PRIMARY KEY (signature);


--
-- Name: route_metrics route_metrics_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.route_metrics
    ADD CONSTRAINT route_metrics_pkey PRIMARY KEY (id);


--
-- Name: rpc_cursor rpc_cursor_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.rpc_cursor
    ADD CONSTRAINT rpc_cursor_pkey PRIMARY KEY (relayed_by);


--
-- Name: rpc_error rpc_error_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.rpc_error
    ADD CONSTRAINT rpc_error_pkey PRIMARY KEY (sig);


--
-- Name: rpc_log rpc_log_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.rpc_log
    ADD CONSTRAINT rpc_log_pkey PRIMARY KEY (sig);


--
-- Name: saves saves_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.saves
    ADD CONSTRAINT saves_pkey PRIMARY KEY (user_id, save_item_id, save_type, txhash);


--
-- Name: schema_migrations schema_migrations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.schema_migrations
    ADD CONSTRAINT schema_migrations_pkey PRIMARY KEY (version);


--
-- Name: schema_version schema_version_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.schema_version
    ADD CONSTRAINT schema_version_pkey PRIMARY KEY (file_name);


--
-- Name: shares shares_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.shares
    ADD CONSTRAINT shares_pkey PRIMARY KEY (user_id, share_item_id, share_type, txhash);


--
-- Name: skipped_transactions skipped_transactions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.skipped_transactions
    ADD CONSTRAINT skipped_transactions_pkey PRIMARY KEY (id);


--
-- Name: sla_auditor_version_data sla_auditor_version_data_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sla_auditor_version_data
    ADD CONSTRAINT sla_auditor_version_data_pkey PRIMARY KEY (id);


--
-- Name: sla_node_reports sla_node_reports_address_sla_rollup_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sla_node_reports
    ADD CONSTRAINT sla_node_reports_address_sla_rollup_id_key UNIQUE (address, sla_rollup_id);


--
-- Name: sla_node_reports sla_node_reports_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sla_node_reports
    ADD CONSTRAINT sla_node_reports_pkey PRIMARY KEY (id);


--
-- Name: sla_rollups sla_rollups_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sla_rollups
    ADD CONSTRAINT sla_rollups_pkey PRIMARY KEY (id);


--
-- Name: sol_claimable_account_transfers sol_claimable_account_transfers_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sol_claimable_account_transfers
    ADD CONSTRAINT sol_claimable_account_transfers_pkey PRIMARY KEY (signature, instruction_index);


--
-- Name: sol_claimable_accounts sol_claimable_accounts_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sol_claimable_accounts
    ADD CONSTRAINT sol_claimable_accounts_pkey PRIMARY KEY (signature, instruction_index);


--
-- Name: sol_payments sol_payments_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sol_payments
    ADD CONSTRAINT sol_payments_pkey PRIMARY KEY (signature, instruction_index, route_index);


--
-- Name: sol_purchases sol_purchases_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sol_purchases
    ADD CONSTRAINT sol_purchases_pkey PRIMARY KEY (signature, instruction_index);


--
-- Name: sol_reward_disbursements sol_reward_disbursements_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sol_reward_disbursements
    ADD CONSTRAINT sol_reward_disbursements_pkey PRIMARY KEY (signature, instruction_index);


--
-- Name: sol_slot_checkpoints sol_slot_checkpoints_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sol_slot_checkpoints
    ADD CONSTRAINT sol_slot_checkpoints_pkey PRIMARY KEY (id);


--
-- Name: sol_swaps sol_swaps_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sol_swaps
    ADD CONSTRAINT sol_swaps_pkey PRIMARY KEY (signature, instruction_index);


--
-- Name: sol_token_account_balance_changes sol_token_account_balance_changes_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sol_token_account_balance_changes
    ADD CONSTRAINT sol_token_account_balance_changes_pkey PRIMARY KEY (signature, mint, account);


--
-- Name: sol_token_account_balances sol_token_account_balances_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sol_token_account_balances
    ADD CONSTRAINT sol_token_account_balances_pkey PRIMARY KEY (account);


--
-- Name: sol_token_transfers sol_token_transfers_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sol_token_transfers
    ADD CONSTRAINT sol_token_transfers_pkey PRIMARY KEY (signature, instruction_index);


--
-- Name: sol_user_balances sol_user_balances_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sol_user_balances
    ADD CONSTRAINT sol_user_balances_pkey PRIMARY KEY (user_id, mint);


--
-- Name: sound_recordings sound_recordings_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sound_recordings
    ADD CONSTRAINT sound_recordings_pkey PRIMARY KEY (id);


--
-- Name: spl_token_tx spl_token_tx_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.spl_token_tx
    ADD CONSTRAINT spl_token_tx_pkey PRIMARY KEY (last_scanned_slot);


--
-- Name: stems stems_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.stems
    ADD CONSTRAINT stems_pkey PRIMARY KEY (parent_track_id, child_track_id);


--
-- Name: storage_proof_peers storage_proof_peers_block_height_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.storage_proof_peers
    ADD CONSTRAINT storage_proof_peers_block_height_key UNIQUE (block_height);


--
-- Name: storage_proof_peers storage_proof_peers_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.storage_proof_peers
    ADD CONSTRAINT storage_proof_peers_pkey PRIMARY KEY (id);


--
-- Name: storage_proofs storage_proofs_address_block_height_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.storage_proofs
    ADD CONSTRAINT storage_proofs_address_block_height_key UNIQUE (address, block_height);


--
-- Name: storage_proofs storage_proofs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.storage_proofs
    ADD CONSTRAINT storage_proofs_pkey PRIMARY KEY (id);


--
-- Name: subscriptions subscriptions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.subscriptions
    ADD CONSTRAINT subscriptions_pkey PRIMARY KEY (subscriber_id, user_id, txhash);


--
-- Name: supporter_rank_ups supporter_rank_ups_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.supporter_rank_ups
    ADD CONSTRAINT supporter_rank_ups_pkey PRIMARY KEY (slot, sender_user_id, receiver_user_id);


--
-- Name: track_delist_statuses track_delist_statuses_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.track_delist_statuses
    ADD CONSTRAINT track_delist_statuses_pkey PRIMARY KEY (created_at, track_id, delisted);


--
-- Name: track_downloads track_downloads_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.track_downloads
    ADD CONSTRAINT track_downloads_pkey PRIMARY KEY (parent_track_id, track_id, txhash);


--
-- Name: track_price_history track_price_history_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.track_price_history
    ADD CONSTRAINT track_price_history_pkey PRIMARY KEY (track_id, block_timestamp);


--
-- Name: track_releases track_releases_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.track_releases
    ADD CONSTRAINT track_releases_pkey PRIMARY KEY (id);


--
-- Name: track_releases track_releases_track_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.track_releases
    ADD CONSTRAINT track_releases_track_id_key UNIQUE (track_id);


--
-- Name: track_routes track_routes_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.track_routes
    ADD CONSTRAINT track_routes_pkey PRIMARY KEY (owner_id, slug);


--
-- Name: track_trending_scores track_trending_scores_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.track_trending_scores
    ADD CONSTRAINT track_trending_scores_pkey PRIMARY KEY (track_id, type, version, time_range);


--
-- Name: tracks tracks_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tracks
    ADD CONSTRAINT tracks_pkey PRIMARY KEY (track_id, txhash);


--
-- Name: trending_results trending_results_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.trending_results
    ADD CONSTRAINT trending_results_pkey PRIMARY KEY (rank, type, version, week);


--
-- Name: developer_apps unique_developer_apps_address; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.developer_apps
    ADD CONSTRAINT unique_developer_apps_address UNIQUE (address);


--
-- Name: associated_wallets unique_user_wallet_chain; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.associated_wallets
    ADD CONSTRAINT unique_user_wallet_chain UNIQUE (user_id, wallet, chain);


--
-- Name: notification uq_notification; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.notification
    ADD CONSTRAINT uq_notification UNIQUE (group_id, specifier);


--
-- Name: usdc_purchases usdc_purchases_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.usdc_purchases
    ADD CONSTRAINT usdc_purchases_pkey PRIMARY KEY (slot, signature);


--
-- Name: usdc_transactions_history usdc_transactions_history_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.usdc_transactions_history
    ADD CONSTRAINT usdc_transactions_history_pkey PRIMARY KEY (user_bank, signature);


--
-- Name: usdc_user_bank_accounts usdc_user_bank_accounts_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.usdc_user_bank_accounts
    ADD CONSTRAINT usdc_user_bank_accounts_pkey PRIMARY KEY (signature);


--
-- Name: user_balance_changes user_balance_changes_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_balance_changes
    ADD CONSTRAINT user_balance_changes_pkey PRIMARY KEY (user_id);


--
-- Name: user_balances user_balances_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_balances
    ADD CONSTRAINT user_balances_pkey PRIMARY KEY (user_id);


--
-- Name: user_bank_accounts user_bank_accounts_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_bank_accounts
    ADD CONSTRAINT user_bank_accounts_pkey PRIMARY KEY (signature);


--
-- Name: user_bank_txs user_bank_txs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_bank_txs
    ADD CONSTRAINT user_bank_txs_pkey PRIMARY KEY (signature);


--
-- Name: user_challenges user_challenges_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_challenges
    ADD CONSTRAINT user_challenges_pkey PRIMARY KEY (challenge_id, specifier);


--
-- Name: user_delist_statuses user_delist_statuses_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_delist_statuses
    ADD CONSTRAINT user_delist_statuses_pkey PRIMARY KEY (created_at, user_id, delisted);


--
-- Name: user_events user_events_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_events
    ADD CONSTRAINT user_events_pkey PRIMARY KEY (id);


--
-- Name: user_listening_history user_listening_history_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_listening_history
    ADD CONSTRAINT user_listening_history_pkey PRIMARY KEY (user_id);


--
-- Name: user_payout_wallet_history user_payout_wallet_history_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_payout_wallet_history
    ADD CONSTRAINT user_payout_wallet_history_pkey PRIMARY KEY (user_id, block_timestamp);


--
-- Name: user_pubkeys user_pubkeys_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_pubkeys
    ADD CONSTRAINT user_pubkeys_pkey PRIMARY KEY (user_id);


--
-- Name: user_tips user_tips_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_tips
    ADD CONSTRAINT user_tips_pkey PRIMARY KEY (slot, signature);


--
-- Name: users users_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_pkey PRIMARY KEY (user_id, txhash);


--
-- Name: artist_coins_ticker_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX artist_coins_ticker_idx ON public.artist_coins USING btree (ticker, user_id);


--
-- Name: INDEX artist_coins_ticker_idx; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON INDEX public.artist_coins_ticker_idx IS 'Used for getting mint address by ticker.';


--
-- Name: artist_coins_user_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX artist_coins_user_id_idx ON public.artist_coins USING btree (user_id);


--
-- Name: INDEX artist_coins_user_id_idx; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON INDEX public.artist_coins_user_id_idx IS 'Used for getting coins minted by a particular artist.';


--
-- Name: blocks_is_current_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX blocks_is_current_idx ON public.blocks USING btree (is_current) WHERE (is_current IS TRUE);


--
-- Name: challenge_disbursements_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX challenge_disbursements_user_id ON public.challenge_disbursements USING btree (user_id);


--
-- Name: chat_chat_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX chat_chat_id_idx ON public.chat USING btree (chat_id);


--
-- Name: chat_member_user_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX chat_member_user_idx ON public.chat_member USING btree (user_id);


--
-- Name: eth_registered_endpoints_wallet_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX eth_registered_endpoints_wallet_idx ON public.eth_registered_endpoints USING btree (delegate_wallet);


--
-- Name: fix_tracks_top_genre_users_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX fix_tracks_top_genre_users_idx ON public.tracks USING btree (track_id, owner_id, genre, is_unlisted, is_delete) WHERE (stem_of IS NULL);


--
-- Name: follows_blocknumber_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX follows_blocknumber_idx ON public.follows USING btree (blocknumber);


--
-- Name: follows_inbound_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX follows_inbound_idx ON public.follows USING btree (followee_user_id, follower_user_id, is_delete);


--
-- Name: idx_access_keys_track_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_access_keys_track_id ON public.access_keys USING btree (track_id);


--
-- Name: idx_aggregate_user_follower_count; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aggregate_user_follower_count ON public.aggregate_user USING btree (user_id, follower_count);


--
-- Name: idx_api_metrics_apps_api_key; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_api_metrics_apps_api_key ON public.api_metrics_apps USING btree (api_key) WHERE (api_key IS NOT NULL);


--
-- Name: idx_api_metrics_apps_app_name; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_api_metrics_apps_app_name ON public.api_metrics_apps USING btree (app_name) WHERE (app_name IS NOT NULL);


--
-- Name: idx_api_metrics_apps_date; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_api_metrics_apps_date ON public.api_metrics_apps USING btree (date);


--
-- Name: idx_api_metrics_counts_date; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_api_metrics_counts_date ON public.api_metrics_counts USING btree (date);


--
-- Name: idx_api_metrics_routes_date; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_api_metrics_routes_date ON public.api_metrics_routes USING btree (date);


--
-- Name: idx_api_metrics_routes_method; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_api_metrics_routes_method ON public.api_metrics_routes USING btree (method);


--
-- Name: idx_api_metrics_routes_route_pattern; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_api_metrics_routes_route_pattern ON public.api_metrics_routes USING btree (route_pattern);


--
-- Name: idx_chain_blockhash; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_chain_blockhash ON public.core_indexed_blocks USING btree (blockhash);


--
-- Name: idx_chain_id_height; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_chain_id_height ON public.core_indexed_blocks USING btree (chain_id, height);


--
-- Name: idx_challenge_disbursements_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_challenge_disbursements_created_at ON public.challenge_disbursements USING btree (created_at);


--
-- Name: idx_challenge_disbursements_slot; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_challenge_disbursements_slot ON public.challenge_disbursements USING btree (slot);


--
-- Name: idx_chat_message_chat_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_chat_message_chat_id ON public.chat_message USING btree (chat_id);


--
-- Name: idx_chat_message_reactions_message_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_chat_message_reactions_message_id ON public.chat_message_reactions USING btree (message_id);


--
-- Name: idx_chat_message_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_chat_message_user_id ON public.chat_message USING btree (user_id, created_at);


--
-- Name: idx_core_blocks_chain_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_core_blocks_chain_id ON public.core_blocks USING btree (chain_id);


--
-- Name: idx_core_blocks_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_core_blocks_created_at ON public.core_blocks USING btree (created_at);


--
-- Name: idx_core_blocks_hash; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_core_blocks_hash ON public.core_blocks USING btree (hash);


--
-- Name: idx_core_blocks_height; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_core_blocks_height ON public.core_blocks USING btree (height);


--
-- Name: idx_core_blocks_proposer; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_core_blocks_proposer ON public.core_blocks USING btree (proposer);


--
-- Name: idx_core_stats_tx_type; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_core_stats_tx_type ON public.core_tx_stats USING btree (tx_type);


--
-- Name: idx_core_transactions_block_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_core_transactions_block_id ON public.core_transactions USING btree (block_id);


--
-- Name: idx_core_transactions_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_core_transactions_created_at ON public.core_transactions USING btree (created_at);


--
-- Name: idx_core_transactions_tx_hash; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_core_transactions_tx_hash ON public.core_transactions USING btree (tx_hash);


--
-- Name: idx_core_transactions_tx_hash_lower; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_core_transactions_tx_hash_lower ON public.core_transactions USING btree (lower(tx_hash));


--
-- Name: idx_core_tx_hash; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_core_tx_hash ON public.core_tx_stats USING btree (tx_hash);


--
-- Name: idx_core_tx_stats_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_core_tx_stats_created_at ON public.core_tx_stats USING btree (created_at);


--
-- Name: idx_core_tx_stats_time_type; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_core_tx_stats_time_type ON public.core_tx_stats USING btree (created_at, tx_type);


--
-- Name: idx_core_validators_comet_address; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_core_validators_comet_address ON public.core_validators USING btree (comet_address);


--
-- Name: idx_core_validators_endpoint; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_core_validators_endpoint ON public.core_validators USING btree (endpoint);


--
-- Name: idx_core_validators_eth_address; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_core_validators_eth_address ON public.core_validators USING btree (eth_address);


--
-- Name: idx_ddex_release_ids; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_ddex_release_ids ON public.tracks USING gin (ddex_release_ids);


--
-- Name: idx_email_access_grantor; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_email_access_grantor ON public.email_access USING btree (grantor_user_id);


--
-- Name: idx_email_access_receiver; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_email_access_receiver ON public.email_access USING btree (receiving_user_id);


--
-- Name: idx_encrypted_emails_email_address_owner_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_encrypted_emails_email_address_owner_user_id ON public.encrypted_emails USING btree (email_owner_user_id);


--
-- Name: idx_events_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_events_created_at ON public.events USING btree (created_at);


--
-- Name: idx_events_end_date; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_events_end_date ON public.events USING btree (end_date);


--
-- Name: idx_events_entity_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_events_entity_id ON public.events USING btree (entity_id);


--
-- Name: idx_events_entity_type; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_events_entity_type ON public.events USING btree (entity_type);


--
-- Name: idx_events_event_type; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_events_event_type ON public.events USING btree (event_type);


--
-- Name: idx_fanout; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_fanout ON public.follows USING btree (follower_user_id, followee_user_id);


--
-- Name: idx_fanout_not_deleted; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_fanout_not_deleted ON public.follows USING btree (follower_user_id, followee_user_id) WHERE (is_delete = false);


--
-- Name: idx_genre_related_artists; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_genre_related_artists ON public.aggregate_user USING btree (dominant_genre, follower_count, user_id);


--
-- Name: idx_lower_wallet; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_lower_wallet ON public.users USING btree (lower((wallet)::text));


--
-- Name: idx_management_keys_track_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_management_keys_track_id ON public.management_keys USING btree (track_id);


--
-- Name: idx_payment_router_txs_slot; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_payment_router_txs_slot ON public.payment_router_txs USING btree (slot);


--
-- Name: idx_playlist_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_playlist_status ON public.playlists USING btree (playlist_id, is_album, is_private, is_delete, is_current);


--
-- Name: idx_playlist_tracks_track_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_playlist_tracks_track_id ON public.playlist_tracks USING btree (track_id, created_at);


--
-- Name: idx_playlist_trending_scores_filtered; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_playlist_trending_scores_filtered ON public.playlist_trending_scores USING btree (type, version, time_range, playlist_id, score DESC);


--
-- Name: idx_reward_manager_txs_slot; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_reward_manager_txs_slot ON public.reward_manager_txs USING btree (slot);


--
-- Name: idx_rpc_relayed_by; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_rpc_relayed_by ON public.rpc_log USING btree (relayed_by, relayed_at);


--
-- Name: idx_sound_recordings_track_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_sound_recordings_track_id ON public.sound_recordings USING btree (track_id);


--
-- Name: idx_storage_proof_peers_block_height; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_storage_proof_peers_block_height ON public.storage_proof_peers USING btree (block_height DESC);


--
-- Name: idx_storage_proofs_block_height; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_storage_proofs_block_height ON public.storage_proofs USING btree (block_height DESC);


--
-- Name: idx_time; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_time ON public.sla_rollups USING btree ("time" DESC);


--
-- Name: idx_track_releases_track_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_track_releases_track_id ON public.track_releases USING btree (track_id);


--
-- Name: idx_track_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_track_status ON public.tracks USING btree (track_id, is_unlisted, is_available, is_delete, is_current);


--
-- Name: idx_tracks_stream_conditions_gin; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_tracks_stream_conditions_gin ON public.tracks USING gin (stream_conditions);


--
-- Name: idx_tts_genre_time_score; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_tts_genre_time_score ON public.track_trending_scores USING btree (genre, time_range, score DESC, track_id);


--
-- Name: idx_usdc_purchases_buyer; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_usdc_purchases_buyer ON public.usdc_purchases USING btree (buyer_user_id);


--
-- Name: idx_usdc_purchases_seller; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_usdc_purchases_seller ON public.usdc_purchases USING btree (seller_user_id);


--
-- Name: idx_usdc_purchases_type; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_usdc_purchases_type ON public.usdc_purchases USING btree (content_type);


--
-- Name: idx_usdc_transactions_history_slot; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_usdc_transactions_history_slot ON public.usdc_transactions_history USING btree (slot);


--
-- Name: idx_usdc_transactions_history_type; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_usdc_transactions_history_type ON public.usdc_transactions_history USING btree (transaction_type);


--
-- Name: idx_usdc_user_bank_accounts_eth_address; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_usdc_user_bank_accounts_eth_address ON public.usdc_user_bank_accounts USING btree (ethereum_address);


--
-- Name: idx_user_bank_eth_address; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_user_bank_eth_address ON public.user_bank_accounts USING btree (ethereum_address);


--
-- Name: idx_user_bank_txs_slot; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_user_bank_txs_slot ON public.user_bank_txs USING btree (slot);


--
-- Name: idx_user_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_user_status ON public.users USING btree (user_id, is_deactivated, is_available, is_current);


--
-- Name: interval_play_month_count_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX interval_play_month_count_idx ON public.aggregate_interval_plays USING btree (month_listen_counts);


--
-- Name: interval_play_track_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX interval_play_track_id_idx ON public.aggregate_interval_plays USING btree (track_id);


--
-- Name: interval_play_week_count_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX interval_play_week_count_idx ON public.aggregate_interval_plays USING btree (week_listen_counts);


--
-- Name: is_current_blocks_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX is_current_blocks_idx ON public.blocks USING btree (is_current);


--
-- Name: ix_aggregate_user_tips_receiver_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_aggregate_user_tips_receiver_user_id ON public.aggregate_user_tips USING btree (receiver_user_id);


--
-- Name: ix_announcement; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_announcement ON public.notification USING btree (type, "timestamp") WHERE ((type)::text = 'announcement'::text);


--
-- Name: ix_associated_wallets_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_associated_wallets_user_id ON public.associated_wallets USING btree (user_id);


--
-- Name: ix_associated_wallets_wallet; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_associated_wallets_wallet ON public.associated_wallets USING btree (wallet);


--
-- Name: ix_audio_transactions_history_slot; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_audio_transactions_history_slot ON public.audio_transactions_history USING btree (slot);


--
-- Name: ix_audio_transactions_history_transaction_type; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_audio_transactions_history_transaction_type ON public.audio_transactions_history USING btree (transaction_type);


--
-- Name: ix_follows_followee_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_follows_followee_user_id ON public.follows USING btree (followee_user_id);


--
-- Name: ix_follows_follower_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_follows_follower_user_id ON public.follows USING btree (follower_user_id);


--
-- Name: ix_notification; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_notification ON public.notification USING gin (user_ids);


--
-- Name: ix_playlist_trending_scores_playlist_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_playlist_trending_scores_playlist_id ON public.playlist_trending_scores USING btree (playlist_id);


--
-- Name: ix_plays_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_plays_created_at ON public.plays USING btree (created_at);


--
-- Name: ix_plays_slot; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_plays_slot ON public.plays USING btree (slot);


--
-- Name: ix_plays_sol_signature; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_plays_sol_signature ON public.plays USING btree (signature);


--
-- Name: ix_plays_user_track_date; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_plays_user_track_date ON public.plays USING btree (user_id, play_item_id, created_at) WHERE (user_id IS NOT NULL);


--
-- Name: ix_reactions_reacted_to_reaction_type; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_reactions_reacted_to_reaction_type ON public.reactions USING btree (reacted_to, reaction_type);


--
-- Name: ix_subscriptions_blocknumber; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_subscriptions_blocknumber ON public.subscriptions USING btree (blocknumber);


--
-- Name: ix_subscriptions_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_subscriptions_user_id ON public.subscriptions USING btree (user_id);


--
-- Name: ix_supporter_rank_ups_receiver_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_supporter_rank_ups_receiver_user_id ON public.supporter_rank_ups USING btree (receiver_user_id);


--
-- Name: ix_supporter_rank_ups_sender_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_supporter_rank_ups_sender_user_id ON public.supporter_rank_ups USING btree (sender_user_id);


--
-- Name: ix_supporter_rank_ups_slot; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_supporter_rank_ups_slot ON public.supporter_rank_ups USING btree (slot);


--
-- Name: ix_track_trending_scores_genre; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_track_trending_scores_genre ON public.track_trending_scores USING btree (genre);


--
-- Name: ix_track_trending_scores_track_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_track_trending_scores_track_id ON public.track_trending_scores USING btree (track_id);


--
-- Name: ix_trending_scores; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_trending_scores ON public.track_trending_scores USING btree (type, version, time_range, score DESC, track_id);


--
-- Name: ix_user_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_user_created_at ON public.users USING btree (created_at, user_id, is_current);


--
-- Name: ix_user_tips_receiver_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_user_tips_receiver_user_id ON public.user_tips USING btree (receiver_user_id);


--
-- Name: ix_user_tips_sender_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_user_tips_sender_user_id ON public.user_tips USING btree (sender_user_id);


--
-- Name: ix_user_tips_slot; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_user_tips_slot ON public.user_tips USING btree (slot);


--
-- Name: ix_users_active_count; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_users_active_count ON public.users USING btree (is_deactivated, is_current, handle_lc, is_available) WHERE ((is_deactivated = false) AND (is_current = true) AND (handle_lc IS NOT NULL) AND (is_available = true));


--
-- Name: milestones_name_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX milestones_name_idx ON public.milestones USING btree (name, id);


--
-- Name: notification_seen_blocknumber_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX notification_seen_blocknumber_idx ON public.notification_seen USING btree (blocknumber);


--
-- Name: playlist_created_at_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX playlist_created_at_idx ON public.playlists USING btree (created_at);


--
-- Name: playlist_owner_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX playlist_owner_idx ON public.playlists USING btree (playlist_owner_id, created_at);


--
-- Name: playlist_routes_playlist_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX playlist_routes_playlist_id_idx ON public.playlist_routes USING btree (playlist_id);


--
-- Name: playlists_blocknumber_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX playlists_blocknumber_idx ON public.playlists USING btree (blocknumber);


--
-- Name: related_artists_related_artist_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX related_artists_related_artist_id_idx ON public.related_artists USING btree (related_artist_user_id, user_id);


--
-- Name: remixes_child_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX remixes_child_idx ON public.remixes USING btree (child_track_id, parent_track_id);


--
-- Name: reposts_item_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX reposts_item_idx ON public.reposts USING btree (repost_item_id, repost_type, user_id, is_delete);


--
-- Name: reposts_new_blocknumber_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX reposts_new_blocknumber_idx ON public.reposts USING btree (blocknumber);


--
-- Name: reposts_new_created_at_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX reposts_new_created_at_idx ON public.reposts USING btree (created_at);


--
-- Name: reposts_user_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX reposts_user_idx ON public.reposts USING btree (user_id, repost_type, repost_item_id, created_at, is_delete);


--
-- Name: rpc_log_applied_at_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX rpc_log_applied_at_idx ON public.rpc_log USING brin (applied_at);


--
-- Name: saves_item_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX saves_item_idx ON public.saves USING btree (save_item_id, save_type, user_id, is_delete);


--
-- Name: saves_new_blocknumber_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX saves_new_blocknumber_idx ON public.saves USING btree (blocknumber);


--
-- Name: saves_user_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX saves_user_idx ON public.saves USING btree (user_id, save_type, save_item_id, is_delete);


--
-- Name: shares_item_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX shares_item_idx ON public.shares USING btree (share_item_id, share_type, user_id);


--
-- Name: shares_new_blocknumber_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX shares_new_blocknumber_idx ON public.shares USING btree (blocknumber);


--
-- Name: shares_new_created_at_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX shares_new_created_at_idx ON public.shares USING btree (created_at);


--
-- Name: shares_slot_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX shares_slot_idx ON public.shares USING btree (slot);


--
-- Name: shares_user_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX shares_user_idx ON public.shares USING btree (user_id, share_type, share_item_id, created_at);


--
-- Name: sla_auditor_version_data_nodeendpoint_index; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX sla_auditor_version_data_nodeendpoint_index ON public.sla_auditor_version_data USING btree ("nodeEndpoint");


--
-- Name: sol_claimable_account_transfers_from_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX sol_claimable_account_transfers_from_idx ON public.sol_claimable_account_transfers USING btree (from_account);


--
-- Name: INDEX sol_claimable_account_transfers_from_idx; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON INDEX public.sol_claimable_account_transfers_from_idx IS 'Used for getting transfers by recipient.';


--
-- Name: sol_claimable_account_transfers_sender_eth_address_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX sol_claimable_account_transfers_sender_eth_address_idx ON public.sol_claimable_account_transfers USING btree (sender_eth_address);


--
-- Name: INDEX sol_claimable_account_transfers_sender_eth_address_idx; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON INDEX public.sol_claimable_account_transfers_sender_eth_address_idx IS 'Used for getting transfers by sender user wallet.';


--
-- Name: sol_claimable_account_transfers_to_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX sol_claimable_account_transfers_to_idx ON public.sol_claimable_account_transfers USING btree (to_account);


--
-- Name: INDEX sol_claimable_account_transfers_to_idx; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON INDEX public.sol_claimable_account_transfers_to_idx IS 'Used for getting transfers by sender.';


--
-- Name: sol_claimable_accounts_account_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX sol_claimable_accounts_account_idx ON public.sol_claimable_accounts USING btree (account);


--
-- Name: INDEX sol_claimable_accounts_account_idx; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON INDEX public.sol_claimable_accounts_account_idx IS 'Used for getting user wallet by account.';


--
-- Name: sol_claimable_accounts_ethereum_address_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX sol_claimable_accounts_ethereum_address_idx ON public.sol_claimable_accounts USING btree (ethereum_address, mint);


--
-- Name: INDEX sol_claimable_accounts_ethereum_address_idx; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON INDEX public.sol_claimable_accounts_ethereum_address_idx IS 'Used for getting account by user wallet and mint.';


--
-- Name: sol_payments_to_account; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX sol_payments_to_account ON public.sol_payments USING btree (to_account);


--
-- Name: INDEX sol_payments_to_account; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON INDEX public.sol_payments_to_account IS 'Used for getting payments to a particular user.';


--
-- Name: sol_purchases_buyer_user_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX sol_purchases_buyer_user_id_idx ON public.sol_purchases USING btree (buyer_user_id, is_valid);


--
-- Name: INDEX sol_purchases_buyer_user_id_idx; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON INDEX public.sol_purchases_buyer_user_id_idx IS 'Used for getting purchases by a user.';


--
-- Name: sol_purchases_content_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX sol_purchases_content_idx ON public.sol_purchases USING btree (content_id, content_type, access_type, is_valid);


--
-- Name: INDEX sol_purchases_content_idx; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON INDEX public.sol_purchases_content_idx IS 'Used for getting sales of particular content.';


--
-- Name: sol_purchases_from_account_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX sol_purchases_from_account_idx ON public.sol_purchases USING btree (from_account, is_valid);


--
-- Name: INDEX sol_purchases_from_account_idx; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON INDEX public.sol_purchases_from_account_idx IS 'Used for getting purchases by a user via their account.';


--
-- Name: sol_purchases_valid_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX sol_purchases_valid_idx ON public.sol_purchases USING btree (is_valid, valid_after_blocknumber);


--
-- Name: INDEX sol_purchases_valid_idx; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON INDEX public.sol_purchases_valid_idx IS 'Used for updating purchases to be valid after the specified blocknumber is reached.';


--
-- Name: sol_reward_disbursements_challenge_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX sol_reward_disbursements_challenge_idx ON public.sol_reward_disbursements USING btree (challenge_id, specifier);


--
-- Name: INDEX sol_reward_disbursements_challenge_idx; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON INDEX public.sol_reward_disbursements_challenge_idx IS 'Used for getting reward disbursements for a specific challenge type or claim.';


--
-- Name: sol_reward_disbursements_user_bank_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX sol_reward_disbursements_user_bank_idx ON public.sol_reward_disbursements USING btree (user_bank);


--
-- Name: INDEX sol_reward_disbursements_user_bank_idx; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON INDEX public.sol_reward_disbursements_user_bank_idx IS 'Used for getting reward disbursements for a user.';


--
-- Name: sol_slot_checkpoints_from_slot_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX sol_slot_checkpoints_from_slot_idx ON public.sol_slot_checkpoints USING btree (subscription_hash, from_slot);


--
-- Name: sol_slot_checkpoints_to_slot_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX sol_slot_checkpoints_to_slot_idx ON public.sol_slot_checkpoints USING btree (subscription_hash, to_slot);


--
-- Name: sol_swaps_from_account_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX sol_swaps_from_account_idx ON public.sol_swaps USING btree (from_account);


--
-- Name: sol_swaps_from_mint_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX sol_swaps_from_mint_idx ON public.sol_swaps USING btree (from_mint);


--
-- Name: sol_swaps_to_account_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX sol_swaps_to_account_idx ON public.sol_swaps USING btree (to_account);


--
-- Name: sol_swaps_to_mint_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX sol_swaps_to_mint_idx ON public.sol_swaps USING btree (to_mint);


--
-- Name: sol_token_account_balance_changes_account_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX sol_token_account_balance_changes_account_idx ON public.sol_token_account_balance_changes USING btree (account, slot);


--
-- Name: INDEX sol_token_account_balance_changes_account_idx; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON INDEX public.sol_token_account_balance_changes_account_idx IS 'Used for getting recent transactions by account.';


--
-- Name: sol_token_account_balance_changes_mint_block_timestamp; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX sol_token_account_balance_changes_mint_block_timestamp ON public.sol_token_account_balance_changes USING btree (mint, block_timestamp DESC);


--
-- Name: sol_token_account_balance_changes_mint_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX sol_token_account_balance_changes_mint_idx ON public.sol_token_account_balance_changes USING btree (mint, slot);


--
-- Name: INDEX sol_token_account_balance_changes_mint_idx; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON INDEX public.sol_token_account_balance_changes_mint_idx IS 'Used for getting recent transactions by mint.';


--
-- Name: sol_token_account_balance_changes_owner_slot_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX sol_token_account_balance_changes_owner_slot_idx ON public.sol_token_account_balance_changes USING btree (owner, slot DESC);


--
-- Name: INDEX sol_token_account_balance_changes_owner_slot_idx; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON INDEX public.sol_token_account_balance_changes_owner_slot_idx IS 'Used for associating connected wallets with the transaction.';


--
-- Name: sol_token_account_balances_mint_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX sol_token_account_balances_mint_idx ON public.sol_token_account_balances USING btree (mint);


--
-- Name: INDEX sol_token_account_balances_mint_idx; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON INDEX public.sol_token_account_balances_mint_idx IS 'Used for getting current balances by mint.';


--
-- Name: sol_token_account_balances_owner_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX sol_token_account_balances_owner_idx ON public.sol_token_account_balances USING btree (owner);


--
-- Name: sol_token_transfers_from_account_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX sol_token_transfers_from_account_idx ON public.sol_token_transfers USING btree (from_account);


--
-- Name: sol_token_transfers_to_account_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX sol_token_transfers_to_account_idx ON public.sol_token_transfers USING btree (to_account);


--
-- Name: sol_user_balances_mint_user_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX sol_user_balances_mint_user_id_idx ON public.sol_user_balances USING btree (mint, user_id);


--
-- Name: INDEX sol_user_balances_mint_user_id_idx; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON INDEX public.sol_user_balances_mint_user_id_idx IS 'Index for quick access to user balances by mint and user ID.';


--
-- Name: tag_track_user_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX tag_track_user_idx ON public.tag_track_user USING btree (tag, track_id, owner_id);


--
-- Name: tag_track_user_tag_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX tag_track_user_tag_idx ON public.tag_track_user USING btree (tag);


--
-- Name: track_created_at_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX track_created_at_idx ON public.tracks USING btree (created_at);


--
-- Name: track_delist_statuses_owner_id_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX track_delist_statuses_owner_id_created_at ON public.track_delist_statuses USING btree (owner_id, created_at);


--
-- Name: track_delist_statuses_track_cid_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX track_delist_statuses_track_cid_created_at ON public.track_delist_statuses USING btree (track_cid, created_at);


--
-- Name: track_owner_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX track_owner_id_idx ON public.tracks USING btree (owner_id);


--
-- Name: track_owner_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX track_owner_idx ON public.tracks USING btree (owner_id, created_at);


--
-- Name: track_routes_track_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX track_routes_track_id_idx ON public.track_routes USING btree (track_id);


--
-- Name: tracks_ai_attribution_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX tracks_ai_attribution_user_id ON public.tracks USING btree (ai_attribution_user_id) WHERE (ai_attribution_user_id IS NOT NULL);


--
-- Name: tracks_blocknumber_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX tracks_blocknumber_idx ON public.tracks USING btree (blocknumber);


--
-- Name: tracks_track_cid_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX tracks_track_cid_idx ON public.tracks USING btree (track_cid, is_delete);


--
-- Name: trending_params_track_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX trending_params_track_id_idx ON public.trending_params USING btree (track_id);


--
-- Name: user_challenges_challenge_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX user_challenges_challenge_idx ON public.user_challenges USING btree (challenge_id);


--
-- Name: user_challenges_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX user_challenges_user_id ON public.user_challenges USING btree (user_id);


--
-- Name: user_events_user_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX user_events_user_id_idx ON public.user_events USING btree (user_id);


--
-- Name: users_new_blocknumber_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX users_new_blocknumber_idx ON public.users USING btree (blocknumber);


--
-- Name: users_new_handle_lc_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX users_new_handle_lc_idx ON public.users USING btree (handle_lc);


--
-- Name: users_new_wallet_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX users_new_wallet_idx ON public.users USING btree (wallet);


--
-- Name: artist_coins on_artist_coins_change; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER on_artist_coins_change AFTER INSERT OR DELETE OR UPDATE ON public.artist_coins FOR EACH ROW EXECUTE FUNCTION public.handle_artist_coins_change();


--
-- Name: TRIGGER on_artist_coins_change ON artist_coins; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TRIGGER on_artist_coins_change ON public.artist_coins IS 'Notifies when artist coins are added, removed, or updated.';


--
-- Name: associated_wallets on_associated_wallets; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER on_associated_wallets AFTER INSERT OR DELETE OR UPDATE ON public.associated_wallets FOR EACH ROW EXECUTE FUNCTION public.handle_associated_wallets();


--
-- Name: TRIGGER on_associated_wallets ON associated_wallets; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TRIGGER on_associated_wallets ON public.associated_wallets IS 'Updates sol_user_balances when associated_wallets are added and removed';


--
-- Name: challenge_disbursements on_challenge_disbursement; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER on_challenge_disbursement AFTER INSERT ON public.challenge_disbursements FOR EACH ROW EXECUTE FUNCTION public.handle_challenge_disbursement();


--
-- Name: comments on_comment; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER on_comment AFTER INSERT ON public.comments FOR EACH ROW EXECUTE FUNCTION public.handle_comment();


--
-- Name: events on_event; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER on_event AFTER INSERT ON public.events FOR EACH ROW EXECUTE FUNCTION public.handle_event();


--
-- Name: follows on_follow; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER on_follow AFTER INSERT ON public.follows FOR EACH ROW EXECUTE FUNCTION public.handle_follow();


--
-- Name: plays on_play; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER on_play AFTER INSERT ON public.plays FOR EACH ROW EXECUTE FUNCTION public.handle_play();


--
-- Name: playlists on_playlist; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER on_playlist AFTER INSERT ON public.playlists FOR EACH ROW EXECUTE FUNCTION public.handle_playlist();


--
-- Name: playlist_tracks on_playlist_track; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER on_playlist_track AFTER INSERT ON public.playlist_tracks FOR EACH ROW EXECUTE FUNCTION public.handle_playlist_track();


--
-- Name: reactions on_reaction; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER on_reaction AFTER INSERT ON public.reactions FOR EACH ROW EXECUTE FUNCTION public.handle_reaction();


--
-- Name: reposts on_repost; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER on_repost AFTER INSERT ON public.reposts FOR EACH ROW EXECUTE FUNCTION public.handle_repost();


--
-- Name: saves on_save; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER on_save AFTER INSERT ON public.saves FOR EACH ROW EXECUTE FUNCTION public.handle_save();


--
-- Name: shares on_share; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER on_share AFTER INSERT ON public.shares FOR EACH ROW EXECUTE FUNCTION public.handle_share();


--
-- Name: sol_claimable_accounts on_sol_claimable_accounts; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER on_sol_claimable_accounts AFTER INSERT ON public.sol_claimable_accounts FOR EACH ROW EXECUTE FUNCTION public.handle_sol_claimable_accounts();


--
-- Name: TRIGGER on_sol_claimable_accounts ON sol_claimable_accounts; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TRIGGER on_sol_claimable_accounts ON public.sol_claimable_accounts IS 'Updates sol_user_balances whenever a sol_claimable_account is inserted.';


--
-- Name: sol_token_account_balance_changes on_sol_token_account_balance_changes; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER on_sol_token_account_balance_changes AFTER INSERT ON public.sol_token_account_balance_changes FOR EACH ROW EXECUTE FUNCTION public.handle_sol_token_balance_change();


--
-- Name: TRIGGER on_sol_token_account_balance_changes ON sol_token_account_balance_changes; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TRIGGER on_sol_token_account_balance_changes ON public.sol_token_account_balance_changes IS 'Updates sol_token_account_balances whenever a sol_token_balance_change is inserted with a higher slot.';


--
-- Name: supporter_rank_ups on_supporter_rank_up; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER on_supporter_rank_up AFTER INSERT ON public.supporter_rank_ups FOR EACH ROW EXECUTE FUNCTION public.handle_supporter_rank_up();


--
-- Name: tracks on_track; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER on_track AFTER INSERT OR UPDATE ON public.tracks FOR EACH ROW EXECUTE FUNCTION public.handle_track();


--
-- Name: usdc_purchases on_usdc_purchase; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER on_usdc_purchase AFTER INSERT ON public.usdc_purchases FOR EACH ROW EXECUTE FUNCTION public.handle_usdc_purchase();


--
-- Name: usdc_transactions_history on_usdc_withdrawal; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER on_usdc_withdrawal AFTER INSERT ON public.usdc_transactions_history FOR EACH ROW EXECUTE FUNCTION public.handle_usdc_withdrawal();


--
-- Name: users on_user; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER on_user AFTER INSERT ON public.users FOR EACH ROW EXECUTE FUNCTION public.handle_user();


--
-- Name: user_balance_changes on_user_balance_changes; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER on_user_balance_changes AFTER INSERT OR UPDATE ON public.user_balance_changes FOR EACH ROW EXECUTE FUNCTION public.handle_user_balance_change();


--
-- Name: user_challenges on_user_challenge; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER on_user_challenge AFTER INSERT OR UPDATE ON public.user_challenges FOR EACH ROW EXECUTE FUNCTION public.handle_on_user_challenge();


--
-- Name: user_tips on_user_tip; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER on_user_tip AFTER INSERT ON public.user_tips FOR EACH ROW EXECUTE FUNCTION public.handle_user_tip();


--
-- Name: aggregate_plays trg_aggregate_plays; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER trg_aggregate_plays AFTER INSERT OR UPDATE ON public.aggregate_plays FOR EACH ROW EXECUTE FUNCTION public.on_new_row();


--
-- Name: aggregate_user trg_aggregate_user; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER trg_aggregate_user AFTER INSERT OR UPDATE ON public.aggregate_user FOR EACH ROW EXECUTE FUNCTION public.on_new_row();


--
-- Name: follows trg_follows; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER trg_follows AFTER INSERT OR UPDATE ON public.follows FOR EACH ROW EXECUTE FUNCTION public.on_new_row();


--
-- Name: notification trg_notification; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER trg_notification AFTER INSERT ON public.notification FOR EACH ROW EXECUTE FUNCTION public.on_new_notification_row();


--
-- Name: notification_seen trg_notification_seen; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER trg_notification_seen AFTER INSERT ON public.notification_seen FOR EACH ROW EXECUTE FUNCTION public.on_new_notification_seen_row();


--
-- Name: playlists trg_playlists; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER trg_playlists AFTER INSERT OR UPDATE ON public.playlists FOR EACH ROW EXECUTE FUNCTION public.on_new_row();


--
-- Name: reposts trg_reposts; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER trg_reposts AFTER INSERT OR UPDATE ON public.reposts FOR EACH ROW EXECUTE FUNCTION public.on_new_row();


--
-- Name: saves trg_saves; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER trg_saves AFTER INSERT OR UPDATE ON public.saves FOR EACH ROW EXECUTE FUNCTION public.on_new_row();


--
-- Name: shares trg_shares; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER trg_shares AFTER INSERT OR UPDATE ON public.shares FOR EACH ROW EXECUTE FUNCTION public.on_new_row();


--
-- Name: tracks trg_tracks; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER trg_tracks AFTER INSERT OR UPDATE ON public.tracks FOR EACH ROW EXECUTE FUNCTION public.on_new_row();


--
-- Name: usdc_purchases trg_usdc_purchases; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER trg_usdc_purchases AFTER INSERT OR UPDATE ON public.usdc_purchases FOR EACH ROW EXECUTE FUNCTION public.on_new_row();


--
-- Name: users trg_users; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER trg_users AFTER INSERT OR UPDATE ON public.users FOR EACH ROW EXECUTE FUNCTION public.on_new_row();


--
-- Name: grants trigger_grant_change; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER trigger_grant_change AFTER INSERT OR UPDATE ON public.grants FOR EACH ROW EXECUTE FUNCTION public.process_grant_change();


--
-- Name: album_price_history album_price_history_blocknumber_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.album_price_history
    ADD CONSTRAINT album_price_history_blocknumber_fkey FOREIGN KEY (blocknumber) REFERENCES public.blocks(number) ON DELETE CASCADE;


--
-- Name: associated_wallets associated_wallets_blocknumber_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.associated_wallets
    ADD CONSTRAINT associated_wallets_blocknumber_fkey FOREIGN KEY (blocknumber) REFERENCES public.blocks(number) ON DELETE CASCADE;


--
-- Name: chat_member chat_member_chat_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.chat_member
    ADD CONSTRAINT chat_member_chat_id_fkey FOREIGN KEY (chat_id) REFERENCES public.chat(chat_id) ON DELETE CASCADE;


--
-- Name: chat_message chat_message_blast_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.chat_message
    ADD CONSTRAINT chat_message_blast_id_fkey FOREIGN KEY (blast_id) REFERENCES public.chat_blast(blast_id);


--
-- Name: chat_message chat_message_chat_member_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.chat_message
    ADD CONSTRAINT chat_message_chat_member_fkey FOREIGN KEY (chat_id, user_id) REFERENCES public.chat_member(chat_id, user_id) ON DELETE CASCADE;


--
-- Name: chat_message_reactions chat_message_reactions_message_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.chat_message_reactions
    ADD CONSTRAINT chat_message_reactions_message_id_fkey FOREIGN KEY (message_id) REFERENCES public.chat_message(message_id) ON DELETE CASCADE;


--
-- Name: collectibles collectibles_blocknumber_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.collectibles
    ADD CONSTRAINT collectibles_blocknumber_fkey FOREIGN KEY (blocknumber) REFERENCES public.blocks(number) ON DELETE CASCADE;


--
-- Name: comment_mentions comment_mentions_blocknumber_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.comment_mentions
    ADD CONSTRAINT comment_mentions_blocknumber_fkey FOREIGN KEY (blocknumber) REFERENCES public.blocks(number) ON DELETE CASCADE;


--
-- Name: comment_reactions comment_reactions_blocknumber_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.comment_reactions
    ADD CONSTRAINT comment_reactions_blocknumber_fkey FOREIGN KEY (blocknumber) REFERENCES public.blocks(number) ON DELETE CASCADE;


--
-- Name: comment_reports comment_reports_blocknumber_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.comment_reports
    ADD CONSTRAINT comment_reports_blocknumber_fkey FOREIGN KEY (blocknumber) REFERENCES public.blocks(number) ON DELETE CASCADE;


--
-- Name: comments comments_blocknumber_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.comments
    ADD CONSTRAINT comments_blocknumber_fkey FOREIGN KEY (blocknumber) REFERENCES public.blocks(number) ON DELETE CASCADE;


--
-- Name: dashboard_wallet_users dashboard_wallet_users_blocknumber_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dashboard_wallet_users
    ADD CONSTRAINT dashboard_wallet_users_blocknumber_fkey FOREIGN KEY (blocknumber) REFERENCES public.blocks(number) ON DELETE CASCADE;


--
-- Name: developer_apps developer_apps_blocknumber_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.developer_apps
    ADD CONSTRAINT developer_apps_blocknumber_fkey FOREIGN KEY (blocknumber) REFERENCES public.blocks(number) ON DELETE CASCADE;


--
-- Name: events events_blocknumber_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.events
    ADD CONSTRAINT events_blocknumber_fkey FOREIGN KEY (blocknumber) REFERENCES public.blocks(number);


--
-- Name: follows follows_blocknumber_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.follows
    ADD CONSTRAINT follows_blocknumber_fkey FOREIGN KEY (blocknumber) REFERENCES public.blocks(number) ON DELETE CASCADE;


--
-- Name: grants grants_blocknumber_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.grants
    ADD CONSTRAINT grants_blocknumber_fkey FOREIGN KEY (blocknumber) REFERENCES public.blocks(number) ON DELETE CASCADE;


--
-- Name: muted_users muted_users_blocknumber_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.muted_users
    ADD CONSTRAINT muted_users_blocknumber_fkey FOREIGN KEY (blocknumber) REFERENCES public.blocks(number) ON DELETE CASCADE;


--
-- Name: notification_seen notification_seen_blocknumber_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.notification_seen
    ADD CONSTRAINT notification_seen_blocknumber_fkey FOREIGN KEY (blocknumber) REFERENCES public.blocks(number) ON DELETE CASCADE;


--
-- Name: playlist_seen playlist_seen_blocknumber_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.playlist_seen
    ADD CONSTRAINT playlist_seen_blocknumber_fkey FOREIGN KEY (blocknumber) REFERENCES public.blocks(number) ON DELETE CASCADE;


--
-- Name: playlists playlists_blocknumber_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.playlists
    ADD CONSTRAINT playlists_blocknumber_fkey FOREIGN KEY (blocknumber) REFERENCES public.blocks(number) ON DELETE CASCADE;


--
-- Name: reactions reactions_blocknumber_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.reactions
    ADD CONSTRAINT reactions_blocknumber_fkey FOREIGN KEY (blocknumber) REFERENCES public.blocks(number) ON DELETE CASCADE;


--
-- Name: reported_comments reported_comments_blocknumber_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.reported_comments
    ADD CONSTRAINT reported_comments_blocknumber_fkey FOREIGN KEY (blocknumber) REFERENCES public.blocks(number) ON DELETE CASCADE;


--
-- Name: reposts reposts_blocknumber_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.reposts
    ADD CONSTRAINT reposts_blocknumber_fkey FOREIGN KEY (blocknumber) REFERENCES public.blocks(number) ON DELETE CASCADE;


--
-- Name: revert_blocks revert_blocks_blocknumber_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.revert_blocks
    ADD CONSTRAINT revert_blocks_blocknumber_fkey FOREIGN KEY (blocknumber) REFERENCES public.blocks(number) ON DELETE CASCADE;


--
-- Name: saves saves_blocknumber_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.saves
    ADD CONSTRAINT saves_blocknumber_fkey FOREIGN KEY (blocknumber) REFERENCES public.blocks(number) ON DELETE CASCADE;


--
-- Name: sla_node_reports sla_node_reports_sla_rollup_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sla_node_reports
    ADD CONSTRAINT sla_node_reports_sla_rollup_id_fkey FOREIGN KEY (sla_rollup_id) REFERENCES public.sla_rollups(id);


--
-- Name: subscriptions subscriptions_blocknumber_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.subscriptions
    ADD CONSTRAINT subscriptions_blocknumber_fkey FOREIGN KEY (blocknumber) REFERENCES public.blocks(number) ON DELETE CASCADE;


--
-- Name: track_downloads track_downloads_blocknumber_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.track_downloads
    ADD CONSTRAINT track_downloads_blocknumber_fkey FOREIGN KEY (blocknumber) REFERENCES public.blocks(number) ON DELETE CASCADE;


--
-- Name: track_price_history track_price_history_blocknumber_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.track_price_history
    ADD CONSTRAINT track_price_history_blocknumber_fkey FOREIGN KEY (blocknumber) REFERENCES public.blocks(number) ON DELETE CASCADE;


--
-- Name: tracks tracks_blocknumber_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tracks
    ADD CONSTRAINT tracks_blocknumber_fkey FOREIGN KEY (blocknumber) REFERENCES public.blocks(number) ON DELETE CASCADE;


--
-- Name: user_challenges user_challenges_challenge_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_challenges
    ADD CONSTRAINT user_challenges_challenge_id_fkey FOREIGN KEY (challenge_id) REFERENCES public.challenges(id);


--
-- Name: user_events user_events_blocknumber_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_events
    ADD CONSTRAINT user_events_blocknumber_fkey FOREIGN KEY (blocknumber) REFERENCES public.blocks(number) ON DELETE CASCADE;


--
-- Name: user_payout_wallet_history user_payout_wallet_history_blocknumber_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_payout_wallet_history
    ADD CONSTRAINT user_payout_wallet_history_blocknumber_fkey FOREIGN KEY (blocknumber) REFERENCES public.blocks(number) ON DELETE CASCADE;


--
-- Name: users users_blocknumber_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_blocknumber_fkey FOREIGN KEY (blocknumber) REFERENCES public.blocks(number) ON DELETE CASCADE;


--
-- PostgreSQL database dump complete
--

