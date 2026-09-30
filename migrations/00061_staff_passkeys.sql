-- +goose Up
CREATE TABLE staff_passkey_users (
    staff_id INTEGER NOT NULL REFERENCES staff(id) ON DELETE CASCADE,
    rp_id TEXT NOT NULL,
    handle BLOB NOT NULL UNIQUE,
    PRIMARY KEY(staff_id, rp_id)
);
CREATE TABLE staff_passkeys (
    credential_id TEXT PRIMARY KEY,
    staff_id INTEGER NOT NULL REFERENCES staff(id) ON DELETE CASCADE,
    rp_id TEXT NOT NULL,
    name TEXT NOT NULL,
    credential_json TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    last_used_at INTEGER
);
CREATE INDEX staff_passkeys_owner ON staff_passkeys(staff_id, rp_id);
CREATE TABLE passkey_challenges (
    id TEXT PRIMARY KEY,
    staff_id INTEGER REFERENCES staff(id) ON DELETE CASCADE,
    kind TEXT NOT NULL,
    binding TEXT NOT NULL,
    session_json TEXT NOT NULL,
    name TEXT NOT NULL DEFAULT '',
    expires_at INTEGER NOT NULL
);
CREATE INDEX passkey_challenges_expiry ON passkey_challenges(expires_at);

-- +goose Down
DROP TABLE passkey_challenges;
DROP TABLE staff_passkeys;
DROP TABLE staff_passkey_users;
