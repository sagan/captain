-- +goose Up
-- Keep automatic customer IDs independent of IDs selected by administrators.
-- Preserve the numbering position of existing installations; no IDs change.
INSERT INTO account_sequences (name, value) SELECT 'user', COALESCE(MAX(id), 0) FROM users;
