# Facility glossary

The words of the Facility context, and the identifiers that stand for them in the code.
Facility has no domain layer (TPL-0003), so the identifiers belong to its `app` package, `backend/internal/facility/internal/app`.
`make check` fails when an exported type of that package has no row, or when a row names something that the package does not export (`lint-arch`).

| Identifier | Term | Meaning |
|---|---|---|
| `Room` | room | A meeting room that people can book. Facility registers rooms, and other contexts read them through its API. |
| `Room.Name` | name | The name that people use for a room. No two rooms have the same name. |
| `Room.Capacity` | capacity | The largest number of people that a room holds, from 1 to 1000. |
| `Room.OpenFrom` | opening time | The time of day from which a room can be used. |
| `Room.OpenUntil` | closing time | The time of day until which a room can be used. It is later than the opening time, so no room is open across midnight. |
| `Room.TimeZone` | time zone | The IANA time zone, such as `Asia/Tokyo`, in which the opening time and the closing time of a room are read. |
| `TimeOfDay` | time of day | A time on a 24-hour clock, in hours and minutes, without a date. |
| `Rooms` | room use cases | The use cases about rooms: register a room, get one room, and list the rooms. |
| `Rooms.RegisterRoom` | register a room | Add a room, with its name, its capacity, its opening and closing times, and its time zone. |
| `RegisterRoomInput` | room registration | What the registration of a room needs. |
| `Error` | business error | The error of a request that Facility understood, but that a rule of Facility rejects. |
| `Kind` | kind of business error | The rule that rejected a request. The API answers each kind with a problem of its own. |
