# Run the services locally

This guide starts the local system on your machine, books a room in the browser and through the APIs with curl, and shows the mail that Notification sends about the booking.
You need Docker, and the tools that `make setup` installs.

## Start the system

```sh
make up
```

`make up` starts everything in Docker Compose, in this order:

1. PostgreSQL 18, with `db/bootstrap.sql`, `db/local.sql`, and the `schema.sql` of each context applied (`tools/scripts/db.sh up`).
2. AWS Emulator, built from `infra/local/aws-emulator/Dockerfile` (TPL-0017).
3. The event bus `contexts`, and the queue of Notification with its dead-letter queue and its rule, which Terraform creates in the emulator from `infra/local/` (`tools/scripts/aws.sh apply`). The first apply after the emulator starts takes about a minute, because the AWS provider waits about 25 seconds after it creates each queue. An apply without changes takes a few seconds.
4. Mailpit, every process of every context, built from `backend/Dockerfile`, and the frontend, built from `frontend/Dockerfile`. The processes are the APIs of Facility and Reservation, the relay of Reservation (`reservation relay`), and the worker of Notification (`notification`). `make up` waits until each one is healthy.
5. If step 2 started the emulator anew, a restart of the services that call it, so that they read its new CA ([When AWS Emulator restarts](#when-aws-emulator-restarts)).

Start the system with `make up`, not with `docker compose up` alone: the schemas and the AWS resources are applied from the host, with the pgschema and the Terraform that mise pins, and the relay refuses to start without its bus.

Each service listens on the loopback address only:

| Service | Address | Variable for another port |
|---|---|---|
| PostgreSQL | `127.0.0.1:15432` | `POSTGRES_PORT` |
| AWS Emulator | `https://localhost:19297` | `AWS_PORT` |
| Mailpit, its web interface | `http://127.0.0.1:18025` | `MAILPIT_PORT` |
| Facility | `http://127.0.0.1:18081` | `FACILITY_PORT` |
| Reservation | `http://127.0.0.1:18082` | `RESERVATION_PORT` |
| The frontend (`web`) | `http://127.0.0.1:13000` | `WEB_PORT` |

The relay and the worker publish no port. Mailpit accepts SMTP on port 1025 from the other containers only.

Every published port lies below 32768, and a port that you choose with a variable should too. From 32768 up, Linux and macOS hand out ports by themselves, also to outgoing connections. A test that holds a connection to PostgreSQL can then sit on the port that a service is about to publish, and the service fails to start with "address already in use".

Check: `curl -i http://127.0.0.1:18081/healthz` and `curl -i http://127.0.0.1:18082/healthz` answer `204 No Content`, `curl -i http://127.0.0.1:13000/healthz` answers `200 OK`, and `docker compose ps` shows every service as healthy.

## Book a room in the browser

The frontend is the only service that a browser needs. Its server calls the APIs, and the browser talks to it alone (TPL-0014).

1. Open <http://127.0.0.1:13000>. The first screen lists the rooms of Facility.
2. Choose "Register a room", and register a room with a name, a capacity, its opening hours, and a time zone. The page of the new room follows: the room, its reservations on one day in its time zone, and a form that books it.
3. Choose "Next day", fill in the period, the attendees, and the email of the organizer, and choose "Book the room". The page of the reservation follows, with forms that reschedule and cancel it.
4. Reschedule the reservation, then cancel it. Each form sends the version of the reservation that the page shows. A change from a page that is out of date, such as a second tab, gets a notice, and the page shows the reservation as it is now.

The mail of these changes reaches Mailpit, as [See the mail](#see-the-mail) describes for the calls below.

## Book a room through the APIs

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

`mise.toml` pins the command-line tool of AWS, `aws`, for the deployment. Point it at the emulator with the environment that `tools/scripts/aws.sh` prints, in a shell in which mise is active (or put `mise exec --` before each command):

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

2. Restart the services that call the emulator, so that they read the new CA:

   ```sh
   docker compose restart reservation-relay notification
   ```

`make up` does both steps when it starts the emulator anew. After the emulator restarts in any other way, such as `docker compose restart aws`, run them by hand.
Events wait in the outbox meanwhile, so none is lost; the relay publishes them once it can.
`make aws-server` starts the emulator alone, without the other services, and copies its CA. The tests use it: `make test-integration`, `make smoke`, and `make e2e` start the emulator before they run.

`make rehearse-infra`, part of `make check-all`, uses the same emulator for another purpose: it applies the units of stg to it and plans those of prd against them ([Deploy to AWS](deploy.md#without-an-aws-account)). It works in the Region `ap-northeast-1`, while the local system lives in `us-east-1`, and the queues and buses of one Region are apart from those of another, so the rehearsal leaves the local system alone. What it creates stays in the emulator until the emulator restarts.

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

The calls of [Book a room through the APIs](#book-a-room-through-the-apis) work against these ports, `8081` and `8082`, as well. The events of these bookings reach Mailpit too, if the relay and the worker of `make up` run: the relay reads the same outbox.

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

## Run the frontend from its source

To see a change to the frontend without building its image, run the development server of Next.js against the APIs of the local system:

```sh
make frontend-dev
```

It serves the frontend at <http://127.0.0.1:3000>, and reloads a page when its source changes. It calls the APIs at the ports of the table in [Start the system](#start-the-system), or at the ports that `FACILITY_PORT` and `RESERVATION_PORT` name. To call the APIs that you run from their source, set the base URLs: `FACILITY_API_URL=http://127.0.0.1:8081 RESERVATION_API_URL=http://127.0.0.1:8082 make frontend-dev`.

## The test database

`make test-integration` runs the tests behind the build tag `integration` on a database of their own, `app_test`, on the same PostgreSQL. `tools/scripts/db.sh init` creates it with every schema. The local system uses the database `app`, so the relay of the running system never takes an event that a test wrote to an outbox, and a test never meets the rows of the system.

## The smoke test, the end-to-end test, and stopping

`make smoke` starts the services if they are not running, runs one booking across Facility and Reservation over HTTP, checks every response against the two API documents, and waits for the three mails of the booking in Mailpit. `make check-all` runs it.

`make e2e` starts the services in the same way, and runs the scenarios of `frontend/e2e/` in a browser with Playwright: a person registers a room, books it, reschedules, and cancels, and gets the three mails. Three more scenarios send what a form refuses: a period that is booked already, a change from a page that is out of date, and a form that is not filled in. `make check-all` runs it too. The browser is Chromium, in the build that the version of Playwright names. The first run downloads it into `.cache/ms-playwright/`; `make setup` does not.

```sh
make smoke
make e2e
```

Each of the two tests stops the services that it started, and leaves those that ran before, such as the services of `make up`. When both run at once, as in `make check-all`, the second waits for the first.

`make down` stops every container, and keeps the data of PostgreSQL. The state of AWS Emulator and the mail in Mailpit are gone. `make db-down` deletes the data of PostgreSQL.

## Update AWS Emulator

No Renovate rule updates the version of AWS Emulator yet (TPL-0017), so the update is a change by hand to `infra/local/aws-emulator/Dockerfile`:

1. Find the new version in the list that the comment at the top of the Dockerfile names.
2. Change `EMULATOR_VERSION`, and the two checksums `EMULATOR_SHA256_ARM64` and `EMULATOR_SHA256_AMD64`. `curl --location <URL of the package> | shasum -a 256` gives the checksum of each package, the one for `aarch64` and the one for `x86_64`. The URL of a package is in the `RUN` instruction of the Dockerfile.
3. Run `make up`, which builds the image and recreates the container of the emulator, and `make test-integration smoke rehearse-infra`. The integration tests of `platform/awsevents` and of `notification/internal/adapter/ses`, the smoke test, and the rehearsal show whether the new version still does what the template relies on. The build fails when the package does not match its checksum or the signature of AWS.
