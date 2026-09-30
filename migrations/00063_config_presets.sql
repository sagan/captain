-- +goose Up
CREATE TABLE config_presets (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 name TEXT NOT NULL,
 kind TEXT NOT NULL,
 payload_json TEXT NOT NULL
);

-- +goose Down
DROP TABLE config_presets;
