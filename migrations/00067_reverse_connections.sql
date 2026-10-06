-- +goose Up
CREATE TABLE node_reverse_links (
    id TEXT PRIMARY KEY,
    exit_id INTEGER NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
    transit_id INTEGER NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
    user_inbound_id INTEGER NOT NULL UNIQUE REFERENCES inbounds(id) ON DELETE CASCADE,
    receiver_inbound_id INTEGER NOT NULL UNIQUE REFERENCES inbounds(id) ON DELETE CASCADE,
    entry_id INTEGER NOT NULL UNIQUE REFERENCES entries(id) ON DELETE CASCADE,
    version INTEGER NOT NULL DEFAULT 1,
    UNIQUE(exit_id, transit_id),
    CHECK(exit_id <> transit_id)
);
-- Removing either node/link must retire only the resources it owns, including
-- the transit resources when it is the exit node that was removed.
-- +goose StatementBegin
CREATE TRIGGER reverse_link_cleanup AFTER DELETE ON node_reverse_links BEGIN
    DELETE FROM inbounds WHERE id IN (OLD.user_inbound_id, OLD.receiver_inbound_id);
    DELETE FROM entries WHERE id = OLD.entry_id;
END;
-- +goose StatementEnd

-- +goose Down
DROP TRIGGER reverse_link_cleanup;
DROP TABLE node_reverse_links;
