-- +goose Up
ALTER TABLE ingresses ADD COLUMN port_mappings_json TEXT NOT NULL DEFAULT '[]';
ALTER TABLE ingresses ADD COLUMN require_ingress INTEGER NOT NULL DEFAULT 0;

-- +goose Down
SELECT 1;
