-- +goose Up
CREATE TABLE infra_suppliers (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 name TEXT NOT NULL,
 url TEXT NOT NULL DEFAULT '',
 contact TEXT NOT NULL DEFAULT '',
 notes TEXT NOT NULL DEFAULT ''
);
CREATE TABLE infra_assets (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 supplier_id INTEGER NOT NULL REFERENCES infra_suppliers(id),
 node_id INTEGER REFERENCES nodes(id) ON DELETE SET NULL,
 name TEXT NOT NULL,
 external_ref TEXT NOT NULL DEFAULT '',
 currency TEXT NOT NULL,
 amount_minor INTEGER NOT NULL CHECK(amount_minor>=0),
 period_months INTEGER NOT NULL,
 anchor_day INTEGER NOT NULL,
 next_due TEXT NOT NULL,
 remind_days INTEGER NOT NULL DEFAULT 7,
 active INTEGER NOT NULL DEFAULT 1,
 notes TEXT NOT NULL DEFAULT '',
 revision INTEGER NOT NULL DEFAULT 1
);
CREATE INDEX infra_assets_due ON infra_assets(active,next_due);
CREATE TABLE infra_payments (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 asset_id INTEGER NOT NULL REFERENCES infra_assets(id),
 staff_id INTEGER REFERENCES staff(id) ON DELETE SET NULL,
 request_key TEXT NOT NULL,
 request_hash TEXT NOT NULL,
 supplier_name TEXT NOT NULL,
 asset_name TEXT NOT NULL,
 currency TEXT NOT NULL,
 amount_minor INTEGER NOT NULL CHECK(amount_minor>=0),
 paid_date TEXT NOT NULL,
 period_start TEXT NOT NULL,
 next_due TEXT NOT NULL,
 reference TEXT NOT NULL DEFAULT '',
 notes TEXT NOT NULL DEFAULT '',
 created_at INTEGER NOT NULL,
 UNIQUE(asset_id,request_key)
);
CREATE INDEX infra_payments_date ON infra_payments(paid_date,id);
CREATE TABLE infra_reminders (
 asset_id INTEGER NOT NULL REFERENCES infra_assets(id) ON DELETE CASCADE,
 due_date TEXT NOT NULL,
 notice_date TEXT NOT NULL,
 lease_until INTEGER NOT NULL,
 sent_at INTEGER,
 PRIMARY KEY(asset_id,due_date,notice_date)
);

-- +goose Down
DROP TABLE infra_reminders;
DROP TABLE infra_payments;
DROP TABLE infra_assets;
DROP TABLE infra_suppliers;
