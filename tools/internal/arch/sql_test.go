package arch

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

func TestCheckSQL(t *testing.T) {
	const (
		bookingQueries = bookingPostgres + "/queries/rooms.sql"
		bookingSchema  = bookingPostgres + "/schema.sql"
		catalogSchema  = catalogPostgres + "/schema.sql"
		catalogQueries = catalogPostgres + "/queries/more.sql"
	)
	runCases(t, []testCase{
		{
			name:   "schema-reference fires on a query of another context",
			change: put(bookingQueries, "-- name: ListRooms :many\nSELECT r.id\nFROM catalog.rooms AS r;\n"),
			want:   []string{bookingQueries + ":3: [arch/schema-reference]"},
		},
		{
			name:   "schema-reference fires on a foreign key to another context",
			change: edit(bookingSchema, "room_id uuid NOT NULL,", "room_id uuid NOT NULL REFERENCES catalog.rooms (id),"),
			want:   []string{bookingSchema + ":5: [arch/schema-reference]"},
		},
		{
			name:   "schema-reference fires on a quoted schema",
			change: put(bookingQueries, "SELECT id FROM \"catalog\".\"rooms\";\n"),
			want:   []string{bookingQueries + ":1: [arch/schema-reference]"},
		},
		{
			name:   "schema-reference fires on a name in three parts",
			change: put(bookingQueries, "SELECT id\nFROM shop . catalog . rooms;\n"),
			want:   []string{bookingQueries + ":2: [arch/schema-reference]"},
		},
		{
			name:   "schema-reference fires on its own schema in schema.sql",
			change: edit(catalogSchema, "CREATE TABLE IF NOT EXISTS rooms (", "CREATE TABLE IF NOT EXISTS catalog.rooms ("),
			want:   []string{catalogSchema + ":1: [arch/schema-reference]"},
		},
		{
			name:   "schema-reference fires on its own schema in upper case in a query",
			change: put(catalogQueries, "SELECT id\n  FROM CATALOG.rooms;\n"),
			want:   []string{catalogQueries + ":2: [arch/schema-reference]"},
		},
		{
			name:   "schema-reference fires on its own schema in any SQL file of the context",
			change: put(catalogPostgres+"/testdata/seed.sql", "INSERT INTO catalog.rooms (id) VALUES ('a');\n"),
			want:   []string{catalogPostgres + "/testdata/seed.sql:1: [arch/schema-reference]"},
		},
		{
			name:   "schema-reference passes a quoted name that differs in case",
			change: put(catalogQueries, "SELECT id FROM \"Catalog\".rooms;\n"),
		},
		{
			name: "schema-reference passes a table with the name of a context",
			change: all(
				edit(catalogSchema, "COMMENT ON", "CREATE TABLE booking (room_id uuid);\n\nCOMMENT ON"),
				put(catalogQueries, "SELECT booking.room_id FROM booking;\n"),
			),
		},
		{
			name:   "schema-reference passes an alias with the name of a context",
			change: put(bookingQueries, "SELECT catalog.id FROM bookings AS catalog;\n"),
		},
		{
			name:   "schema-reference passes a common table expression with the name of a context",
			change: put(bookingQueries, "WITH catalog AS (SELECT 1 AS id)\nSELECT catalog.id FROM catalog;\n"),
		},
		{
			name: "schema-reference passes words that only look like names",
			change: put(bookingQueries, "SELECT 1.5, $1, $$catalog.rooms$$, $a$ catalog.rooms $a$, U&'catalog.rooms', "+
				"B'1', X'1F', N'catalog.rooms', \"a \"\"b\"\" c\" FROM bookings; -- catalog.rooms\n"),
		},
		{
			name:   "schema-reference fires after an unterminated name, which runs to the end",
			change: put(bookingQueries, "SELECT catalog.rooms FROM \"bookings;\n"),
			want:   []string{bookingQueries + ":1: [arch/schema-reference]"},
		},
	})
}

// render writes tokens as "text@line" for the test of the scanner.
func render(tokens []sqlToken) string {
	parts := make([]string, 0, len(tokens))
	for _, t := range tokens {
		parts = append(parts, fmt.Sprintf("%s@%d", t.text, t.line))
	}
	return strings.Join(parts, " ")
}

func TestScanSQL(t *testing.T) {
	tests := []struct {
		name, src, want string
	}{
		{name: "names, dots, and symbols", src: "SELECT a.b, c.* FROM t;", want: "SELECT@1 a@1 .@1 b@1 ,@1 c@1 .@1 *@1 FROM@1 t@1 ;@1"},
		{name: "a line comment", src: "a -- b.c\nd", want: "a@1 d@2"},
		{name: "nested block comments", src: "a /* b /* c */ d.e */ f", want: "a@1 f@1"},
		{name: "an unterminated block comment", src: "a /* b", want: "a@1"},
		{name: "lines inside a block comment", src: "a /*\n\n*/ b", want: "a@1 b@3"},
		{name: "a string with a doubled quote", src: "'it''s a.b' c", want: "c@1"},
		{name: "an escape string", src: `E'it\'s a.b' c`, want: "c@1"},
		{name: "an unterminated string", src: "a 'b.c", want: "a@1"},
		{name: "a string over two lines", src: "'a\nb' c", want: "c@2"},
		{name: "a dollar-quoted string", src: "$$ a.b $$ c", want: "c@1"},
		{name: "a tagged dollar-quoted string", src: "$x$ a $$ b $x$ c", want: "c@1"},
		{name: "an unterminated dollar-quoted string", src: "a $x$ b", want: "a@1"},
		{name: "a parameter", src: "a = $12", want: "a@1 =@1 $12@1"},
		{name: "a lone dollar sign", src: "a $ b", want: "a@1 $@1 b@1"},
		{name: "a name with a dollar sign", src: "a$b.c", want: "a$b@1 .@1 c@1"},
		{name: "a quoted name", src: `"a""b".c`, want: `"a""b"@1 .@1 c@1`},
		{name: "a Unicode name", src: `U&"a".b`, want: `"a"@1 .@1 b@1`},
		{name: "a number with a fraction", src: "1.5 a", want: "1.5@1 a@1"},
		{name: "a name in UTF-8", src: "会議室.a", want: "会議室@1 .@1 a@1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := render(scanSQL([]byte(tt.src))); got != tt.want {
				t.Errorf("scanSQL(%q) = %q, want %q", tt.src, got, tt.want)
			}
		})
	}
}

func TestCreatedRelationsAndAliases(t *testing.T) {
	src := "CREATE TABLE IF NOT EXISTS a (id int);\n" +
		"CREATE OR REPLACE TEMP VIEW s.b AS SELECT 1 AS x;\n" +
		"CREATE UNLOGGED TABLE \"C\" ();\n" +
		"CREATE MATERIALIZED VIEW d AS SELECT 1;\n" +
		"CREATE TYPE e AS ENUM ('x');\n" +
		"CREATE INDEX f ON a (id);\n" +
		"ALTER TABLE g ADD COLUMN h int;\n" +
		"WITH w AS (SELECT 1) SELECT * FROM w;\n"
	tokens := scanSQL([]byte(src))
	if got, want := createdRelations(tokens), []string{"a", "b", "C", "d", "e"}; !slices.Equal(got, want) {
		t.Errorf("createdRelations() = %q, want %q", got, want)
	}
	aliases := aliasNames(tokens)
	for _, name := range []string{"x", "w"} {
		if !aliases[name] {
			t.Errorf("aliasNames() lacks %q", name)
		}
	}
}
