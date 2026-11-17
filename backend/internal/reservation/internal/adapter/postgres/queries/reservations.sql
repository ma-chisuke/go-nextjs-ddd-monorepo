-- name: InsertReservation :one
INSERT INTO reservations (
    id, room_id, starts_at, ends_at, attendees, organizer_email, status,
    version, created_at, updated_at
)
VALUES (
    sqlc.arg(id), sqlc.arg(room_id), sqlc.arg(starts_at), sqlc.arg(ends_at),
    sqlc.arg(attendees), sqlc.arg(organizer_email), sqlc.arg(status),
    1, sqlc.arg(at), sqlc.arg(at)
)
RETURNING version;

-- name: GetReservation :one
SELECT id, room_id, starts_at, ends_at, attendees, organizer_email, status, version
FROM reservations
WHERE id = sqlc.arg(id);

-- name: UpdateReservation :one
-- Changes the reservation only if it is still at the version that was read,
-- and counts the change.
UPDATE reservations
SET starts_at = sqlc.arg(starts_at),
    ends_at = sqlc.arg(ends_at),
    attendees = sqlc.arg(attendees),
    status = sqlc.arg(status),
    version = version + 1,
    updated_at = sqlc.arg(at)
WHERE id = sqlc.arg(id) AND version = sqlc.arg(read_version)
RETURNING version;

-- name: Overlaps :one
-- The same test as the exclusion constraint reservations_no_overlap, so that
-- it can use the index of the constraint.
SELECT EXISTS (
    SELECT 1
    FROM reservations
    WHERE room_id = sqlc.arg(room_id)
        AND status = 'confirmed'
        AND id <> sqlc.arg(except_id)
        AND tstzrange(starts_at, ends_at, '[)')
        && tstzrange(sqlc.arg(starts_at)::timestamptz, sqlc.arg(ends_at)::timestamptz, '[)')
) AS overlaps;

-- name: ListReservations :many
-- One page of reservations in the order of (starts_at, id), after the given
-- pair. A filter that is null selects every reservation.
SELECT id, room_id, starts_at, ends_at, attendees, organizer_email, status, version
FROM reservations
WHERE (starts_at, id) > (sqlc.arg(after_starts_at)::timestamptz, sqlc.arg(after_id)::uuid)
    AND (sqlc.narg(room_id)::uuid IS NULL OR room_id = sqlc.narg(room_id)::uuid)
    AND (sqlc.narg(window_from)::timestamptz IS NULL OR ends_at > sqlc.narg(window_from)::timestamptz)
    AND (sqlc.narg(window_to)::timestamptz IS NULL OR starts_at < sqlc.narg(window_to)::timestamptz)
ORDER BY starts_at, id
LIMIT sqlc.arg(page_size);
