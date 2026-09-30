-- +goose Up
ALTER TABLE node_ping_stats ADD COLUMN quality_json TEXT NOT NULL DEFAULT '{}';
CREATE TABLE node_ping_cursors (
 node_id INTEGER NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
 task_id INTEGER NOT NULL,
 name TEXT NOT NULL,
 epoch TEXT NOT NULL,
 sequence INTEGER NOT NULL,
 last_ms REAL NOT NULL,
 seen_at INTEGER NOT NULL,
 PRIMARY KEY(node_id, task_id, name, epoch)
);
CREATE INDEX node_ping_cursors_retention ON node_ping_cursors(seen_at);
