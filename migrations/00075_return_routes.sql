-- +goose Up
ALTER TABLE node_jobs ADD COLUMN route_check INTEGER NOT NULL DEFAULT 0;
CREATE INDEX node_jobs_routes ON node_jobs(node_id, route_check, created_at);
CREATE TABLE node_route_schedules (
    node_id INTEGER PRIMARY KEY REFERENCES nodes(id) ON DELETE CASCADE,
    config_json TEXT NOT NULL,
    next_at INTEGER NOT NULL
);
-- +goose Down
SELECT 1;
