-- +goose Up
-- Existing duplicates stay editable; runtime DNS protection also covers them.
ALTER TABLE nodes ADD COLUMN domain_shared INTEGER NOT NULL DEFAULT 0;

-- +goose Down
SELECT 1;
