-- name: GetNodeRelay :one
SELECT * FROM node_relays WHERE node_id = $1;

-- name: ListNodeRelays :many
SELECT * FROM node_relays ORDER BY node_id;

-- name: CreateNodeRelay :one
INSERT INTO node_relays (node_id, port, config, created_at) VALUES ($1, $2, $3, $4)
ON CONFLICT (node_id) DO UPDATE SET node_id = excluded.node_id
RETURNING *;

-- name: SetNodeRelayPort :exec
UPDATE node_relays SET port = $1 WHERE node_id = $2;

-- name: SetNodeRelayRoute :exec
UPDATE node_relays SET outbound = $1, exit_node_id = $2 WHERE node_id = $3;

-- name: GetRelayUser :one
SELECT uuid FROM relay_users WHERE exit_node_id = $1 AND src_node_id = $2;

-- name: AddRelayUser :exec
INSERT INTO relay_users (exit_node_id, src_node_id, uuid) VALUES ($1, $2, $3)
ON CONFLICT (exit_node_id, src_node_id) DO NOTHING;

-- name: ListRelayUsers :many
SELECT * FROM relay_users WHERE exit_node_id = $1 ORDER BY src_node_id;

-- name: SetInboundExit :exec
UPDATE inbounds SET exit_node_id = $1, outbound = $2 WHERE id = $3;

-- name: ListCascadeRules :many
SELECT * FROM cascade_rules WHERE node_id = $1 ORDER BY position, id;

-- name: ListCascadeRulesByExit :many
SELECT * FROM cascade_rules WHERE exit_node_id = $1 ORDER BY node_id, position, id;

-- name: GetCascadeRule :one
SELECT * FROM cascade_rules WHERE id = $1 AND node_id = $2;

-- name: CreateCascadeRule :one
INSERT INTO cascade_rules (node_id, inbound_id, match, exit_node_id, position, created_at)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: UpdateCascadeRule :one
UPDATE cascade_rules SET match = $1, inbound_id = $2, exit_node_id = $3
WHERE id = $4 AND node_id = $5
RETURNING *;

-- name: SetCascadeRulePosition :exec
UPDATE cascade_rules SET position = $1 WHERE id = $2 AND node_id = $3;

-- name: DeleteCascadeRule :exec
DELETE FROM cascade_rules WHERE id = $1 AND node_id = $2;

-- name: DeleteNodeCascadeRules :exec
DELETE FROM cascade_rules WHERE node_id = $1;
