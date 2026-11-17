-- name: InsertIdempotencyKey :exec
INSERT INTO idempotency_keys (reservation_id, key, fingerprint, created_at)
VALUES (sqlc.arg(reservation_id), sqlc.arg(key), sqlc.arg(fingerprint), sqlc.arg(created_at));

-- name: GetIdempotencyKey :one
SELECT reservation_id, key, fingerprint
FROM idempotency_keys
WHERE key = sqlc.arg(key);
