# Run the services locally

This guide starts the local system on your machine, books a room through the APIs with curl, and shows the mail that Notification sends about the booking.
You need Docker, and the tools that `make setup` installs.

## Start the system

```sh
make up
```

`make up` starts everything in Docker Compose, in this order:

1. PostgreSQL 18, with `db/bootstrap.sql`, `db/local.sql`, and the `schema.sql` of each context applied (`tools/scripts/db.sh up`).
2. AWS Emulator, built from `infra/local/aws-emulator/Dockerfile` (TPL-0017).
3. The event bus `contexts`, and the queue of Notification with its dead-letter queue and its rule, which Terraform creates in the emulator from `infra/local/` (`tools/scripts/aws.sh apply`). The first apply after the emulator starts takes about a minute, because the AWS provider waits about 25 seconds after it creates each queue. An apply without changes takes a few seconds.
4. Mailpit, and every process of every context, built from `backend/Dockerfile`: the APIs of Facility and Reservation, the relay of Reservation (`reservation relay`), and the worker of Notification (`notification`). `make up` waits until each one is healthy.

Start the system with `make up`, not with `docker compose up` alone: the schemas and the AWS resources are applied from the host, with the pgschema and the Terraform that mise pins, and the relay refuses to start without its bus.

Each service listens on the loopback address only:

| Service | Address | Variable for another port |
|---|---|---|
| PostgreSQL | `127.0.0.1:15432` | `POSTGRES_PORT` |
| AWS Emulator | `https://localhost:19297` | `AWS_PORT` |
| Mailpit, its web interface | `http://127.0.0.1:18025` | `MAILPIT_PORT` |
| Facility | `http://127.0.0.1:18081` | `FACILITY_PORT` |
| Reservation | `http://127.0.0.1:18082` | `RESERVATION_PORT` |

The relay and the worker publish no port. Mailpit accepts SMTP on port 1025 from the other containers only.

Every published port lies below 32768, and a port that you choose with a variable should too. From 32768 up, Linux and macOS hand out ports by themselves, also to outgoing connections. A test that holds a connection to PostgreSQL can then sit on the port that a service is about to publish, and the service fails to start with "address already in use".

Check: `curl -i http://127.0.0.1:18081/healthz` and `curl -i http://127.0.0.1:18082/healthz` answer `204 No Content`, and `docker compose ps` shows every service as healthy.

## Book a room

1. Register a room in Facility:

   ```sh
   curl -i -X POST http://127.0.0.1:18081/rooms \
     -H 'Content-Type: application/json' \
     -d '{"name":"Aoba","capacity":8,"openFrom":"09:00","openUntil":"18:00","timeZone":"Asia/Tokyo"}'
   ```

   The answer is `201 Created`, with the room in the body and its URL in `Location`. Keep its id:

   ```sh
   room=<the id of the room>
   ```

   If a room named Aoba exists already, the answer is `409 Conflict` with the problem room-name-taken: choose another name, or find the room with `curl http://127.0.0.1:18081/rooms`.

2. Book the room from 10:00 to 11:00 in Tokyo, two days from now:

   ```sh
   day=$(date -d '+2 days' +%F 2> /dev/null || date -v+2d +%F)   # GNU date, or the date of macOS
   curl -i -X POST http://127.0.0.1:18082/reservations \
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
   curl -i -X PATCH http://127.0.0.1:18082/reservations/$reservation \
     -H 'Content-Type: application/json' -H 'If-Match: "1"' -d '{"attendees":6}'
   ```

   The answer is `200 OK` with `ETag: "2"`. The same request again gets `412 Precondition Failed`, because the reservation is no longer at version 1.

4. Cancel it:

   ```sh
   curl -i -X POST http://127.0.0.1:18082/reservations/$reservation/cancellation -H 'If-Match: "2"'
   ```

   The answer is `200 OK`, with the status `cancelled` and `ETag: "3"`.

5. Stop Facility, and repeat the booking of step 2 with a new `Idempotency-Key`. Reservation answers `503 Service Unavailable` with the problem facility-unavailable and `Retry-After: 10`. Then start Facility again.

   ```sh
   docker compose stop facility
   # the booking of step 2, with another Idempotency-Key
   docker compose start facility
   ```

## See the mail

The booking, the rescheduling, and the cancellation each wrote an event to the outbox of Reservation. Within a few seconds, the relay publishes each to the bus, the rule of Notification sends it to its queue, and the worker sends a mail to Mailpit.

Open <http://127.0.0.1:18025>. It holds three mails to `organizer@example.com`: two whose subject starts with `Reservation confirmed`, for the booking and for the rescheduling, and one that starts with `Reservation cancelled`. The repeated booking of step 2 and the refused requests sent none.

Each process writes one JSON record per line to its log. To follow an event, read the logs of the relay and the worker; each record names the `event_id`:

```sh
docker compose logs reservation-relay notification
```

Mailpit keeps the mail on a temporary file system, so the mail is gone when its container stops.

## Look at the queues

The template pins no AWS CLI. If you have one installed, point it at the emulator with the environment that `tools/scripts/aws.sh` prints:

