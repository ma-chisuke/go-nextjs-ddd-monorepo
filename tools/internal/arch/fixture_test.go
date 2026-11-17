package arch

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// The fixture is a backend with two contexts that keep every rule, in the
// layout of the repository:
//
//   - booking, a core context with a domain layer, and a client of catalog;
//   - catalog, a supporting context: a transaction script whose app imports
//     its own adapter/postgres.
const mod = "example.com/shop/backend"

// Paths of the fixture.
const (
	bookingDomain   = "backend/internal/booking/internal/domain"
	bookingApp      = "backend/internal/booking/internal/app"
	bookingHTTP     = "backend/internal/booking/internal/adapter/http"
	bookingPostgres = "backend/internal/booking/internal/adapter/postgres"
	catalogApp      = "backend/internal/catalog/internal/app"
	catalogHTTP     = "backend/internal/catalog/internal/adapter/http"
	catalogPostgres = "backend/internal/catalog/internal/adapter/postgres"
	platformClock   = "backend/internal/platform/clock"
	bookingGlossary = "docs/contexts/booking/glossary.md"
	bookingJA       = "docs/contexts/booking/glossary.ja.md"
	catalogGlossary = "docs/contexts/catalog/glossary.md"
	catalogJA       = "docs/contexts/catalog/glossary.ja.md"
)

// imp returns the import path of a directory of the fixture.
func imp(dir string) string {
	return mod + strings.TrimPrefix(dir, "backend")
}

