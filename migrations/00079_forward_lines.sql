-- +goose Up
CREATE TABLE forward_lines (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    version INTEGER NOT NULL DEFAULT 1,
    protocol TEXT NOT NULL,
    backend TEXT NOT NULL,
    target_inbound_id INTEGER NOT NULL REFERENCES inbounds(id),
    groups_json TEXT NOT NULL,
    updated_at INTEGER NOT NULL
);
CREATE TABLE forward_line_members (
    line_id INTEGER NOT NULL REFERENCES forward_lines(id) ON DELETE CASCADE,
    member_id TEXT NOT NULL,
    node_id INTEGER NOT NULL REFERENCES nodes(id),
    ingress_id INTEGER REFERENCES ingresses(id),
    PRIMARY KEY(line_id, member_id)
);
CREATE INDEX forward_line_members_node ON forward_line_members(node_id);
ALTER TABLE entries ADD COLUMN line_id INTEGER REFERENCES forward_lines(id);
ALTER TABLE entries ADD COLUMN line_member_id TEXT NOT NULL DEFAULT '';
CREATE INDEX entries_line ON entries(line_id);

-- +goose Down
SELECT 1;
