// Package domain is the model of the Reservation context: the reservation of
// a meeting room, the rules that a reservation follows, and what happens when
// it is booked, rescheduled, or cancelled.
//
// The package does no I/O (TPL-0004). It reads no clock and makes no ids: the
// current time and the id of a new reservation are arguments. It knows a room
// only as far as a reservation needs it (Room), in the words of this context,
// not in those of the Facility context that owns rooms.
package domain

import "encoding/hex"

// ReservationID identifies a reservation. It is a UUID version 7, made by the
// use case before the reservation exists (TPL-0012).
type ReservationID [16]byte

// String writes the id in the text form of a UUID.
func (id ReservationID) String() string {
	return uuidText(id)
}

// RoomID identifies a meeting room. The Facility context gives rooms their
// ids, and Reservation uses them as they are.
type RoomID [16]byte

// String writes the id in the text form of a UUID.
func (id RoomID) String() string {
	return uuidText(id)
}

// uuidText writes 16 bytes as xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx (RFC 9562).
func uuidText(b [16]byte) string {
	var s [36]byte
	hex.Encode(s[0:8], b[0:4])
	s[8] = '-'
	hex.Encode(s[9:13], b[4:6])
	s[13] = '-'
	hex.Encode(s[14:18], b[6:8])
	s[18] = '-'
	hex.Encode(s[19:23], b[8:10])
	s[23] = '-'
	hex.Encode(s[24:36], b[10:16])
	return string(s[:])
}