func fixtureFiles() map[string]string {
	return map[string]string{
		"backend/go.mod": "module " + mod + "\n\ngo 1.24\n",

		"backend/cmd/booking/main.go": `package main

import (
	"net/http"

	"` + imp("backend/internal/booking") + `"
	"` + imp("backend/internal/platform/httpserver") + `"
)

func main() {
	_ = httpserver.Run(booking.NewHandler(http.DefaultClient))
}
`,
		"backend/cmd/catalog/main.go": `package main

import (
	"` + imp("backend/internal/catalog") + `"
	"` + imp("backend/internal/platform/httpserver") + `"
)

func main() {
	_ = httpserver.Run(catalog.NewHandler())
}
`,

		// Platform: technical packages. The server is the one place, besides
		// generated code, that registers routes.
		"backend/internal/platform/httpserver/server.go": `package httpserver

import "net/http"

// Run serves api, next to a health check.
func Run(api http.Handler) error {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(http.ResponseWriter, *http.Request) {})
	mux.Handle("/", api)
	return http.ListenAndServe(":8080", mux)
}
`,
		platformClock + "/clock.go": `package clock

import "time"

// System reads the clock of the machine.
func System() time.Time {
	return time.Now()
}
`,

		// booking: a core context.
		"backend/internal/booking/module.go": `package booking

import (
	"net/http"
	"time"

	"` + imp(bookingApp) + `"
	"` + imp("backend/internal/booking/internal/adapter/catalog") + `"
	bookinghttp "` + imp(bookingHTTP) + `"
	"` + imp(bookingPostgres) + `"
)

// NewHandler wires the context.
func NewHandler(client *http.Client) http.Handler {
	book := app.NewBook(postgres.New(), catalog.New(client), time.Now)
	return bookinghttp.NewHandler(book)
}
`,
		"backend/internal/booking/module_test.go": `package booking_test

import (
	"testing"

	"` + imp("backend/internal/booking") + `"
)

func TestNewHandler(t *testing.T) {
	if booking.NewHandler(nil) == nil {
		t.Fatal("no handler")
	}
}
`,
		bookingDomain + "/booking.go": `package domain

import (
	"errors"
	"time"

	"` + imp(bookingDomain+"/period") + `"
)

// Booking is a room, booked for a period.
type Booking struct {
	Room   string
	Period period.Period
}

// Status is where a booking stands.
type Status int

// ErrPast rejects a booking that starts before now.
var ErrPast = errors.New("the booking starts in the past")

// New books a room. now is the current time, which the use case passes in.
func New(room string, p period.Period, now time.Time) (Booking, error) {
	if p.Start.Before(now) {
		return Booking{}, ErrPast
	}
	return Booking{Room: room, Period: p}, nil
}

// Expires reports when an unconfirmed booking lapses.
func (b Booking) Expires(c interface{ Now() time.Time }, wait time.Duration) time.Time {
	return c.Now().Add(wait)
}
`,
		bookingDomain + "/booking_test.go": `package domain

import (
	"testing"
	"time"

	"` + imp(bookingDomain+"/period") + `"
)

func TestNew(t *testing.T) {
	if _, err := New("a", period.Period{}, time.Now()); err == nil {
		t.Fatal("no error")
	}
}
`,
		bookingDomain + "/period/period.go": `package period

import "time"

// Period is a span of time.
type Period struct {
	Start, End time.Time
}
`,
		bookingApp + "/book.go": `package app

import (
	"context"
	"time"

	"` + imp(bookingDomain) + `"
	"` + imp(bookingDomain+"/period") + `"
)

// Repository stores bookings.
type Repository interface {
	Save(ctx context.Context, b domain.Booking) error
}

// Catalog answers questions about rooms.
type Catalog interface {
	Capacity(ctx context.Context, room string) (int, error)
}

// Book is the use case that books a room.
type Book struct {
	repo    Repository
	catalog Catalog
	now     func() time.Time
}

// NewBook returns the use case.
func NewBook(repo Repository, catalog Catalog, now func() time.Time) *Book {
	return &Book{repo: repo, catalog: catalog, now: now}
}

// Do books a room.
func (b *Book) Do(ctx context.Context, room string, p period.Period) error {
	booking, err := domain.New(room, p, b.now())
	if err != nil {
		return err
	}
	return b.repo.Save(ctx, booking)
}
`,
		bookingHTTP + "/api.gen.go": `// Code generated by oapi-codegen. DO NOT EDIT.

package http

import "net/http"

// HandlerFromMux registers the operations on m.
func HandlerFromMux(m *http.ServeMux) {
	m.HandleFunc("POST /bookings", nil)
}
`,
		bookingHTTP + "/handler.go": `package http

import (
	"net/http"

	"` + imp(bookingApp) + `"
)

// NewHandler returns the API.
func NewHandler(book *app.Book) http.Handler {
	mux := http.NewServeMux()
	HandlerFromMux(mux)
	_ = book
	return mux
}
`,
		bookingHTTP + "/handler_test.go": `package http_test

import (
	"net/http"
	"testing"

	"` + imp(bookingApp) + `"
	bookinghttp "` + imp(bookingHTTP) + `"
	"` + imp(bookingPostgres+"/postgrestest") + `"
)

func TestHandler(t *testing.T) {
	fake := http.NewServeMux()
	fake.HandleFunc("GET /rooms/{id}", func(http.ResponseWriter, *http.Request) {})
	_ = bookinghttp.NewHandler(app.NewBook(postgrestest.New(), nil, nil))
}
`,
		bookingPostgres + "/repo.go": `package postgres

import (
	"context"

	"` + imp(bookingDomain) + `"
)

// Repo stores bookings in PostgreSQL.
type Repo struct{}

// New returns a repository.
func New() *Repo { return &Repo{} }

// Save stores a booking.
func (r *Repo) Save(context.Context, domain.Booking) error { return nil }
`,
		bookingPostgres + "/postgrestest/fake.go": `package postgrestest

import "` + imp(bookingPostgres) + `"

// New returns a repository for tests.
func New() *postgres.Repo { return postgres.New() }
`,
		bookingPostgres + "/schema.sql": `-- The schema of booking. A room is an id of catalog.rooms, but no key
-- refers to it: /* catalog.rooms */ belongs to another context.
CREATE TABLE bookings (
	id uuid NOT NULL,
	room_id uuid NOT NULL, /* nested /* catalog.rooms */ comment */
	note text DEFAULT 'see catalog.rooms' NOT NULL,
	CONSTRAINT bookings_pkey PRIMARY KEY (id)
);

COMMENT ON COLUMN bookings.room_id IS 'The id of a room in catalog.rooms.';

CREATE FUNCTION touch() RETURNS trigger LANGUAGE plpgsql AS $body$
BEGIN
	PERFORM 1 FROM catalog.rooms;
END
$body$;
`,
		bookingPostgres + "/queries/bookings.sql": `-- name: ListBookings :many
SELECT b.id, b.room_id
FROM bookings AS b
WHERE b.room_id = $1 AND b.note <> E'it\'s catalog.rooms';
`,
		bookingPostgres + "/testdata/denied.sql": `-- The runtime role of booking cannot read this.
SELECT id FROM catalog.rooms;
`,
		"backend/internal/booking/internal/adapter/catalog/client.go": `package catalog

import (
	"context"
	"net/http"
)

// Client asks catalog over HTTP.
type Client struct {
	http *http.Client
}

// New returns a client.
func New(c *http.Client) *Client { return &Client{http: c} }

// Capacity returns the capacity of a room.
func (c *Client) Capacity(context.Context, string) (int, error) { return 0, nil }
`,

		// catalog: a supporting context.
		"backend/internal/catalog/module.go": `package catalog

import (
	"net/http"

	"` + imp(catalogApp) + `"
	cataloghttp "` + imp(catalogHTTP) + `"
	"` + imp(catalogPostgres) + `"
)

// NewHandler wires the context.
func NewHandler() http.Handler {
	return cataloghttp.NewHandler(app.NewRooms(postgres.Queries{}))
}
`,
		catalogApp + "/rooms.go": `package app

import (
	"time"

	"` + imp(catalogPostgres) + `"
)

// Room is a room that people can book.
type Room struct {
	Name     string
	Capacity int
	created  time.Time
}

// MaxCapacity is the largest capacity of a room.
const MaxCapacity = 1000

// Rooms holds the use cases about rooms.
type Rooms struct {
	queries postgres.Queries
}

// NewRooms returns the use cases.
func NewRooms(q postgres.Queries) *Rooms { return &Rooms{queries: q} }

// Register registers a room.
func (r *Rooms) Register(name string) Room { return Room{Name: name, created: time.Now()} }

type cursor struct{}
`,
		catalogApp + "/rooms_integration_test.go": `//go:build integration

package app

import "testing"

// Fixture is a type of a test file, which the glossary leaves out.
type Fixture struct{}

func TestRegister(t *testing.T) {}
`,
		catalogHTTP + "/handler.go": `package http

import (
	"net/http"

	"` + imp(catalogApp) + `"
)

// NewHandler returns the API.
func NewHandler(rooms *app.Rooms) http.Handler {
	_ = rooms
	return http.NewServeMux()
}
`,
		catalogPostgres + "/queries.go": `package postgres

// Queries runs the queries of catalog.
type Queries struct{}
`,
		catalogPostgres + "/schema.sql": `CREATE TABLE IF NOT EXISTS rooms (
	id uuid NOT NULL,
	name text NOT NULL
);

COMMENT ON COLUMN rooms.id IS 'The id of the room.';
`,
		catalogPostgres + "/queries/rooms.sql": `-- name: GetRoom :one
SELECT rooms.id, rooms.name FROM rooms WHERE rooms.id = sqlc.arg(id);
`,

		bookingGlossary: "# Booking glossary\n\n" +
			"The words of the booking context.\n\n" +
			"| Identifier | Term | Meaning |\n" +
			"|---|---|---|\n" +
			"| `Booking` | booking | A room, booked for a period. |\n" +
			"| `Booking.Room` | room | The room that a booking holds. |\n" +
			"| `Status` | status | Where a booking stands. |\n" +
			"| `ErrPast` | booking in the past | A booking that starts before now. |\n" +
			"| `New` | book | Book a room \\| or try to. |\n",
		bookingJA: "# booking の用語集\n\n" +
			"| 識別子 | 用語 | 意味 |\n" +
			"|---|---|---|\n" +
			"| `Booking` | 予約 | 期間を決めて押さえた部屋。 |\n" +
			"| `Booking.Room` | 部屋 | 予約が押さえる部屋。 |\n" +
			"| `Status` | 状態 | 予約の状態。 |\n" +
			"| `ErrPast` | 過去の予約 | 今より前に始まる予約。 |\n" +
			"| `New` | 予約する | 部屋を予約する。 |\n" +
			"\n<!-- i18n-source: glossary.md sha256:0000 -->\n",
		catalogGlossary: "# Catalog glossary\n\n" +
			"```markdown\n| Not | The | Table |\n```\n\n" +
			"| Identifier | Term | Meaning |\n" +
			"| :--- | --- | ---: |\n" +
			"| `Room` | room | A room that people can book. |\n" +
			"| `Room.Capacity` | capacity | How many people a room holds. |\n" +
			"| `Rooms` | rooms | The use cases about rooms. |\n" +
			"| `Rooms.Register` | register | Add a room. |\n" +
			"| `MaxCapacity` | largest capacity | The most people any room holds. |\n",
		catalogJA: "# catalog の用語集\n\n" +
			"| 識別子 | 用語 | 意味 |\n" +
			"|---|---|---|\n" +
			"| `Rooms.Register` | 登録する | 部屋を加える。 |\n" +
			"| `Room` | 部屋 | 予約できる部屋。 |\n" +
			"| `Room.Capacity` | 定員 | 部屋に入れる人数。 |\n" +
			"| `Rooms` | 部屋のユースケース | 部屋についてのユースケース。 |\n" +
			"| `MaxCapacity` | 最大の定員 | どの部屋にも入れる人数の上限。 |\n",
	}
}

