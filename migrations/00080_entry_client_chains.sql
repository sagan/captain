-- +goose Up
ALTER TABLE entries ADD COLUMN client_dialer_entry_id INTEGER REFERENCES entries(id) ON DELETE RESTRICT;
CREATE INDEX entries_client_dialer ON entries(client_dialer_entry_id);

-- +goose Down
SELECT 1;
