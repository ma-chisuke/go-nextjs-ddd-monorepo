// Package http is the HTTP side of the Facility context: the code that
// oapi-codegen generates from contracts/http/facility.openapi.yaml
// (api.gen.go), the operations that implement its strict interface, and the
// table from business error to problem.
package http

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/facility/internal/app"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/httpserver"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/problem"
)

// defaultLimit is the page size when a request names none. It is the default
// of the limit parameter in the API document.
const defaultLimit = 20

// operations implements the operations of the Facility API over the use cases.
// It translates between the types of the API document and the types of the
// use cases, and nothing else.
type operations struct {
	rooms *app.Rooms
}

var _ StrictServerInterface = (*operations)(nil)

// Document returns the API document that this package was generated from.
func Document() (*openapi3.T, error) {
	return GetSwagger()
}

// NewHandler returns the HTTP handler of the Facility API.
func NewHandler(rooms *app.Rooms, logger *slog.Logger) (http.Handler, error) {
	doc, err := Document()
	if err != nil {
		return nil, err
	}
	strict := NewStrictHandlerWithOptions(&operations{rooms: rooms}, nil, StrictHTTPServerOptions{
		RequestErrorHandlerFunc:  httpserver.RequestError,
		ResponseErrorHandlerFunc: httpserver.ResponseError(logger),
	})
	return httpserver.NewHandler(doc, logger, func(mux *http.ServeMux) {
		HandlerWithOptions(strict, StdHTTPServerOptions{
			BaseRouter:       mux,
			ErrorHandlerFunc: httpserver.RequestError,
		})
	})
}

// RegisterRoom implements the operation registerRoom.
func (h *operations) RegisterRoom(ctx context.Context, request RegisterRoomRequestObject) (RegisterRoomResponseObject, error) {
	body := request.Body
	openFrom, err := app.ParseTimeOfDay(body.OpenFrom)
	if err != nil {
		return nil, problem.Invalid("openFrom is not a time of day.", problem.InvalidParam{Pointer: "/openFrom", Detail: err.Error()})
	}
	openUntil, err := app.ParseTimeOfDay(body.OpenUntil)
	if err != nil {
		return nil, problem.Invalid("openUntil is not a time of day.", problem.InvalidParam{Pointer: "/openUntil", Detail: err.Error()})
	}
	room, err := h.rooms.RegisterRoom(ctx, app.RegisterRoomInput{
		Name:      body.Name,
		Capacity:  body.Capacity,
		OpenFrom:  openFrom,
		OpenUntil: openUntil,
		TimeZone:  body.TimeZone,
	})
	if err != nil {
		return nil, toProblem(err)
	}
	return RegisterRoom201JSONResponse{
		Body:    roomResponse(room),
		Headers: RegisterRoom201ResponseHeaders{Location: "/rooms/" + room.ID.String()},
	}, nil
}

// GetRoom implements the operation getRoom.
func (h *operations) GetRoom(ctx context.Context, request GetRoomRequestObject) (GetRoomResponseObject, error) {
	room, err := h.rooms.GetRoom(ctx, request.RoomId)
	if err != nil {
		return nil, toProblem(err)
	}
	return GetRoom200JSONResponse(roomResponse(room)), nil
}

// ListRooms implements the operation listRooms.
func (h *operations) ListRooms(ctx context.Context, request ListRoomsRequestObject) (ListRoomsResponseObject, error) {
	var limit int32 = defaultLimit
	if request.Params.Limit != nil {
		limit = *request.Params.Limit
	}
	cursor := ""
	if request.Params.Cursor != nil {
		cursor = *request.Params.Cursor
	}
	rooms, next, err := h.rooms.ListRooms(ctx, limit, cursor)
	if err != nil {
		return nil, toProblem(err)
	}
	page := RoomPage{Items: make([]Room, 0, len(rooms))}
	for _, room := range rooms {
		page.Items = append(page.Items, roomResponse(room))
	}
	if next != "" {
		page.NextCursor = &next
	}
	return ListRooms200JSONResponse(page), nil
}

func roomResponse(room app.Room) Room {
	return Room{
		Id:        room.ID,
		Name:      room.Name,
		Capacity:  room.Capacity,
		OpenFrom:  room.OpenFrom.String(),
		OpenUntil: room.OpenUntil.String(),
		TimeZone:  room.TimeZone,
		CreatedAt: room.CreatedAt,
	}
}
