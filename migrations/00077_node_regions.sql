-- +goose Up
-- Empty mode retains legacy manual/hidden semantics without rewriting JSON.
ALTER TABLE nodes ADD COLUMN region_mode TEXT NOT NULL DEFAULT '';
ALTER TABLE nodes ADD COLUMN region_detection_json TEXT NOT NULL DEFAULT '{}';

-- +goose Down
-- Forward-only migration.
