# Run the services locally

This guide runs Facility and Reservation on your machine against the local PostgreSQL, and books a room through their APIs with curl.
You need Docker, and the tools that `make setup` installs.

## Start the database

```sh
make db-up
```

It starts PostgreSQL 18 with Docker Compose on `127.0.0.1:55432`, runs `db/bootstrap.sql` and `db/local.sql`, and applies the `schema.sql` of each context. Run it again at any time to bring the schemas up to date.
`make db-down` stops PostgreSQL and deletes its data.

## Run the two services

Each command reads its configuration from the environment; the comment at the top of `backend/cmd/<context>/main.go` lists the variables. Both listen on `:8080` by default, so give each a port of its own.
`mise exec --` puts the Go of `mise.toml` on the `PATH`; a shell in which mise is active does not need it.

In one terminal, start Facility as its runtime role:

```sh
cd backend
DATABASE_URL='postgres://facility_runtime:facility_runtime@127.0.0.1:55432/app?sslmode=disable' \
HTTP_ADDR=127.0.0.1:8081 \
mise exec -- go run ./cmd/facility
```

In another terminal, start Reservation, and tell it where Facility is:

```sh
cd backend
DATABASE_URL='postgres://reservation_runtime:reservation_runtime@127.0.0.1:55432/app?sslmode=disable' \
FACILITY_API_URL=http://127.0.0.1:8081 \
HTTP_ADDR=127.0.0.1:8082 \
mise exec -- go run ./cmd/reservation
```

Check: `curl -i http://127.0.0.1:8081/healthz` and `curl -i http://127.0.0.1:8082/healthz` answer `204 No Content`. Each service writes one JSON record per request to its terminal.

## Book a room

1. Register a room in Facility:

   ```sh
   curl -i -X POST http://127.0.0.1:8081/rooms \
     -H 'Content-Type: application/json' \
     -d '{"name":"Aoba","capacity":8,"openFrom":"09:00","openUntil":"18:00","timeZone":"Asia/Tokyo"}'
   ```

   The answer is `201 Created`, with the room in the body and its URL in `Location`. Keep its id:

   ```sh
   room=<the id of the room>
   ```

   If a room named Aoba exists already, the answer is `409 Conflict` with the problem room-name-taken: choose another name, or find the room with `curl http://127.0.0.1:8081/rooms`.

2. Book the room from 10:00 to 11:00 in Tokyo, two days from now:

   ```sh
   day=$(date -d '+2 days' +%F 2> /dev/null || date -v+2d +%F)   # GNU date, or the date of macOS
   curl -i -X POST http://127.0.0.1:8082/reservations \
     -H 'Content-Type: application/json' \
     -H "Idempotency-Key: aoba-$day-10" \
     -d "{\"roomId\":\"$room\",\"startsAt\":\"${day}T10:00:00+09:00\",\"endsAt\":\"${day}T11:00:00+09:00\",\"attendees\":4,\"organizerEmail\":\"organizer@example.com\"}"
   ```

   Reservation asks Facility for the room, applies the rules, and answers `201 Created` with the reservation, its URL in `Location`, and `ETag: "1"`. It writes instants in UTC. Keep the id:

   ```sh
   reservation=<the id of the reservation>
   ```

   The same request sent again, with the same `Idempotency-Key`, gets the same reservation and books nothing new.

3. Reschedule it, naming the version that you read:

   ```sh
   curl -i -X PATCH http://127.0.0.1:8082/reservations/$reservation \
     -H 'Content-Type: application/json' -H 'If-Match: "1"' -d '{"attendees":6}'
   ```

   The answer is `200 OK` with `ETag: "2"`. The same request again gets `412 Precondition Failed`, because the reservation is no longer at version 1.

4. Cancel it:

   ```sh
   curl -i -X POST http://127.0.0.1:8082/reservations/$reservation/cancellation -H 'If-Match: "2"'
   ```

   The answer is `200 OK`, with the status `cancelled` and `ETag: "3"`.

5. Stop Facility with Ctrl+C, and repeat the booking of step 2 with a new `Idempotency-Key`. Reservation answers `503 Service Unavailable` with the problem facility-unavailable and `Retry-After: 10`.

Stop the services with Ctrl+C.

## Run the services in containers

One command builds the image of every service and starts the whole system in Docker Compose: PostgreSQL with every schema, Facility, and Reservation.

```sh
make up
```

- Facility listens on `http://127.0.0.1:58081`, and Reservation on `http://127.0.0.1:58082`. Set `FACILITY_PORT` or `RESERVATION_PORT` to use other ports.
- The services connect to PostgreSQL as their runtime roles, and Reservation reaches Facility over the network of Compose.
- Start the system with `make up`, not with `docker compose up` alone: `make up` applies the schemas first, with the pgschema that mise pins.

The calls of the preceding section work against these ports as well.

`make smoke` starts the services if they are not running, runs one booking across both of them over HTTP, and checks every response against the two API documents. `make check-all` runs it.

```sh
make smoke
```

`make down` stops the services and PostgreSQL, and keeps the data. `make db-down` deletes the data.
