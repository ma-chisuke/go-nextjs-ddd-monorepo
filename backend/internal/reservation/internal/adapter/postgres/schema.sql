-- The schema of the Reservation context: the state that the database must
-- have. pgschema compares it with the database and applies the difference
-- (TPL-0010); sqlc reads it to type the queries (TPL-0011). Names carry no
-- schema: the roles of the context have "reservation, extensions" as their
-- search path, and btree_gist lives in "extensions" (db/bootstrap.sql).

CREATE TABLE reservations (
    id uuid DEFAULT uuidv7() NOT NULL,
    room_id uuid NOT NULL,
    starts_at timestamptz NOT NULL,
    ends_at timestamptz NOT NULL,
    attendees integer NOT NULL,
    organizer_email text NOT NULL,
    status text NOT NULL,
    version integer DEFAULT 1 NOT NULL,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    CONSTRAINT reservations_pkey PRIMARY KEY (id),
    CONSTRAINT reservations_period_check CHECK (starts_at < ends_at),
    CONSTRAINT reservations_attendees_check CHECK (attendees >= 1),
    CONSTRAINT reservations_status_check CHECK (status IN ('confirmed', 'cancelled')),
    CONSTRAINT reservations_version_check CHECK (version >= 1),
    -- Two confirmed reservations of one room never overlap, even when two
    -- requests store them at the same time. A period includes its start and
    -- excludes its end, so one reservation may end when the next starts.
    CONSTRAINT reservations_no_overlap EXCLUDE USING gist (
        room_id WITH =,
        tstzrange(starts_at, ends_at, '[)') WITH &&
    ) WHERE (status = 'confirmed')
);

-- Pages of the list, in the order of (starts_at, id).
CREATE INDEX reservations_starts_at_id_idx ON reservations (starts_at, id);

CREATE TABLE idempotency_keys (
    reservation_id uuid NOT NULL,
    key text NOT NULL,
    fingerprint text NOT NULL,
    created_at timestamptz NOT NULL,
    CONSTRAINT idempotency_keys_pkey PRIMARY KEY (reservation_id),
    CONSTRAINT idempotency_keys_key_key UNIQUE (key),
    CONSTRAINT idempotency_keys_reservation_id_fkey FOREIGN KEY (reservation_id) REFERENCES reservations (id)
);

-- The runtime role reads and writes rows, and nothing else. pgschema manages
-- privileges, so each table states its own.
GRANT SELECT, INSERT, UPDATE, DELETE ON TABLE reservations TO reservation_runtime;
GRANT SELECT, INSERT, UPDATE, DELETE ON TABLE idempotency_keys TO reservation_runtime;

COMMENT ON TABLE reservations IS 'Reservations of meeting rooms, confirmed and cancelled.';
COMMENT ON COLUMN reservations.id IS 'The id of the reservation, a UUID version 7.';
COMMENT ON COLUMN reservations.room_id IS 'The id of the room in the Facility context. No foreign key: the room lives in another context (TPL-0005).';
COMMENT ON COLUMN reservations.starts_at IS 'When the reservation starts. The period includes this instant.';
COMMENT ON COLUMN reservations.ends_at IS 'When the reservation ends. The period excludes this instant.';
COMMENT ON COLUMN reservations.attendees IS 'How many people attend, the organizer included.';
COMMENT ON COLUMN reservations.organizer_email IS 'The email address of the person who booked the room.';
COMMENT ON COLUMN reservations.status IS 'confirmed while the reservation holds the room, cancelled after it was cancelled.';
COMMENT ON COLUMN reservations.version IS 'Counts the changes of the reservation, from 1. A change names the version it read (optimistic locking).';
COMMENT ON COLUMN reservations.created_at IS 'When the room was booked.';
COMMENT ON COLUMN reservations.updated_at IS 'When the reservation last changed.';

COMMENT ON TABLE idempotency_keys IS 'The Idempotency-Key of each booking, so that a repeated request books nothing new.';
COMMENT ON COLUMN idempotency_keys.reservation_id IS 'The reservation that the request booked. It is the primary key: one key per booking.';
COMMENT ON COLUMN idempotency_keys.key IS 'The key that the client sent. Unique.';
COMMENT ON COLUMN idempotency_keys.fingerprint IS 'The SHA-256 of the content of the request, to tell a repeat from another request with the same key.';
COMMENT ON COLUMN idempotency_keys.created_at IS 'When the key was stored.';
