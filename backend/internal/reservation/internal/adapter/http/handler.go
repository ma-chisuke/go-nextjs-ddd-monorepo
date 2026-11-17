// Package http is the HTTP side of the Reservation context: the code that
// oapi-codegen generates from contracts/http/reservation.openapi.yaml
// (api.gen.go), the operations that implement its strict interface, the
// table from business error to problem, and the ETags of reservations.
package http

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/google/uuid"

	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/httpserver"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/problem"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/reservation/internal/app"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/reservation/internal/domain"
)

// defaultLimit is the page size when a request names none. It is the default
// of the limit parameter in the API document.
const defaultLimit = 20

// retryAfterSeconds is what Retry-After says when Facility does not answer:
// long enough for a restarted task of Facility to come up.
const retryAfterSeconds = 10

// operations implements the operations of the Reservation API over the use
// cases. It translates between the types of the API document and the types
// of the use cases and the domain, and nothing else.
type operations struct {
	reservations *app.Reservations
	logger       *slog.Logger
}

var _ StrictServerInterface = (*operations)(nil)

// Document returns the API document that this package was generated from.
func Document() (*openapi3.T, error) {
	return GetSwagger()
}

// NewHandler returns the HTTP handler of the Reservation API.
func NewHandler(reservations *app.Reservations, logger *slog.Logger) (http.Handler, error) {
	doc, err := Document()
	if err != nil {
		return nil, err
	}
	ops := &operations{reservations: reservations, logger: logger}
	strict := NewStrictHandlerWithOptions(ops, nil, StrictHTTPServerOptions{
		RequestErrorHandlerFunc:  httpserver.RequestError,
		ResponseErrorHandlerFunc: responseError(logger),
	})
	return httpserver.NewHandler(doc, logger, func(mux *http.ServeMux) {
		HandlerWithOptions(strict, StdHTTPServerOptions{
			BaseRouter:       mux,
			ErrorHandlerFunc: httpserver.RequestError,
		})
	})
}

// responseError writes the problem of a failed operation, as every API does.
// When Facility does not answer, it also tells the client when to try again,
// and logs the cause, which the problem does not reveal.
func responseError(logger *slog.Logger) func(http.ResponseWriter, *http.Request, error) {
	write := httpserver.ResponseError(logger)
	return func(w http.ResponseWriter, r *http.Request, err error) {
		var unavailable *app.Error
		if errors.As(err, &unavailable) && unavailable.Kind == app.KindFacilityUnavailable {
			logger.WarnContext(r.Context(), "Facility does not answer", slog.Any("error", unavailable))
			w.Header().Set("Retry-After", strconv.Itoa(retryAfterSeconds))
		}
		write(w, r, toProblem(err))
	}
}

// BookRoom implements the operation bookRoom.
func (h *operations) BookRoom(ctx context.Context, request BookRoomRequestObject) (BookRoomResponseObject, error) {
	body := request.Body
	v, err := h.reservations.BookRoom(ctx, app.BookRoomInput{
		IdempotencyKey: request.Params.IdempotencyKey,
		Room:           domain.RoomID(body.RoomId),
		Period:         domain.Period{Start: body.StartsAt, End: body.EndsAt},
		Attendees:      int(body.Attendees),
		Organizer:      body.OrganizerEmail,
	})
	if err != nil {
		return nil, err
	}
	return BookRoom201JSONResponse{
		Body: reservationBody(v.Reservation),
		Headers: BookRoom201ResponseHeaders{
			ETag:     etag(v.Version),
			Location: "/reservations/" + v.Reservation.ID().String(),
		},
	}, nil
}

// GetReservation implements the operation getReservation.
func (h *operations) GetReservation(ctx context.Context, request GetReservationRequestObject) (GetReservationResponseObject, error) {
	v, err := h.reservations.GetReservation(ctx, domain.ReservationID(request.ReservationId))
	if err != nil {
		return nil, err
	}
	return GetReservation200JSONResponse{
		Body:    reservationBody(v.Reservation),
		Headers: GetReservation200ResponseHeaders{ETag: etag(v.Version)},
	}, nil
}

