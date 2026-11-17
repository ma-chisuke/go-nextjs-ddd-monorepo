# Glossary of Reservation

The words of the business that the domain model of the Reservation context uses.
Each row names an exported type of the package `backend/internal/reservation/internal/domain`, the business term that it stands for, and what the term means.

| Identifier | Term | Meaning |
|---|---|---|
| `Reservation` | reservation | The booking of a meeting room for a period, for a number of attendees, made by an organizer. It is confirmed or cancelled. |
| `ReservationID` | reservation id | What identifies a reservation: a UUID version 7, made when the room is booked. |
| `RoomID` | room id | What identifies a meeting room. The Facility context gives rooms their ids. |
| `Room` | room | A meeting room as far as a reservation needs to know it: its capacity, its opening hours, and its time zone. |
| `OpeningHours` | opening hours | The times of day between which a room can be used, every day, read in the time zone of the room. |
| `TimeOfDay` | time of day | A time on a 24-hour clock, such as 09:00. |
| `Period` | period | The time during which a reservation holds a room. It includes its start and excludes its end, so one reservation may end when the next starts. |
| `Status` | status | Whether a reservation holds its room: confirmed or cancelled. |
| `Event` | domain event | Something that happened to a reservation, which other parts of the business may act on. |
| `ReservationConfirmed` | reservation confirmed | A reservation holds its room for its period, with its attendees: the room was booked, or the reservation was rescheduled. |
| `ReservationCancelled` | reservation cancelled | A reservation was cancelled, and its room is free for its period. |
| `Kind` | kind of business error | Which rule of reservations a request breaks. |
| `Error` | business error | A rule of reservations rejects the request. |
