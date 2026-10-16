-- name: CreateExternalSubscription :one
INSERT INTO external_subscriptions (tariff_id, name, url, enabled, interval_sec, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $6)
RETURNING *;

-- name: GetExternalSubscription :one
SELECT * FROM external_subscriptions WHERE id = $1;

-- name: ListExternalSubscriptions :many
SELECT * FROM external_subscriptions WHERE tariff_id = $1 ORDER BY id;

-- name: ListAllExternalSubscriptions :many
SELECT * FROM external_subscriptions WHERE enabled = true ORDER BY id;

-- name: UpdateExternalSubscription :one
UPDATE external_subscriptions
SET name = $2, url = $3, enabled = $4, interval_sec = $5, updated_at = $6
WHERE id = $1
RETURNING *;

-- name: DeleteExternalSubscription :exec
DELETE FROM external_subscriptions WHERE id = $1;

-- name: SetExternalSubscriptionFetch :exec
UPDATE external_subscriptions
SET last_fetch = $2, last_error = $3, last_count = $4
WHERE id = $1;

-- name: CreateExternalServer :one
INSERT INTO external_servers (external_sub_id, name, link, enabled, mihomo, created_at)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetExternalServer :one
SELECT * FROM external_servers WHERE id = $1;

-- name: ListExternalServers :many
SELECT * FROM external_servers WHERE external_sub_id = $1 ORDER BY id;

-- name: ListExternalServersForTariff :many
SELECT s.id, s.name, s.link, s.mihomo, e.name AS group_name
FROM external_servers s
JOIN external_subscriptions e ON e.id = s.external_sub_id
WHERE e.tariff_id = $1 AND e.enabled = true AND s.enabled = true
ORDER BY e.id, s.id;

-- name: ReplaceExternalServers :exec
DELETE FROM external_servers WHERE external_sub_id = $1;

-- name: SetExternalServerEnabled :exec
UPDATE external_servers SET enabled = $3 WHERE id = $2 AND external_sub_id = $1;

-- name: SetExternalServersEnabledAll :exec
UPDATE external_servers SET enabled = $2 WHERE external_sub_id = $1;