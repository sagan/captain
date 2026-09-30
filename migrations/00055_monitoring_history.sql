-- +goose Up
-- One compressed bucket per node/resolution, with bounded series cardinality.
CREATE TABLE node_resource_stats (
    node_id INTEGER NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
    res TEXT NOT NULL,
    ts INTEGER NOT NULL,
    data BLOB NOT NULL,
    PRIMARY KEY (node_id, res, ts)
);
CREATE INDEX node_resource_stats_retention ON node_resource_stats(res, ts);
ALTER TABLE nodes ADD COLUMN monitor_host_json TEXT NOT NULL DEFAULT '{}';
ALTER TABLE nodes ADD COLUMN monitor_at INTEGER NOT NULL DEFAULT 0;
ALTER TABLE nodes ADD COLUMN host_reported_at INTEGER NOT NULL DEFAULT 0;
ALTER TABLE nodes ADD COLUMN monitor_group TEXT NOT NULL DEFAULT '';
