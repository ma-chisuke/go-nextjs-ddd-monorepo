package domain

import (
	"fmt"
	"time"
)

// The numbers of the booking rules.
const (
	// PeriodStep is the grid of the clock that reservations follow: a period
	// starts and ends on a multiple of it, such as 10:00, 10:15, or 10:30.
	PeriodStep = 15 * time.Minute
	// LongestPeriod is the longest time that one reservation holds a room.
	LongestPeriod = 4 * time.Hour
	// CancellationNotice is how long before its start a reservation can
	// still be cancelled, at the latest.
	CancellationNotice = 30 * time.Minute
)

// Period is the time during which a reservation holds a room. It starts at
// Start and ends just before End, so one period can end when the next one
// starts without the two overlapping.
type Period struct {
	Start time.Time
	End   time.Time
}

// Length returns how long the period lasts.
func (p Period) Length() time.Duration {
	return p.End.Sub(p.Start)
}

// Overlaps reports whether p and q share at least one instant.
func (p Period) Overlaps(q Period) bool {
	return p.Start.Before(q.End) && q.Start.Before(p.End)
}

// onStep reports whether t lies on the grid of PeriodStep. Every time zone in
// use today is offset from UTC by a multiple of 15 minutes, so a quarter hour
// in UTC is a quarter hour in the time zone of the room too.
func onStep(t time.Time) bool {
	return t.Truncate(PeriodStep).Equal(t)
}

// describe writes a duration for people: "4 hours", or "75 minutes".
func describe(d time.Duration) string {
	if d%time.Hour == 0 {
		return fmt.Sprintf("%d hours", d/time.Hour)
	}
	return fmt.Sprintf("%d minutes", d/time.Minute)
}