```sh
eval "$(tools/scripts/aws.sh env)"
aws sqs list-queues
aws sqs get-queue-attributes --attribute-names ApproximateNumberOfMessages \
  --queue-url https://sqs.us-east-1.amazonaws.com/000000000000/notification-events-dlq
aws events list-rules --event-bus-name contexts
```

The environment sets `AWS_ENDPOINT_URL` to the port of the emulator, `AWS_CA_BUNDLE` to the CA that `make up` copied to `.cache/aws-emulator/ca.pem`, and placeholder credentials: the emulator checks no signature. The emulator names its queues as AWS does, with the account `000000000000`, and answers for those URLs on its own endpoint.

A message that the worker could not handle five times waits in `notification-events-dlq`, and the log of the worker names the error of each attempt.

## When AWS Emulator restarts

AWS Emulator keeps its state in memory, and makes a new CA at every start. After its container restarts, the bus and the queues are gone, and the relay and the worker no longer trust its certificate: they log errors about the certificate or a missing queue, and try again.

1. Create the bus and the queues again, and copy the new CA to `.cache/aws-emulator/ca.pem`:

   ```sh
   make aws-apply
   ```

   `make up` does this too, but it does not restart the services that run already.
2. Restart the services that call the emulator, so that they read the new CA:

   ```sh
   docker compose restart reservation-relay notification
   ```

Events wait in the outbox meanwhile, so none is lost; the relay publishes them once it can.
`make aws-server` starts the emulator alone, without the other services, and copies its CA. The integration tests use it: `make test-integration` and `make smoke` start the emulator before they run.

## Run an API from its source

To run an API with your own build, start it with `go run` against the local PostgreSQL. Each command reads its configuration from the environment; the comment at the top of `backend/cmd/<context>/main.go` lists the variables. Both APIs listen on `:8080` by default, so give each a port of its own.
`mise exec --` puts the Go of `mise.toml` on the `PATH`; a shell in which mise is active does not need it.

`make db-up` starts PostgreSQL alone and applies the schemas, if `make up` has not. In a Git worktree, `make db-server` starts it only if it does not run, and leaves a container that another checkout started alone.

In one terminal, start Facility as its runtime role:

```sh
cd backend
DATABASE_URL='postgres://facility_runtime:facility_runtime@127.0.0.1:15432/app?sslmode=disable' \
HTTP_ADDR=127.0.0.1:8081 \
mise exec -- go run ./cmd/facility
```

In another terminal, start Reservation, and tell it where Facility is:

```sh
cd backend
DATABASE_URL='postgres://reservation_runtime:reservation_runtime@127.0.0.1:15432/app?sslmode=disable' \
FACILITY_API_URL=http://127.0.0.1:8081 \
HTTP_ADDR=127.0.0.1:8082 \
mise exec -- go run ./cmd/reservation
```

The calls of [Book a room](#book-a-room) work against these ports, `8081` and `8082`, as well. The events of these bookings reach Mailpit too, if the relay and the worker of `make up` run: the relay reads the same outbox.

To run the relay from its source, point it at the emulator with the environment of `tools/scripts/aws.sh`:

```sh
make aws-server aws-apply
eval "$(tools/scripts/aws.sh env)"
cd backend
DATABASE_URL='postgres://reservation_runtime:reservation_runtime@127.0.0.1:15432/app?sslmode=disable' \
EVENT_BUS_NAME=contexts \
HTTP_ADDR=127.0.0.1:8083 \
mise exec -- go run ./cmd/reservation relay
```

Several relays can run on one outbox, so this one and the one of `make up` share the events. The worker of Notification needs Mailpit over SMTP, which only the containers reach, so run it with `make up`.

## The test database

`make test-integration` runs the tests behind the build tag `integration` on a database of their own, `app_test`, on the same PostgreSQL. `tools/scripts/db.sh init` creates it with every schema. The local system uses the database `app`, so the relay of the running system never takes an event that a test wrote to an outbox, and a test never meets the rows of the system.

## The smoke test, and stopping

`make smoke` starts the services if they are not running, runs one booking across Facility and Reservation over HTTP, checks every response against the two API documents, and waits for the three mails of the booking in Mailpit. `make check-all` runs it.

```sh
make smoke
```

`make down` stops every container, and keeps the data of PostgreSQL. The state of AWS Emulator and the mail in Mailpit are gone. `make db-down` deletes the data of PostgreSQL.

## Update AWS Emulator

No Renovate rule updates the version of AWS Emulator yet (TPL-0017), so the update is a change by hand to `infra/local/aws-emulator/Dockerfile`:

1. Find the new version in the list that the comment at the top of the Dockerfile names.
2. Change `EMULATOR_VERSION`, and the two checksums `EMULATOR_SHA256_ARM64` and `EMULATOR_SHA256_AMD64`. `curl --location <URL of the package> | shasum -a 256` gives the checksum of each package, the one for `aarch64` and the one for `x86_64`. The URL of a package is in the `RUN` instruction of the Dockerfile.
3. Run `make up`, which builds the image and recreates the container of the emulator, and `CHECK_SCOPE=changed make check-all`. The integration tests of `platform/awsevents` and `make smoke` show whether the new version still does what the template relies on. The build fails when the package does not match its checksum or the signature of AWS.
