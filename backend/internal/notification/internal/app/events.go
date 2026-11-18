package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/event"
)

// The types of the events that Notification handles. The JSON Schema of the
// data of each is in contracts/events/reservation/, and
// contracts/events/notification.asyncapi.yaml lists them.
var (
	typeReservationConfirmed = event.Type("reservation", "ReservationConfirmed", 1)
	typeReservationCancelled = event.Type("reservation", "ReservationCancelled", 1)
)

// This file is the anticorruption layer between the events of Reservation and
// Notification: it reads the data of each event into the structs below, in
// the words of Notification, and nothing else in the context sees the shape
// of an event. A property that Notification does not read is ignored, so
// Reservation can add properties without breaking it (TPL-0013).

// booking is what both events tell about a reservation.
type booking struct {
	ReservationID uuid.UUID `json:"reservationId"`
	// RoomID is all that an event says about the room: it carries no name.
	RoomID   uuid.UUID `json:"roomId"`
	StartsAt time.Time `json:"startsAt"`
	EndsAt   time.Time `json:"endsAt"`
	// Organizer is the email address of the person who organizes the
	// meeting, to whom the mail goes.
	Organizer string `json:"organizerEmail"`
}

// confirmation is the data of reservation.ReservationConfirmed.v1: the
// reservation holds its room for its period. Reservation publishes it when a
// room is booked, and again when the reservation is rescheduled.
type confirmation struct {
	booking

	Attendees int `json:"attendees"`
}

// cancellation is the data of reservation.ReservationCancelled.v1: the
// reservation is cancelled, and its room is free for its period.
type cancellation struct {
	booking
}

// compose returns the mail that an event asks for. known is false for an
// event of a type that Notification does not handle. An error says that the
// data of an event of a known type is not what its contract promises.
func compose(e event.Envelope) (m Message, known bool, err error) {
	switch e.Type {
	case typeReservationConfirmed:
		var c confirmation
		if err := decode(e.Data, &c); err != nil {
			return Message{}, true, err
		}
		problems := c.check()
		if c.Attendees < 1 {
			problems = append(problems, errors.New("attendees is missing or below 1"))
		}
		if len(problems) > 0 {
			return Message{}, true, invalid(problems)
		}
		return confirmedMail(c), true, nil
	case typeReservationCancelled:
		var c cancellation
		if err := decode(e.Data, &c); err != nil {
			return Message{}, true, err
		}
		if problems := c.check(); len(problems) > 0 {
			return Message{}, true, invalid(problems)
		}
		return cancelledMail(c), true, nil
	default:
		return Message{}, false, nil
	}
}

func decode(data json.RawMessage, v any) error {
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("the data cannot be read: %w", err)
	}
	return nil
}

func invalid(problems []error) error {
	return fmt.Errorf("the data is not valid: %w", errors.Join(problems...))
}

// check reports the properties that the contract requires and that the data
// lacks, or that Notification cannot use.
func (b *booking) check() []error {
	var problems []error
	if b.ReservationID == uuid.Nil {
		problems = append(problems, errors.New("reservationId is missing"))
	}
	if b.RoomID == uuid.Nil {
		problems = append(problems, errors.New("roomId is missing"))
	}
	if b.StartsAt.IsZero() {
		problems = append(problems, errors.New("startsAt is missing"))
	}
	if b.EndsAt.IsZero() {
		problems = append(problems, errors.New("endsAt is missing"))
	}
	address, err := mail.ParseAddress(b.Organizer)
	if err != nil || strings.ContainsAny(b.Organizer, "\r\n") {
		problems = append(problems, fmt.Errorf("organizerEmail %q is not an email address", b.Organizer))
	} else {
		// "Ann <ann@example.com>" becomes "ann@example.com".
		b.Organizer = address.Address
	}
	return problems
}

// The texts of the mails. An event names the room only by its id and carries
// no time zone, so the mail shows the id and the period in UTC.

// instantLayout writes an instant for people, such as
// "Fri, 20 Nov 2026 01:00 UTC".
const instantLayout = "Mon, 2 Jan 2006 15:04 MST"

func instant(t time.Time) string {
	return t.UTC().Format(instantLayout)
}

func confirmedMail(c confirmation) Message {
	return Message{
		To:      c.Organizer,
		Subject: "Reservation confirmed: " + instant(c.StartsAt),
		Body: "Your reservation of a meeting room is confirmed.\n" +
			"If you rescheduled it, this period replaces the one before.\n\n" +
			details(c.booking) +
			fmt.Sprintf("Attendees:   %d\n", c.Attendees),
	}
}

func cancelledMail(c cancellation) Message {
	return Message{
		To:      c.Organizer,
		Subject: "Reservation cancelled: " + instant(c.StartsAt),
		Body: "Your reservation of a meeting room is cancelled.\n" +
			"The room is free again for this period.\n\n" +
			details(c.booking),
	}
}

func details(b booking) string {
	return fmt.Sprintf("Reservation: %s\nRoom:        %s\nFrom:        %s\nUntil:       %s\n",
		b.ReservationID, b.RoomID, instant(b.StartsAt), instant(b.EndsAt))
}
