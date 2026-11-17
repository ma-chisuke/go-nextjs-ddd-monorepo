package http

import (
	"errors"
	"net/http"

	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/facility/internal/app"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/problem"
)

// The kinds of problem that only the Facility API answers with. Each has a
// page docs/problems/<slug>.md.
var (
	roomNotFound        = problem.Type{Slug: "room-not-found", Title: "The room does not exist", Status: http.StatusNotFound}
	roomNameTaken       = problem.Type{Slug: "room-name-taken", Title: "Another room has this name", Status: http.StatusConflict}
	invalidOpeningHours = problem.Type{Slug: "invalid-opening-hours", Title: "The room would close before it opens", Status: http.StatusUnprocessableEntity}
	unknownTimeZone     = problem.Type{Slug: "unknown-time-zone", Title: "The time zone is not known", Status: http.StatusUnprocessableEntity}
	invalidCursor       = problem.Type{Slug: "invalid-cursor", Title: "The cursor is not valid", Status: http.StatusUnprocessableEntity}
)

// problemType is the table from business error to problem. The switch has no
// default on purpose: the linter (exhaustive) fails when app gains a Kind
// that has no entry here (TPL-0008).
func problemType(kind app.Kind) problem.Type {
	switch kind {
	case app.KindRoomNotFound:
		return roomNotFound
	case app.KindRoomNameTaken:
		return roomNameTaken
	case app.KindInvalidOpeningHours:
		return invalidOpeningHours
	case app.KindUnknownTimeZone:
		return unknownTimeZone
	case app.KindInvalidCursor:
		return invalidCursor
	}
	return problem.InternalError
}

// toProblem turns a business error into its problem. Any other error passes
// through and becomes an internal error in the server.
func toProblem(err error) error {
	var business *app.Error
	if errors.As(err, &business) {
		return problem.New(problemType(business.Kind), business.Detail)
	}
	return err
}
