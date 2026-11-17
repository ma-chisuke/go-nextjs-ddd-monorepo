package http

import (
	"errors"
	"net/http"

	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/problem"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/reservation/internal/app"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/reservation/internal/domain"
)

// The kinds of problem that the Reservation API answers with, besides those
// that every API shares (package problem). Each has a page
// docs/problems/<slug>.md.
var (
	// The rules of the domain.
	periodOutOfOrder     = problem.Type{Slug: "period-out-of-order", Title: "The period does not end after it starts", Status: http.StatusUnprocessableEntity}
	periodMisaligned     = problem.Type{Slug: "period-misaligned", Title: "The period does not start and end on a quarter hour", Status: http.StatusUnprocessableEntity}
	periodTooLong        = problem.Type{Slug: "period-too-long", Title: "The period is longer than a reservation may last", Status: http.StatusUnprocessableEntity}
	periodInPast         = problem.Type{Slug: "period-in-past", Title: "The period starts in the past", Status: http.StatusUnprocessableEntity}
	noAttendees          = problem.Type{Slug: "no-attendees", Title: "Nobody attends", Status: http.StatusUnprocessableEntity}
	overCapacity         = problem.Type{Slug: "over-capacity", Title: "The attendees do not fit in the room", Status: http.StatusUnprocessableEntity}
	outsideOpeningHours  = problem.Type{Slug: "outside-opening-hours", Title: "The room is not open for the whole period", Status: http.StatusUnprocessableEntity}
	cancellationTooLate  = problem.Type{Slug: "cancellation-too-late", Title: "It is too late to cancel the reservation", Status: http.StatusUnprocessableEntity}
	reservationCancelled = problem.Type{Slug: "reservation-cancelled", Title: "The reservation is cancelled", Status: http.StatusConflict}

	// The errors of the use cases.
	reservationNotFound  = problem.Type{Slug: "reservation-not-found", Title: "The reservation does not exist", Status: http.StatusNotFound}
	unknownRoom          = problem.Type{Slug: "unknown-room", Title: "The room is not known", Status: http.StatusUnprocessableEntity}
	roomAlreadyBooked    = problem.Type{Slug: "room-already-booked", Title: "The room is booked for the period", Status: http.StatusConflict}
	idempotencyKeyReused = problem.Type{Slug: "idempotency-key-reused", Title: "The idempotency key came with another request", Status: http.StatusUnprocessableEntity}
	facilityUnavailable  = problem.Type{Slug: "facility-unavailable", Title: "The rooms cannot be checked now", Status: http.StatusServiceUnavailable}
	// The same problem as in the Facility API, with the same page.
	invalidCursor = problem.Type{Slug: "invalid-cursor", Title: "The cursor is not valid", Status: http.StatusUnprocessableEntity}
)

// domainProblem is the table from a rule of the domain to its problem. The
// switch has no default on purpose: the linter (exhaustive) fails when the
// domain gains a Kind that has no entry here (TPL-0008).
func domainProblem(kind domain.Kind) problem.Type {
	switch kind {
	case domain.KindPeriodOutOfOrder:
		return periodOutOfOrder
	case domain.KindPeriodMisaligned:
		return periodMisaligned
	case domain.KindPeriodTooLong:
		return periodTooLong
	case domain.KindPeriodInPast:
		return periodInPast
	case domain.KindNoAttendees:
		return noAttendees
	case domain.KindOverCapacity:
		return overCapacity
	case domain.KindOutsideOpeningHours:
		return outsideOpeningHours
	case domain.KindCancellationTooLate:
		return cancellationTooLate
	case domain.KindReservationCancelled:
		return reservationCancelled
	}
	return problem.InternalError
}

// appProblem is the table from an error of a use case to its problem, with
// no default for the same reason.
func appProblem(kind app.Kind) problem.Type {
	switch kind {
	case app.KindReservationNotFound:
		return reservationNotFound
	case app.KindUnknownRoom:
		return unknownRoom
	case app.KindRoomAlreadyBooked:
		return roomAlreadyBooked
	case app.KindVersionMismatch:
		return problem.PreconditionFailed
	case app.KindIdempotencyKeyReused:
		return idempotencyKeyReused
	case app.KindFacilityUnavailable:
		return facilityUnavailable
	case app.KindInvalidCursor:
		return invalidCursor
	}
	return problem.InternalError
}

// toProblem turns a business error into its problem. Any other error passes
// through and becomes an internal error in the server.
func toProblem(err error) error {
	var rule *domain.Error
	if errors.As(err, &rule) {
		return problem.New(domainProblem(rule.Kind), rule.Detail)
	}
	var business *app.Error
	if errors.As(err, &business) {
		return problem.New(appProblem(business.Kind), business.Detail)
	}
	return err
}
