-- +goose Up
-- Empty preserves the original IPv6-first routing of existing connections.
ALTER TABLE node_wireguard_links ADD COLUMN domain_preference TEXT NOT NULL DEFAULT '';
-- Existing host forwarding rules remain administrator-managed until opted in.
ALTER TABLE node_wireguard_links ADD COLUMN auto_forwarding INTEGER NOT NULL DEFAULT 0;

-- +goose Down
-- Forward-only migration.
