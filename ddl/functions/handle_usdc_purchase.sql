create or replace function handle_usdc_purchase() returns trigger as $$
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
$$ language plpgsql;

do $$ begin
  create trigger on_usdc_purchase
  after insert on usdc_purchases
  for each row execute procedure handle_usdc_purchase();
exception
  when others then null;
end $$;


-- Mirror of handle_usdc_purchase for the new indexer's table. Resolves
-- seller_user_id from current content ownership (same CASE used in
-- v_usdc_purchases). Notification shape and group_id format match the legacy
-- trigger so the two can dual-fire during cutover (second insert is a no-op
-- via on conflict). extra_amount and vendor are emitted as null in the new
-- payload because they aren't denormalized onto sol_purchases.
create or replace function handle_sol_purchase() returns trigger as $$
declare
  resolved_seller_user_id integer;
begin
  if new.is_valid is not true then
    return null;
  end if;

  if new.content_type = 'track' then
    select owner_id into resolved_seller_user_id
      from tracks
     where track_id = new.content_id
       and is_current = true
     limit 1;
  else
    select playlist_owner_id into resolved_seller_user_id
      from playlists
     where playlist_id = new.content_id
       and is_current = true
     limit 1;
  end if;

  if resolved_seller_user_id is null then
    return null;
  end if;

  insert into notification
        (slot, user_ids, timestamp, type, specifier, group_id, data)
      values
        (
          new.slot,
          ARRAY [resolved_seller_user_id],
          new.created_at,
          'usdc_purchase_seller',
          new.buyer_user_id,
          'usdc_purchase_seller:' || 'seller_user_id:' || resolved_seller_user_id || ':buyer_user_id:' || new.buyer_user_id || ':content_id:' || new.content_id || ':content_type:' || new.content_type,
          json_build_object(
              'content_type',   new.content_type,
              'buyer_user_id',  new.buyer_user_id,
              'seller_user_id', resolved_seller_user_id,
              'amount',         new.amount,
              'extra_amount',   null,
              'content_id',     new.content_id,
              'vendor',         null
          )
        ),
        (
          new.slot,
          ARRAY [new.buyer_user_id],
          new.created_at,
          'usdc_purchase_buyer',
          new.buyer_user_id,
          'usdc_purchase_buyer:' || 'seller_user_id:' || resolved_seller_user_id || ':buyer_user_id:' || new.buyer_user_id || ':content_id:' || new.content_id || ':content_type:' || new.content_type,
          json_build_object(
              'content_type',   new.content_type,
              'buyer_user_id',  new.buyer_user_id,
              'seller_user_id', resolved_seller_user_id,
              'amount',         new.amount,
              'extra_amount',   null,
              'content_id',     new.content_id,
              'vendor',         null
          )
        )
      on conflict do nothing;

  return null;
  exception
    when others then
        raise warning 'An error occurred in %: %', tg_name, sqlerrm;
        return null;
end;
$$ language plpgsql;

do $$ begin
  create trigger on_sol_purchase
  after insert on sol_purchases
  for each row execute procedure handle_sol_purchase();
exception
  when others then null;
end $$;
