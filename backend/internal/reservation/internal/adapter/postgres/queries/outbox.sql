-- name: InsertOutboxEvent :exec
INSERT INTO outbox (id, event_type, payload, created_at)
VALUES (sqlc.arg(id), sqlc.arg(event_type), sqlc.arg(payload), sqlc.arg(created_at));

-- name: LockUnpublishedEvents :many
-- Up to batch_size events that wait, the oldest first, locked until the end
-- of the transaction. The events that another relay holds are left out, so
-- two relays never take the same event at once.
SELECT id, payload
FROM outbox
WHERE published_at IS NULL
ORDER BY id
LIMIT sqlc.arg(batch_size)
FOR UPDATE SKIP LOCKED;

-- name: MarkEventsPublished :execrows
UPDATE outbox
SET published_at = sqlc.arg(published_at)::timestamptz
WHERE id = ANY(sqlc.arg(ids)::uuid[]) AND published_at IS NULL;

-- name: DeletePublishedEvents :execrows
DELETE FROM outbox
WHERE published_at < sqlc.arg(before)::timestamptz;