// ListReservations implements the operation listReservations.
func (h *operations) ListReservations(ctx context.Context, request ListReservationsRequestObject) (ListReservationsResponseObject, error) {
	params := request.Params
	if params.From != nil && params.To != nil && !params.From.Before(*params.To) {
		return nil, problem.Invalid("The window ends before it starts.",
			problem.InvalidParam{Pointer: "to", Detail: "to must be later than from"})
	}
	in := app.ListReservationsInput{From: params.From, To: params.To, Limit: defaultLimit}
	if params.RoomId != nil {
		room := domain.RoomID(*params.RoomId)
		in.Room = &room
	}
	if params.Limit != nil {
		in.Limit = int(*params.Limit)
	}
	if params.Cursor != nil {
		in.Cursor = *params.Cursor
	}
	reservations, next, err := h.reservations.ListReservations(ctx, in)
	if err != nil {
		return nil, err
	}
	page := ReservationPage{Items: make([]Reservation, 0, len(reservations))}
	for _, r := range reservations {
		page.Items = append(page.Items, reservationBody(r))
	}
	if next != "" {
		page.NextCursor = &next
	}
	return ListReservations200JSONResponse(page), nil
}

// RescheduleReservation implements the operation rescheduleReservation.
func (h *operations) RescheduleReservation(ctx context.Context, request RescheduleReservationRequestObject) (RescheduleReservationResponseObject, error) {
	read, err := ifMatch(request.Params.IfMatch)
	if err != nil {
		return nil, err
	}
	body := request.Body
	in := app.RescheduleReservationInput{
		Reservation: domain.ReservationID(request.ReservationId),
		Read:        read,
		Start:       body.StartsAt,
		End:         body.EndsAt,
	}
	if body.Attendees != nil {
		attendees := int(*body.Attendees)
		in.Attendees = &attendees
	}
	v, err := h.reservations.RescheduleReservation(ctx, in)
	if err != nil {
		return nil, err
	}
	return RescheduleReservation200JSONResponse{
		Body:    reservationBody(v.Reservation),
		Headers: RescheduleReservation200ResponseHeaders{ETag: etag(v.Version)},
	}, nil
}

// CancelReservation implements the operation cancelReservation.
func (h *operations) CancelReservation(ctx context.Context, request CancelReservationRequestObject) (CancelReservationResponseObject, error) {
	read, err := ifMatch(request.Params.IfMatch)
	if err != nil {
		return nil, err
	}
	v, err := h.reservations.CancelReservation(ctx, domain.ReservationID(request.ReservationId), read)
	if err != nil {
		return nil, err
	}
	return CancelReservation200JSONResponse{
		Body:    reservationBody(v.Reservation),
		Headers: CancelReservation200ResponseHeaders{ETag: etag(v.Version)},
	}, nil
}

// reservationBody writes a reservation in the words of the API document.
// Instants are written in UTC.
func reservationBody(r domain.Reservation) Reservation {
	return Reservation{
		Id:             uuid.UUID(r.ID()),
		RoomId:         uuid.UUID(r.Room()),
		StartsAt:       r.Period().Start.UTC(),
		EndsAt:         r.Period().End.UTC(),
		Attendees:      int32(r.Attendees()), //nolint:gosec // The API document and the database keep attendees within an int32.
		OrganizerEmail: r.Organizer(),
		Status:         statusBody(r.Status()),
	}
}

// statusBody writes a status in the words of the API document. The switch
// has no default, so that the linter (exhaustive) finds a new status.
func statusBody(s domain.Status) ReservationStatus {
	switch s {
	case domain.StatusConfirmed:
		return Confirmed
	case domain.StatusCancelled:
		return Cancelled
	}
	return ReservationStatus(s.String())
}
