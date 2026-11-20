-- +goose Up
-- IPv6 connectivity and resolution on the node: 1 enabled, 0 disabled.
ALTER TABLE nodes ADD COLUMN ipv6 BIGINT NOT NULL DEFAULT 1;

-- +goose Down
ALTER TABLE nodes DROP COLUMN ipv6;
