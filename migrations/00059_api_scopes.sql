-- +goose Up
ALTER TABLE api_tokens ADD COLUMN scopes_json TEXT NOT NULL DEFAULT 'null';
