-- +goose Up
-- Account IDs are editable. The ID used by agents is immutable, including
-- while an offline node retains an old state or retries an old traffic batch.
ALTER TABLE users ADD COLUMN agent_id INTEGER NOT NULL DEFAULT 0;
UPDATE users SET agent_id = id;
CREATE UNIQUE INDEX users_agent_id ON users(agent_id);
CREATE TABLE account_sequences (name TEXT PRIMARY KEY, value INTEGER NOT NULL);
INSERT INTO account_sequences (name,value) SELECT 'agent', COALESCE(MAX(id),0) FROM users;
