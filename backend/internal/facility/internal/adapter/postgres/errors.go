// Package postgres is the database side of the Facility context: its schema
// (schema.sql), its queries (queries/), and the code that sqlc generates from
// them (*.gen.go).
package postgres

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

// uniqueViolation is the SQLSTATE of a broken unique constraint.
const uniqueViolation = "23505"

// IsUniqueViolation reports whether err says that the unique constraint named
// constraint was broken.
func IsUniqueViolation(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == uniqueViolation && pgErr.ConstraintName == constraint
}
