-- +goose Up
CREATE TABLE subscription_templates (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    format TEXT NOT NULL,
    body TEXT NOT NULL,
    updated_at INTEGER NOT NULL
);
CREATE TABLE subscription_profiles (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    is_default INTEGER NOT NULL DEFAULT 0,
    settings_json TEXT NOT NULL DEFAULT '{}',
    updated_at INTEGER NOT NULL
);
CREATE UNIQUE INDEX subscription_profiles_default ON subscription_profiles(is_default) WHERE is_default=1;
CREATE TABLE subscription_profile_templates (
    profile_id INTEGER NOT NULL REFERENCES subscription_profiles(id) ON DELETE CASCADE,
    format TEXT NOT NULL,
    template_id INTEGER NOT NULL REFERENCES subscription_templates(id),
    PRIMARY KEY(profile_id,format)
);
CREATE TABLE user_subscription_profiles (
    user_id INTEGER PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    profile_id INTEGER NOT NULL REFERENCES subscription_profiles(id)
);

-- +goose Down
DROP TABLE user_subscription_profiles;
DROP TABLE subscription_profile_templates;
DROP TABLE subscription_profiles;
DROP TABLE subscription_templates;
