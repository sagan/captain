-- +goose Up
ALTER TABLE nodes ADD COLUMN core_inventory_json TEXT NOT NULL DEFAULT 'null';

-- +goose Down
SELECT 1;
