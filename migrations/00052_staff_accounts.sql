-- +goose Up
-- Console identities have their own sequence and never consume customer IDs.
CREATE TABLE staff (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    email TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('admin', 'operator', 'support')),
    status TEXT NOT NULL DEFAULT 'active',
    totp_secret TEXT NOT NULL DEFAULT '',
    totp_enabled INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);
INSERT INTO staff (id, email, password_hash, role, status, totp_secret, totp_enabled, created_at, updated_at)
    SELECT id, email, password_hash, role, status, totp_secret, totp_enabled, created_at, updated_at
    FROM users WHERE role IN ('admin', 'operator', 'support');

-- Both numeric ID 1s may exist: foreign keys, not just a flag, bind each
-- session to its namespace. Discard old staff sessions minted by the portal.
CREATE TABLE sessions_new (
    id TEXT PRIMARY KEY,
    user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
    staff_id INTEGER REFERENCES staff(id) ON DELETE CASCADE,
    expires_at INTEGER NOT NULL,
    created_at INTEGER NOT NULL,
    admin INTEGER NOT NULL DEFAULT 0,
    CHECK ((admin = 0 AND user_id IS NOT NULL AND staff_id IS NULL)
        OR (admin = 1 AND user_id IS NULL AND staff_id IS NOT NULL))
);
INSERT INTO sessions_new (id, user_id, expires_at, created_at, admin)
    SELECT s.id, s.user_id, s.expires_at, s.created_at, 0 FROM sessions s
    JOIN users u ON u.id = s.user_id WHERE u.role = 'user' AND s.admin = 0;
INSERT INTO sessions_new (id, staff_id, expires_at, created_at, admin)
    SELECT s.id, s.user_id, s.expires_at, s.created_at, 1 FROM sessions s
    JOIN staff a ON a.id = s.user_id WHERE s.admin = 1;
DROP TABLE sessions;
ALTER TABLE sessions_new RENAME TO sessions;
CREATE INDEX sessions_user ON sessions(user_id);
CREATE INDEX sessions_staff ON sessions(staff_id);

CREATE TABLE api_tokens_new (
    id INTEGER PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES staff(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    token_hash TEXT NOT NULL UNIQUE,
    created_at INTEGER NOT NULL,
    last_used_at INTEGER,
    scope TEXT NOT NULL DEFAULT 'full',
    expires_at INTEGER
);
INSERT INTO api_tokens_new SELECT t.* FROM api_tokens t JOIN staff a ON a.id = t.user_id;
DROP TABLE api_tokens;
ALTER TABLE api_tokens_new RENAME TO api_tokens;
CREATE INDEX api_tokens_user ON api_tokens(user_id);

CREATE TABLE staff_identities (
    provider TEXT NOT NULL,
    subject TEXT NOT NULL,
    user_id INTEGER NOT NULL REFERENCES staff(id) ON DELETE CASCADE,
    email TEXT NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL,
    PRIMARY KEY (provider, subject)
);
CREATE INDEX staff_identities_user ON staff_identities(user_id);
INSERT INTO staff_identities SELECT i.* FROM identities i JOIN staff a ON a.id = i.user_id;
DELETE FROM identities WHERE user_id IN (SELECT id FROM staff);

-- A former staff account may have bought a plan through the old portal.
-- Preserve its customer history as a disabled, passwordless customer; never
-- provision it or reuse its console password. Pure console accounts disappear
-- from users entirely. Existing customers keep IDs, UUIDs and subscriptions.
UPDATE users SET role = 'user', status = 'banned', password_hash = ''
WHERE id IN (SELECT id FROM staff) AND (
    balance_cents <> 0 OR commission_cents <> 0
    OR id IN (SELECT user_id FROM orders)
    OR id IN (SELECT user_id FROM subscriptions)
    OR id IN (SELECT user_id FROM tickets)
    OR id IN (SELECT user_id FROM withdrawals)
    OR id IN (SELECT inviter_id FROM commissions)
    OR id IN (SELECT invitee_id FROM commissions)
    OR id IN (SELECT invited_by FROM users WHERE invited_by IS NOT NULL)
    OR id IN (SELECT redeemed_by FROM gift_codes WHERE redeemed_by IS NOT NULL)
);
DELETE FROM conn_log WHERE user_id IN (SELECT id FROM users WHERE role <> 'user');
DELETE FROM audit_log WHERE user_id IN (SELECT id FROM users WHERE role <> 'user');
DELETE FROM dyn_limits WHERE user_id IN (SELECT id FROM users WHERE role <> 'user');
DELETE FROM traffic_daily WHERE user_id IN (SELECT id FROM users WHERE role <> 'user');
DELETE FROM online_devices WHERE user_id IN (SELECT id FROM users WHERE role <> 'user');
DELETE FROM users WHERE role <> 'user';
ALTER TABLE users DROP COLUMN totp_secret;
ALTER TABLE users DROP COLUMN totp_enabled;
