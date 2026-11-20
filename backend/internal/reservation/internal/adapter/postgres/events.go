package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/event"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/reservation/internal/domain"
)

// The name of this context in the source and the types of its events.
const contextName = "reservation"

// The types of the integration events of Reservation. The JSON Schema of each
// is contracts/events/reservation/<Name>.v<N>.json.
var (
	typeReservationConfirmed = event.Type(contextName, "ReservationConfirmed", 1)
	typeReservationCancelled = event.Type(contextName, "ReservationCancelled", 1)
)

// reservationConfirmedData is the data of reservation.ReservationConfirmed.v1.
type reservationConfirmedData struct {
	ReservationID  uuid.UUID `json:"reservationId"`
	RoomID         uuid.UUID `json:"roomId"`
	StartsAt       time.Time `json:"startsAt"`
	EndsAt         time.Time `json:"endsAt"`
	Attendees      int32     `json:"attendees"`
	OrganizerEmail string    `json:"organizerEmail"`
}

// reservationCancelledData is the data of reservation.ReservationCancelled.v1.
type reservationCancelledData struct {
	ReservationID  uuid.UUID `json:"reservationId"`
	RoomID         uuid.UUID `json:"roomId"`
	StartsAt       time.Time `json:"startsAt"`
	EndsAt         time.Time `json:"endsAt"`
	OrganizerEmail string    `json:"organizerEmail"`
}

// integrationEvent returns the integration event, with the id, that publishes
// a domain event. The domain event is what happened in the words of this
// context; the integration event is the published contract of it
// (contracts/events/reservation/). A domain event without an integration
// event is an error, never dropped: a new type of domain event needs its
// case here, and its contract.
func integrationEvent(id uuid.UUID, e domain.Event) (event.Envelope, error) {
	switch e := e.(type) {
	case domain.ReservationConfirmed:
		return event.New(id, event.Source(contextName), typeReservationConfirmed, e.Reservation.String(), e.OccurredAt,
			reservationConfirmedData{
				ReservationID:  uuid.UUID(e.Reservation),
				RoomID:         uuid.UUID(e.Room),
				StartsAt:       e.Period.Start.UTC(),
				EndsAt:         e.Period.End.UTC(),
				Attendees:      e.Attendees,
				OrganizerEmail: e.Organizer,
			})
	case domain.ReservationCancelled:
		return event.New(id, event.Source(contextName), typeReservationCancelled, e.Reservation.String(), e.OccurredAt,
			reservationCancelledData{
				ReservationID:  uuid.UUID(e.Reservation),
				RoomID:         uuid.UUID(e.Room),
				StartsAt:       e.Period.Start.UTC(),
				EndsAt:         e.Period.End.UTC(),
				OrganizerEmail: e.Organizer,
			})
	}
	return event.Envelope{}, fmt.Errorf("store reservation: the domain event %T has no integration event", e)
}

// outboxEvents returns the rows of the outbox that publish the domain events
// of a change stored at the instant. Each event gets a new id, a UUID version
// 7, and the relay publishes the events in the order of their ids, which is
// close to the order of their changes. The rows are made before the
// transaction that stores them, so a domain event without a mapping stores
// nothing at all.
func outboxEvents(events []domain.Event, at time.Time) ([]InsertOutboxEventParams, error) {
	rows := make([]InsertOutboxEventParams, 0, len(events))
	for _, e := range events {
		id, err := uuid.NewV7()
		if err != nil {
			return nil, fmt.Errorf("store reservation: the id of an event: %w", err)
		}
		envelope, err := integrationEvent(id, e)
		if err != nil {
			return nil, err
		}
		payload, err := json.Marshal(envelope)
		if err != nil {
			return nil, fmt.Errorf("store reservation: event %s: %w", envelope.Type, err)
		}
		rows = append(rows, InsertOutboxEventParams{ID: id, EventType: envelope.Type, Payload: payload, CreatedAt: at})
	}
	return rows, nil
}

// insertEvents writes rows into the outbox, in the transaction of q.
func insertEvents(ctx context.Context, q *Queries, rows []InsertOutboxEventParams) error {
	for _, row := range rows {
		if err := q.InsertOutboxEvent(ctx, row); err != nil {
			return err
		}
	}
	return nil
}
