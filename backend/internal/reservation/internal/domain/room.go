package domain

import (
	"fmt"
	"time"
)

// Room is a meeting room as far as a reservation needs to know it: how many
// people it holds, and when it can be used. The Facility context owns rooms;
// an adapter translates its answer into this value.
type Room struct {
	// ID identifies the room.
	ID RoomID
	// Capacity is the largest number of people that the room holds.
	Capacity int32
	// Hours are the times of day between which the room can be used, read
	// in TimeZone.
	Hours OpeningHours
	// TimeZone is where the room is. Its clock reads the opening hours, and
	// it decides on which day a period falls. Nil means UTC.
	TimeZone *time.Location
}

// OpeningHours are the times of day between which a room can be used, every
// day. A room opens and closes on the same day.
type OpeningHours struct {
	Opens  TimeOfDay
	Closes TimeOfDay
}

// TimeOfDay is a time on a 24-hour clock, in minutes after midnight.
type TimeOfDay int

// NewTimeOfDay returns the time of day hour:minute.
func NewTimeOfDay(hour, minute int) TimeOfDay {
	return TimeOfDay(hour*60 + minute)
}

// Hour returns the hour of t.
func (t TimeOfDay) Hour() int {
	return int(t) / 60
}

// Minute returns the minute of t within its hour.
func (t TimeOfDay) Minute() int {
	return int(t) % 60
}

// String writes t as HH:MM.
func (t TimeOfDay) String() string {
	return fmt.Sprintf("%02d:%02d", t.Hour(), t.Minute())
}

// location returns the time zone of the room.
func (r Room) location() *time.Location {
	if r.TimeZone == nil {
		return time.UTC
	}
	return r.TimeZone
}

// isOpenDuring reports whether the room is open for the whole period. The
// opening hours are those of the day on which the period starts, read in the
// time zone of the room. The room closes on that day, so a period that runs
// into the next day is never inside them.
func (r Room) isOpenDuring(p Period) bool {
	return !p.Start.Before(r.at(p.Start, r.Hours.Opens)) && !p.End.After(r.at(p.Start, r.Hours.Closes))
}

// at returns the instant at which the clock of the room shows t, on the day
// on which it shows day.
func (r Room) at(day time.Time, t TimeOfDay) time.Time {
	y, m, d := day.In(r.location()).Date()
	return time.Date(y, m, d, t.Hour(), t.Minute(), 0, 0, r.location())
}

// describeHours writes the opening hours and the time zone, for the detail of
// an error.
func (r Room) describeHours() string {
	return fmt.Sprintf("%s to %s in %s", r.Hours.Opens, r.Hours.Closes, r.location())
}
