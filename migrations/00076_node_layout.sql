-- +goose Up
ALTER TABLE nodes ADD COLUMN display_order INTEGER NOT NULL DEFAULT 0;
UPDATE nodes SET display_order = id;
CREATE TABLE node_display_groups (
    name TEXT PRIMARY KEY,
    position INTEGER NOT NULL
);
INSERT INTO node_display_groups (name, position)
SELECT monitor_group, MIN(id) FROM nodes WHERE monitor_group <> '' GROUP BY monitor_group;

-- +goose Down
-- Forward-only migration.
