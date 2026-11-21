-- +goose Up
ALTER TABLE sessions ADD COLUMN auth_method TEXT NOT NULL DEFAULT 'password';
ALTER TABLE sessions ADD COLUMN auth_identity TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE sessions DROP COLUMN auth_identity;
ALTER TABLE sessions DROP COLUMN auth_method;
