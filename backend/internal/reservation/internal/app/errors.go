package app

// Kind is a kind of business error that belongs to a use case rather than to
// a rule of the domain. The HTTP adapter maps every Kind to a problem, and the
// linter (exhaustive) fails when a Kind is added without its mapping
// (TPL-0008).
type Kind int

// The kinds of business error.
const (
	// KindReservationNotFound: no reservation has the given id.
	KindReservationNotFound Kind = iota + 1
	// KindUnknownRoom: the Facility context knows no room with the id.
	KindUnknownRoom
	// KindRoomAlreadyBooked: another confirmed reservation of the room
	// overlaps the period.
	KindRoomAlreadyBooked
	// KindVersionMismatch: the reservation changed since the client read it.
	KindVersionMismatch
	// KindIdempotencyKeyReused: the idempotency key came before with another
	// request.
	KindIdempotencyKeyReused
	// KindFacilityUnavailable: the rooms cannot be checked, because the
	// Facility context does not answer.
	KindFacilityUnavailable
	// KindInvalidCursor: the cursor did not come from this API.
	KindInvalidCursor
)

// Kinds lists every Kind, for the tests that must cover all of them.
func Kinds() []Kind {
	return []Kind{
		KindReservationNotFound, KindUnknownRoom, KindRoomAlreadyBooked, KindVersionMismatch,
		KindIdempotencyKeyReused, KindFacilityUnavailable, KindInvalidCursor,
	}
}

// Error is a business error of a use case: the request was understood, and
// the state of the system rejects it.
type Error struct {
	Kind Kind
	// Detail says, in a sentence for people, why the request was rejected.
	Detail string
	// Err is the cause, when one exists. It is for the log, never for the
	// client.
	Err error
}

func (e *Error) Error() string {
	if e.Err != nil {
		return e.Detail + ": " + e.Err.Error()
	}
	return e.Detail
}

// Unwrap returns the cause.
func (e *Error) Unwrap() error {
	return e.Err
}

func newError(kind Kind, detail string) *Error {
	return &Error{Kind: kind, Detail: detail}
}
