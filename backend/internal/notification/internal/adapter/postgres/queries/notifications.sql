-- name: InsertNotification :execrows
-- Record the mail of an event, unless a mail of the event is recorded
-- already: then no row is inserted. When another transaction has inserted
-- the event and not ended yet, this waits for it, and inserts only if that
-- transaction rolls back.
INSERT INTO notifications (event_id, event_type, recipient_email, subject, sent_at)
VALUES (
    sqlc.arg(event_id), sqlc.arg(event_type), sqlc.arg(recipient_email),
    sqlc.arg(subject), sqlc.arg(sent_at)
)
ON CONFLICT (event_id) DO NOTHING;
