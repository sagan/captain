-- +goose Up
ALTER TABLE nodes ADD COLUMN node_mode TEXT NOT NULL DEFAULT 'full';
ALTER TABLE nodes ADD COLUMN reported_mode TEXT NOT NULL DEFAULT '';
ALTER TABLE nodes ADD COLUMN mode_reported_at INTEGER NOT NULL DEFAULT 0;
ALTER TABLE nodes ADD COLUMN mode_managed INTEGER NOT NULL DEFAULT 0;
ALTER TABLE nodes ADD COLUMN mode_error TEXT NOT NULL DEFAULT '';

ALTER TABLE nodes ADD COLUMN mode_revision INTEGER NOT NULL DEFAULT 0;
ALTER TABLE nodes ADD COLUMN reported_mode_revision INTEGER NOT NULL DEFAULT 0;

-- Also protect internal topology writers (e.g. reverse resources).
-- +goose StatementBegin
CREATE TRIGGER monitor_inbound_insert BEFORE INSERT ON inbounds
WHEN EXISTS (SELECT 1 FROM nodes WHERE id = NEW.node_id AND (node_mode = 'monitor' OR reported_mode = 'monitor' OR (mode_managed = 1 AND (reported_mode = '' OR mode_revision <> reported_mode_revision)))) BEGIN
    SELECT RAISE(ABORT, 'monitor-only node cannot have inbounds');
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER monitor_inbound_update BEFORE UPDATE ON inbounds
WHEN EXISTS (SELECT 1 FROM nodes WHERE id = NEW.node_id AND (node_mode = 'monitor' OR reported_mode = 'monitor' OR (mode_managed = 1 AND (reported_mode = '' OR mode_revision <> reported_mode_revision)))) BEGIN
    SELECT RAISE(ABORT, 'monitor-only node cannot have inbounds');
END;
-- +goose StatementEnd

-- +goose Down
DROP TRIGGER monitor_inbound_update;
DROP TRIGGER monitor_inbound_insert;
ALTER TABLE nodes DROP COLUMN reported_mode_revision;
ALTER TABLE nodes DROP COLUMN mode_revision;
ALTER TABLE nodes DROP COLUMN mode_error;
ALTER TABLE nodes DROP COLUMN mode_managed;
ALTER TABLE nodes DROP COLUMN mode_reported_at;
ALTER TABLE nodes DROP COLUMN reported_mode;
ALTER TABLE nodes DROP COLUMN node_mode;
