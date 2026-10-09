-- +goose Up
CREATE TABLE dns_changes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    node_id INTEGER REFERENCES nodes(id) ON DELETE SET NULL,
    source TEXT NOT NULL,
    actor TEXT NOT NULL,
    zone TEXT NOT NULL,
    name TEXT NOT NULL,
    action TEXT NOT NULL,
    before_json TEXT NOT NULL,
    wanted_json TEXT NOT NULL,
    after_json TEXT NOT NULL DEFAULT 'null',
    outcome TEXT NOT NULL DEFAULT 'unknown',
    created_at INTEGER NOT NULL,
    finished_at INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX dns_changes_node ON dns_changes(node_id, id);
CREATE TABLE node_dns_health (
    node_id INTEGER PRIMARY KEY REFERENCES nodes(id) ON DELETE CASCADE,
    snapshot_json TEXT NOT NULL
);

-- +goose Down
DROP TABLE node_dns_health;
DROP TABLE dns_changes;
