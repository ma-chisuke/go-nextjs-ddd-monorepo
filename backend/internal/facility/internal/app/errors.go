package app

// Kind is a kind of business error of the Facility context. The HTTP adapter
// maps every Kind to a problem, and the linter (exhaustive) fails when a
// Kind is added without its mapping (TPL-0008).
type Kind int

// The kinds of business error.
const (
	// KindRoomNotFound: no room has the given id.
	KindRoomNotFound Kind = iota + 1
	// KindRoomNameTaken: another room has the name.
	KindRoomNameTaken
	// KindInvalidOpeningHours: the room would close before it opens.
	KindInvalidOpeningHours
	// KindUnknownTimeZone: the time zone is not an IANA time zone.
	KindUnknownTimeZone
	// KindInvalidCursor: the cursor did not come from this API.
	KindInvalidCursor
)

// Kinds lists every Kind, for tests that must cover all of them.
func Kinds() []Kind {
	return []Kind{KindRoomNotFound, KindRoomNameTaken, KindInvalidOpeningHours, KindUnknownTimeZone, KindInvalidCursor}
}

// Error is a business error: the request was understood and a rule of the
// business rejects it.
type Error struct {
	Kind   Kind
	Detail string
}

func (e *Error) Error() string {
	return e.Detail
}

func newError(kind Kind, detail string) *Error {
	return &Error{Kind: kind, Detail: detail}
}
