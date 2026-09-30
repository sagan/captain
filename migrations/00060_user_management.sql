-- +goose Up
ALTER TABLE users ADD COLUMN metadata_json TEXT NOT NULL DEFAULT '{}';
ALTER TABLE nodes ADD COLUMN metadata_json TEXT NOT NULL DEFAULT '{}';
CREATE TABLE user_bulk_jobs (
    id TEXT PRIMARY KEY,
    staff_id INTEGER NOT NULL REFERENCES staff(id) ON DELETE CASCADE,
    request_json TEXT NOT NULL,
    preview_json TEXT NOT NULL,
    result_json TEXT NOT NULL DEFAULT '[]',
    created_at INTEGER NOT NULL,
    expires_at INTEGER NOT NULL,
    done_at INTEGER
);
CREATE INDEX user_bulk_jobs_created ON user_bulk_jobs(created_at);

-- +goose Down
DROP TABLE user_bulk_jobs;
ALTER TABLE nodes DROP COLUMN metadata_json;
ALTER TABLE users DROP COLUMN metadata_json;
