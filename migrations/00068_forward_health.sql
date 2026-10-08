-- +goose Up
ALTER TABLE forward_status ADD COLUMN health TEXT NOT NULL DEFAULT '';
ALTER TABLE forward_status ADD COLUMN probe_protocol TEXT NOT NULL DEFAULT '';

ALTER TABLE nodes ADD COLUMN egress_upstreams_json TEXT NOT NULL DEFAULT '[]';

-- +goose Down
SELECT 1;
