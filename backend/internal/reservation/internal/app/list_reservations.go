package app

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/reservation/internal/domain"
)

// ListReservationsInput is what ListReservations needs. A filter that is nil
// selects everything.
type ListReservationsInput struct {
	// Room selects the reservations of one room.
	Room *domain.RoomID
	// From selects the reservations that end after it.
	From *time.Time
	// To selects the reservations that start before it.
	To *time.Time
	// Limit is the largest number of reservations in the page.
	Limit int
	// Cursor is the cursor of the page before, or empty for the first page.
	Cursor string
}

// cursor is the position after the last reservation of a page: reservations
// are listed in the order of (start, id).
type cursor struct {
	Start time.Time `json:"s"`
	ID    uuid.UUID `json:"i"`
}

// ListReservations returns one page of the reservations that the filters
// select, confirmed and cancelled, in the order of their start and then of
// their id, and the cursor of the next page. The next cursor is empty on the
// last page.
func (s *Reservations) ListReservations(ctx context.Context, in ListReservationsInput) (page []domain.Reservation, next string, err error) {
	var after cursor
	if in.Cursor != "" {
		raw, err := base64.RawURLEncoding.DecodeString(in.Cursor)
		if err != nil || json.Unmarshal(raw, &after) != nil {
			return nil, "", newError(KindInvalidCursor, "The cursor is not one that this API returned.")
		}
	}
	limit := max(in.Limit, 1)
	// One reservation more than the page, to learn whether another page
	// follows.
	page, err = s.store.List(ctx, ListQuery{
		Room:       in.Room,
		From:       in.From,
		To:         in.To,
		AfterStart: after.Start,
		AfterID:    domain.ReservationID(after.ID),
		Limit:      limit + 1,
	})
	if err != nil {
		return nil, "", fmt.Errorf("list reservations: %w", err)
	}
	if len(page) > limit {
		page = page[:limit]
		last := page[len(page)-1]
		raw, err := json.Marshal(cursor{Start: last.Period().Start.UTC(), ID: uuid.UUID(last.ID())})
		if err != nil {
			return nil, "", fmt.Errorf("list reservations: cursor: %w", err)
		}
		next = base64.RawURLEncoding.EncodeToString(raw)
	}
	return page, next, nil
}
