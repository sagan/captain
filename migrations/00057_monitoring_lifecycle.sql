-- +goose Up
CREATE TABLE monitor_incidents (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 node_id INTEGER NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
 kind TEXT NOT NULL,
 started_at INTEGER NOT NULL,
 updated_at INTEGER NOT NULL,
 ended_at INTEGER,
 resolution TEXT NOT NULL DEFAULT '',
 value REAL NOT NULL DEFAULT 0,
 threshold REAL NOT NULL DEFAULT 0,
 notified_at INTEGER NOT NULL DEFAULT 0,
 acknowledged_at INTEGER NOT NULL DEFAULT 0,
 acknowledged_by TEXT NOT NULL DEFAULT ''
);
CREATE UNIQUE INDEX monitor_incidents_open ON monitor_incidents(node_id, kind) WHERE ended_at IS NULL;
CREATE INDEX monitor_incidents_retention ON monitor_incidents(ended_at);
CREATE TABLE monitor_windows (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 node_id INTEGER REFERENCES nodes(id) ON DELETE CASCADE,
 kind TEXT NOT NULL CHECK(kind IN ('maintenance', 'silence')),
 starts_at INTEGER NOT NULL,
 ends_at INTEGER NOT NULL CHECK(ends_at > starts_at),
 note TEXT NOT NULL DEFAULT '',
 created_at INTEGER NOT NULL,
 canceled_at INTEGER
);
CREATE INDEX monitor_windows_time ON monitor_windows(ends_at, starts_at);
CREATE TABLE monitor_observations (
 node_id INTEGER PRIMARY KEY REFERENCES nodes(id) ON DELETE CASCADE,
 epoch TEXT NOT NULL,
 observed_at INTEGER NOT NULL,
 last_seen INTEGER NOT NULL
);
CREATE TABLE monitor_intervals (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 node_id INTEGER NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
 starts_at INTEGER NOT NULL,
 ends_at INTEGER NOT NULL CHECK(ends_at > starts_at),
 online INTEGER NOT NULL
);
CREATE INDEX monitor_intervals_node_time ON monitor_intervals(node_id, ends_at);
