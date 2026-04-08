begin;
alter table comments add column if not exists video_url text;
commit;
