package domain

// Kind is a kind of business error: a rule of reservations that a request
// breaks. The HTTP adapter maps every Kind to a problem, and the linter
// (exhaustive) fails when a Kind is added without its mapping (TPL-0008).
type Kind int

// The kinds of business error.
const (
	// KindPeriodOutOfOrder: the period does not end after it starts.
	KindPeriodOutOfOrder Kind = iota + 1
	// KindPeriodMisaligned: the period does not start and end on the grid of
	// PeriodStep.
	KindPeriodMisaligned
	// KindPeriodTooLong: the period is longer than LongestPeriod.
	KindPeriodTooLong
	// KindPeriodInPast: the period starts before now.
	KindPeriodInPast
	// KindNoAttendees: nobody attends.
	KindNoAttendees
	// KindOverCapacity: the attendees do not fit in the room.
	KindOverCapacity
	// KindOutsideOpeningHours: the room is not open for the whole period.
	KindOutsideOpeningHours
	// KindCancellationTooLate: the reservation starts in less than
	// CancellationNotice, so it can no longer be cancelled.
	KindCancellationTooLate
	// KindReservationCancelled: the reservation was cancelled, and a
	// cancelled reservation is neither changed nor cancelled again.
	KindReservationCancelled
)

// Kinds lists every Kind, for the tests that must cover all of them.
func Kinds() []Kind {
	return []Kind{
		KindPeriodOutOfOrder, KindPeriodMisaligned, KindPeriodTooLong, KindPeriodInPast,
		KindNoAttendees, KindOverCapacity, KindOutsideOpeningHours,
		KindCancellationTooLate, KindReservationCancelled,
	}
}

// Error is a business error: a rule of reservations rejects the request.
type Error struct {
	Kind Kind
	// Detail says, in a sentence for people, what in this request breaks
	// the rule.
	Detail string
}

func (e *Error) Error() string {
	return e.Detail
}

func newError(kind Kind, detail string) *Error {
	return &Error{Kind: kind, Detail: detail}
}
