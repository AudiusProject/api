begin;

-- make hll_sketch nullable in api_metrics_counts
alter table api_metrics_counts 
alter column hll_sketch drop not null;

commit;

