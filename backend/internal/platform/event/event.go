// Package event holds the envelope of integration events: the facts that one
// bounded context publishes for the others (TPL-0013). The envelope is
// CloudEvents 1.0 in JSON, and the JSON Schema of each type of event in
// contracts/events/<context>/ defines its data.
//
// A context publishes an event by writing it to its outbox (package outbox),
// and handles the events of another context with a Handler. Neither side
// knows the broker between them.
package event

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// The values that every envelope of this system carries.
const (
	// SpecVersion is the version of CloudEvents.
	SpecVersion = "1.0"
	// DataContentType is the media type of the data of an event.
	DataContentType = "application/json"
)

// Envelope is one integration event: the CloudEvents attributes that this
// system uses, and the data of the event.
type Envelope struct {
	// SpecVersion is always SpecVersion.
	SpecVersion string `json:"specversion"`
	// ID identifies the event: a UUID version 7 (TPL-0012). An event that is
	// delivered twice keeps its id, so a consumer can tell.
	ID uuid.UUID `json:"id"`
	// Source names the context that published the event (Source).
	Source string `json:"source"`
	// Type names the event and the version of its data (Type).
	Type string `json:"type"`
	// Time is when the change that the event reports happened.
	Time time.Time `json:"time"`
	// Subject is the id of the thing that the event is about, such as the
	// reservation.
	Subject string `json:"subject,omitempty"`
	// DataContentType is always DataContentType.
	DataContentType string `json:"datacontenttype"`
	// Data is what happened, as the JSON Schema of Type defines it.
	Data json.RawMessage `json:"data"`
}

// Handler handles one event of another context. It returns nil when the event
// is handled, also when it was handled before or is of no interest: the event
// is then gone. On an error the event comes again later, and after a number of
// attempts the broker sets it aside. Delivery is at least once, so a handler
// must be idempotent.
type Handler func(ctx context.Context, e Envelope) error

// Source returns the source of the events of a context, such as
// "/contexts/reservation".
func Source(contextName string) string {
	return "/contexts/" + contextName
}

// Type returns the type of an event, such as
// "reservation.ReservationConfirmed.v1". The JSON Schema of its data is
// contracts/events/<contextName>/<name>.v<version>.json.
func Type(contextName, name string, version int) string {
	return fmt.Sprintf("%s.%s.v%d", contextName, name, version)
}

// New returns the envelope of an event with data marshaled as JSON.
func New(id uuid.UUID, source, eventType, subject string, at time.Time, data any) (Envelope, error) {
	raw, err := json.Marshal(data)
	if err != nil {
		return Envelope{}, fmt.Errorf("event %s: data: %w", eventType, err)
	}
	e := Envelope{
		SpecVersion:     SpecVersion,
		ID:              id,
		Source:          source,
		Type:            eventType,
		Time:            at.UTC(),
		Subject:         subject,
		DataContentType: DataContentType,
		Data:            raw,
	}
	if err := e.Validate(); err != nil {
		return Envelope{}, err
	}
	return e, nil
}

// Parse reads an envelope from JSON and validates it.
func Parse(raw []byte) (Envelope, error) {
	var e Envelope
	if err := json.Unmarshal(raw, &e); err != nil {
		return Envelope{}, fmt.Errorf("event: not an envelope: %w", err)
	}
	if err := e.Validate(); err != nil {
		return Envelope{}, err
	}
	return e, nil
}

// Validate reports what keeps e from being an envelope of this system. It
// does not look into the data: the JSON Schema of the type does that, in the
// tests of the publisher (eventtest).
func (e Envelope) Validate() error {
	var problems []error
	if e.SpecVersion != SpecVersion {
		problems = append(problems, fmt.Errorf("specversion is %q, want %q", e.SpecVersion, SpecVersion))
	}
	if e.ID == uuid.Nil {
		problems = append(problems, errors.New("id is missing"))
	}
	if e.Source == "" {
		problems = append(problems, errors.New("source is missing"))
	}
	if e.Type == "" {
		problems = append(problems, errors.New("type is missing"))
	}
	if e.Time.IsZero() {
		problems = append(problems, errors.New("time is missing"))
	}
	if e.DataContentType != DataContentType {
		problems = append(problems, fmt.Errorf("datacontenttype is %q, want %q", e.DataContentType, DataContentType))
	}
	if len(e.Data) == 0 || !json.Valid(e.Data) {
		problems = append(problems, errors.New("data is not JSON"))
	}
	if len(problems) > 0 {
		return fmt.Errorf("event: invalid envelope: %w", errors.Join(problems...))
	}
	return nil
}
