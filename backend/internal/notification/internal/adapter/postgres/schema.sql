-- The schema of the Notification context: the state that the database must
-- have. pgschema compares it with the database and applies the difference
-- (TPL-0010); sqlc reads it to type the queries (TPL-0011). Names carry no
-- schema: the roles of the context have "notification" as their search path.

CREATE TABLE notifications (
    id uuid DEFAULT uuidv7() NOT NULL,
    event_id uuid NOT NULL,
    event_type text NOT NULL,
    recipient_email text NOT NULL,
    subject text NOT NULL,
    sent_at timestamptz NOT NULL,
    CONSTRAINT notifications_pkey PRIMARY KEY (id),
    -- One mail per event, however often the event is delivered (TPL-0013).
    CONSTRAINT notifications_event_id_key UNIQUE (event_id)
);

-- The runtime role reads and writes rows, and nothing else. pgschema manages
-- privileges, so each table states its own.
GRANT SELECT, INSERT, UPDATE, DELETE ON TABLE notifications TO notification_runtime;

COMMENT ON TABLE notifications IS 'The mails that Notification sent, one for each event that it handled.';
COMMENT ON COLUMN notifications.id IS 'The id of the notification, a UUID version 7.';
COMMENT ON COLUMN notifications.event_id IS 'The id of the event that the mail reports. Unique, so an event that comes again sends no second mail.';
COMMENT ON COLUMN notifications.event_type IS 'The type of that event, such as reservation.ReservationConfirmed.v1.';
COMMENT ON COLUMN notifications.recipient_email IS 'The address that the mail went to.';
COMMENT ON COLUMN notifications.subject IS 'The subject of the mail.';
COMMENT ON COLUMN notifications.sent_at IS 'When Notification sent the mail: the time just before it handed the mail to the mail server.';
