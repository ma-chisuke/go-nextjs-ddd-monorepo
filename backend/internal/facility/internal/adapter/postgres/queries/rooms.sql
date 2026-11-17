-- name: InsertRoom :one
INSERT INTO rooms (id, name, capacity, open_from, open_until, time_zone, created_at)
VALUES (
    sqlc.arg(id), sqlc.arg(name), sqlc.arg(capacity), sqlc.arg(open_from),
    sqlc.arg(open_until), sqlc.arg(time_zone), sqlc.arg(created_at)
)
RETURNING id, name, capacity, open_from, open_until, time_zone, created_at;

-- name: GetRoom :one
SELECT id, name, capacity, open_from, open_until, time_zone, created_at
FROM rooms
WHERE id = sqlc.arg(id);

-- name: ListRooms :many
-- One page of rooms in the order of (name, id), after the given pair.
SELECT id, name, capacity, open_from, open_until, time_zone, created_at
FROM rooms
WHERE (name, id) > (sqlc.arg(after_name)::text, sqlc.arg(after_id)::uuid)
ORDER BY name, id
LIMIT sqlc.arg(page_size);
