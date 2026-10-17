-- +goose Up
ALTER TABLE nodes ADD COLUMN wireguard_json TEXT NOT NULL DEFAULT 'null';
ALTER TABLE nodes ADD COLUMN wireguard_reported_at INTEGER;
CREATE TABLE node_wireguard_links (
    slot INTEGER PRIMARY KEY AUTOINCREMENT CHECK(slot BETWEEN 1 AND 16383),
    id TEXT NOT NULL UNIQUE,
    exit_id INTEGER NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
    transit_id INTEGER NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
    user_inbound_id INTEGER NOT NULL UNIQUE REFERENCES inbounds(id) ON DELETE CASCADE,
    entry_id INTEGER NOT NULL UNIQUE REFERENCES entries(id) ON DELETE CASCADE,
    version INTEGER NOT NULL DEFAULT 1,
    transit_key TEXT NOT NULL,
    exit_key TEXT NOT NULL,
    endpoint TEXT NOT NULL,
    listen_port INTEGER NOT NULL CHECK(listen_port BETWEEN 1 AND 65535),
    mtu INTEGER NOT NULL CHECK(mtu BETWEEN 1280 AND 1420),
    UNIQUE(exit_id,transit_id),
    CHECK(exit_id <> transit_id)
);
-- AUTOINCREMENT deliberately never reuses a deleted slot. Old nodes/sockets
-- may still hold the previous link's interface and routing identities.
-- +goose StatementBegin
CREATE TRIGGER wireguard_link_cleanup AFTER DELETE ON node_wireguard_links BEGIN
    DELETE FROM inbounds WHERE id=OLD.user_inbound_id;
    DELETE FROM entries WHERE id=OLD.entry_id;
END;
-- +goose StatementEnd

-- +goose Down
DROP TRIGGER wireguard_link_cleanup;
DROP TABLE node_wireguard_links;
ALTER TABLE nodes DROP COLUMN wireguard_reported_at;
ALTER TABLE nodes DROP COLUMN wireguard_json;
