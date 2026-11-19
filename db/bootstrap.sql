-- What a database needs before any context can use it, and what pgschema
-- does not manage: extensions, schemas, and roles (TPL-0009, TPL-0010).
--
-- Run it once per database, as a superuser, or as a role that owns the
-- database and may create roles without being a superuser, such as the master
-- user of Aurora (TPL-0016). It is safe to run again.
-- Each bounded context gets
--   a schema of its own,
--   an owner role, which owns the schema and changes it (pgschema runs as it),
--   a runtime role, which the service connects as and which can only read
--   and write rows of its own schema. The schema.sql of the context grants
--   it those rights on tables, because pgschema manages privileges too.
-- No role can see the schema of another context, so a query across contexts
-- fails in the database, whatever the code does (TPL-0005).
--
-- Roles are created without passwords. Each environment sets them, or uses
-- another way to sign in; db/local.sql does it for Docker Compose.

-- Extensions live in a schema of their own, on the search path of every role.
CREATE SCHEMA IF NOT EXISTS extensions;
-- btree_gist lets an exclusion constraint compare a uuid with "=" next to a
-- range with "&&" (the Reservation context).
CREATE EXTENSION IF NOT EXISTS btree_gist WITH SCHEMA extensions;

-- Nobody creates objects in the public schema.
REVOKE CREATE ON SCHEMA public FROM PUBLIC;

-- Facility ------------------------------------------------------------------

DO $$
BEGIN
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'facility_owner') THEN
        CREATE ROLE facility_owner LOGIN;
    END IF;
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'facility_runtime') THEN
        CREATE ROLE facility_runtime LOGIN;
    END IF;
    -- A role that is not a superuser may hand a schema to the owner role, and
    -- grant rights on it, only as a member of that role (PostgreSQL 16 and
    -- later). Its creator holds the ADMIN OPTION on it, so it can join it; a
    -- superuser needs no membership.
    IF NOT pg_has_role('facility_owner', 'USAGE') THEN
        GRANT facility_owner TO CURRENT_USER WITH INHERIT TRUE, SET TRUE;
    END IF;
END
$$;

CREATE SCHEMA IF NOT EXISTS facility AUTHORIZATION facility_owner;
REVOKE ALL ON SCHEMA facility FROM PUBLIC;
GRANT USAGE ON SCHEMA facility TO facility_runtime;
GRANT USAGE ON SCHEMA extensions TO facility_owner, facility_runtime;
ALTER ROLE facility_owner SET search_path = facility, extensions;
ALTER ROLE facility_runtime SET search_path = facility, extensions;

-- Reservation ---------------------------------------------------------------

DO $$
BEGIN
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'reservation_owner') THEN
        CREATE ROLE reservation_owner LOGIN;
    END IF;
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'reservation_runtime') THEN
        CREATE ROLE reservation_runtime LOGIN;
    END IF;
    IF NOT pg_has_role('reservation_owner', 'USAGE') THEN
        GRANT reservation_owner TO CURRENT_USER WITH INHERIT TRUE, SET TRUE;
    END IF;
END
$$;

CREATE SCHEMA IF NOT EXISTS reservation AUTHORIZATION reservation_owner;
REVOKE ALL ON SCHEMA reservation FROM PUBLIC;
GRANT USAGE ON SCHEMA reservation TO reservation_runtime;
GRANT USAGE ON SCHEMA extensions TO reservation_owner, reservation_runtime;
ALTER ROLE reservation_owner SET search_path = reservation, extensions;
ALTER ROLE reservation_runtime SET search_path = reservation, extensions;

-- Notification --------------------------------------------------------------

DO $$
BEGIN
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'notification_owner') THEN
        CREATE ROLE notification_owner LOGIN;
    END IF;
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'notification_runtime') THEN
        CREATE ROLE notification_runtime LOGIN;
    END IF;
    IF NOT pg_has_role('notification_owner', 'USAGE') THEN
        GRANT notification_owner TO CURRENT_USER WITH INHERIT TRUE, SET TRUE;
    END IF;
END
$$;

CREATE SCHEMA IF NOT EXISTS notification AUTHORIZATION notification_owner;
REVOKE ALL ON SCHEMA notification FROM PUBLIC;
GRANT USAGE ON SCHEMA notification TO notification_runtime;
GRANT USAGE ON SCHEMA extensions TO notification_owner, notification_runtime;
ALTER ROLE notification_owner SET search_path = notification, extensions;
ALTER ROLE notification_runtime SET search_path = notification, extensions;
