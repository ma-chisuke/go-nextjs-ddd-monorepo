-- The schema of the Facility context: the state that the database must have.
-- pgschema compares it with the database and applies the difference
-- (TPL-0010); sqlc reads it to type the queries (TPL-0011). Names carry no
-- schema: the roles of the context have "facility" as their search path.

CREATE TABLE rooms (
    id uuid DEFAULT uuidv7() NOT NULL,
    name text NOT NULL,
    capacity integer NOT NULL,
    open_from time NOT NULL,
    open_until time NOT NULL,
    time_zone text NOT NULL,
    created_at timestamptz NOT NULL,
    CONSTRAINT rooms_pkey PRIMARY KEY (id),
    CONSTRAINT rooms_name_key UNIQUE (name),
    CONSTRAINT rooms_capacity_check CHECK (capacity >= 1 AND capacity <= 1000),
    CONSTRAINT rooms_open_hours_check CHECK (open_from < open_until)
);

-- The runtime role reads and writes rows, and nothing else. pgschema manages
-- privileges, so each table states its own.
GRANT SELECT, INSERT, UPDATE, DELETE ON TABLE rooms TO facility_runtime;

COMMENT ON TABLE rooms IS 'Meeting rooms that can be booked.';
COMMENT ON COLUMN rooms.id IS 'The id of the room, a UUID version 7.';
COMMENT ON COLUMN rooms.name IS 'The name that people use for the room. Unique.';
COMMENT ON COLUMN rooms.capacity IS 'The largest number of people the room holds.';
COMMENT ON COLUMN rooms.open_from IS 'The time of day from which the room can be used, in time_zone.';
COMMENT ON COLUMN rooms.open_until IS 'The time of day until which the room can be used, in time_zone.';
COMMENT ON COLUMN rooms.time_zone IS 'The IANA time zone in which open_from and open_until are read.';
COMMENT ON COLUMN rooms.created_at IS 'When the room was registered.';
