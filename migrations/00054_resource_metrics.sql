-- +goose Up
-- Keep old numeric API fields, but average each metric over its own valid samples.
ALTER TABLE node_stats ADD COLUMN cpu_n INTEGER NOT NULL DEFAULT 0;
ALTER TABLE node_stats ADD COLUMN mem_n INTEGER NOT NULL DEFAULT 0;
ALTER TABLE node_stats ADD COLUMN swap_n INTEGER NOT NULL DEFAULT 0;
ALTER TABLE node_stats ADD COLUMN disk_n INTEGER NOT NULL DEFAULT 0;
ALTER TABLE node_stats ADD COLUMN net_n INTEGER NOT NULL DEFAULT 0;
ALTER TABLE node_stats ADD COLUMN load_n INTEGER NOT NULL DEFAULT 0;
ALTER TABLE node_stats ADD COLUMN connections_n INTEGER NOT NULL DEFAULT 0;
ALTER TABLE node_stats ADD COLUMN procs_n INTEGER NOT NULL DEFAULT 0;
UPDATE node_stats SET cpu_n=samples, mem_n=samples, swap_n=samples, disk_n=samples, net_n=samples, load_n=samples, connections_n=samples, procs_n=samples;
ALTER TABLE nodes ADD COLUMN probe_resources_json TEXT NOT NULL DEFAULT '{}';
ALTER TABLE nodes ADD COLUMN probe_counters_json TEXT NOT NULL DEFAULT '{}';