// writeFile writes one file under dir, creating its directory.
func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	full := filepath.Join(dir, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// readFile reads one file under dir.
func readFile(t *testing.T, dir, name string) string {
	t.Helper()
	data, err := fs.ReadFile(os.DirFS(dir), name)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// newRepo writes the fixture into a new directory.
func newRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range fixtureFiles() {
		writeFile(t, dir, name, content)
	}
	return dir
}

// check runs Check on dir and returns "path:line: [rule]" of each finding,
// sorted.
func check(t *testing.T, dir string) []string {
	t.Helper()
	findings, err := Check(t.Context(), dir)
	if err != nil {
		t.Fatalf("Check() error: %v", err)
	}
	got := make([]string, 0, len(findings))
	for _, f := range findings {
		s := f.String()
		got = append(got, s[:strings.Index(s, "]")+1])
	}
	slices.Sort(got)
	return got
}

// change edits the fixture of one test case.
type change func(t *testing.T, dir string)

// put writes a file.
func put(name, content string) change {
	return func(t *testing.T, dir string) {
		t.Helper()
		writeFile(t, dir, name, content)
	}
}

// remove deletes a file.
func remove(name string) change {
	return func(t *testing.T, dir string) {
		t.Helper()
		if err := os.Remove(filepath.Join(dir, filepath.FromSlash(name))); err != nil {
			t.Fatal(err)
		}
	}
}

// edit replaces the first old with replacement in a file, and fails when the
// file does not contain old.
func edit(name, old, replacement string) change {
	return func(t *testing.T, dir string) {
		t.Helper()
		text := readFile(t, dir, name)
		if !strings.Contains(text, old) {
			t.Fatalf("%s does not contain %q", name, old)
		}
		writeFile(t, dir, name, strings.Replace(text, old, replacement, 1))
	}
}

// all applies several changes in order.
func all(changes ...change) change {
	return func(t *testing.T, dir string) {
		t.Helper()
		for _, c := range changes {
			c(t, dir)
		}
	}
}

// goFileImporting returns the source of a Go file of package pkg that imports
// the given paths, for their side effects.
func goFileImporting(pkg string, paths ...string) string {
	var b strings.Builder
	b.WriteString("package " + pkg + "\n\nimport (\n")
	for _, p := range paths {
		b.WriteString("\t_ \"" + p + "\"\n")
	}
	b.WriteString(")\n")
	return b.String()
}

// testCase is one change to the fixture and the findings that it must give.
type testCase struct {
	name   string
	change change
	want   []string
}

// runCases checks each case on a fresh copy of the fixture, in parallel: each
// one runs the go command.
func runCases(t *testing.T, tests []testCase) {
	t.Helper()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir := newRepo(t)
			if tt.change != nil {
				tt.change(t, dir)
			}
			if got := check(t, dir); !slices.Equal(got, tt.want) {
				t.Errorf("findings:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(tt.want, "\n"))
			}
		})
	}
}
