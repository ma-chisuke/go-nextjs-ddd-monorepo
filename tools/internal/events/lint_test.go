package events

import (
	"slices"
	"strings"
	"testing"
)

func TestLint(t *testing.T) {
	tests := []struct {
		name string
		// schema replaces goodSchema, and asyncAPI replaces goodAsyncAPI;
		// "-" leaves the AsyncAPI document out.
		schema   string
		asyncAPI string
		// path replaces fixtureSchema.
		path  string
		other map[string]string
		want  []string
	}{
		{
			name: "a schema that keeps every rule passes",
		},
		{
			name: "a property schema that refers to a file next to it passes",
			schema: edit(t, goodSchema, `"type": "integer",
			"minimum": 0`, `"$ref": "Floor.json"`),
			other: map[string]string{"contracts/events/facility/Floor.json": `{"type": "integer", "minimum": 0}`},
		},
		{
			name: "a message that refers to the schema through the document passes",
			asyncAPI: edit(t, goodAsyncAPI, "$ref: ./facility/RoomCleaned.v1.json", `$ref: "#/components/schemas/roomCleaned"`) +
				"    roomCleaned:\n      $ref: ./facility/RoomCleaned.v1.json\n"},
		{
			name: "a context in upper case",
			path: "contracts/events/Facility/RoomCleaned.v1.json",
			want: []string{"contracts/events/Facility/RoomCleaned.v1.json:1: [events/file-name]"},
		},
		{
			name: "a name in lowerCamelCase",
			path: "contracts/events/facility/roomCleaned.v1.json",
			want: []string{"contracts/events/facility/roomCleaned.v1.json:1: [events/file-name]"},
		},
		{
			name: "a version with a leading zero",
			path: "contracts/events/facility/RoomCleaned.v01.json",
			want: []string{"contracts/events/facility/RoomCleaned.v01.json:1: [events/file-name]"},
		},
		{
			name: "a version 0",
			path: "contracts/events/facility/RoomCleaned.v0.json",
			want: []string{"contracts/events/facility/RoomCleaned.v0.json:1: [events/file-name]"},
		},
		{
			name: "no version",
			path: "contracts/events/facility/RoomCleaned.json",
			want: []string{"contracts/events/facility/RoomCleaned.json:1: [events/file-name]"},
		},
		{
			name: "a schema outside the directory of a context",
			path: "contracts/events/RoomCleaned.v1.json",
			want: []string{"contracts/events/RoomCleaned.v1.json:1: [events/file-name]"},
		},
		{
			name:   "a file that is not JSON",
			schema: edit(t, goodSchema, `"type": "integer",`, `"type": integer,`),
			want:   []string{fixtureSchema + ":15: [events/json]"},
		},
		{
			name:   "two JSON values",
			schema: goodSchema + "{}\n",
			want:   []string{fixtureSchema + ":23: [events/json]"},
		},
		{
			name: "a key twice in an object",
			schema: edit(t, goodSchema, `"minimum": 0`, `"minimum": 0,
			"type": "number"`),
			want: []string{fixtureSchema + ":17: [events/json]"},
		},
		{
			name:   "an array",
			schema: "[]\n",
			want:   []string{fixtureSchema + ":1: [events/type]"},
		},
		{
			name:   "no $schema",
			schema: edit(t, goodSchema, `"$schema": "https://json-schema.org/draft/2020-12/schema",`, ``),
			want:   []string{fixtureSchema + ":1: [events/dialect]"},
		},
		{
			name:   "another dialect",
			schema: edit(t, goodSchema, `draft/2020-12/schema`, `draft/2019-09/schema`),
			want:   []string{fixtureSchema + ":2: [events/dialect]"},
		},
		{
			name:   "a title that is not the name of the event",
			schema: edit(t, goodSchema, `"title": "RoomCleaned"`, `"title": "Room cleaned"`),
			want:   []string{fixtureSchema + ":3: [events/title]"},
		},
		{
			name:   "no description",
			schema: edit(t, goodSchema, `"description": "A room was cleaned. A consumer ignores the properties that it does not know.",`, ``),
			want:   []string{fixtureSchema + ":1: [events/description]"},
		},
		{
			name:   "a property without a description",
			schema: edit(t, goodSchema, `"description": "The floor of the room.",`, ``),
			want:   []string{fixtureSchema + ":13: [events/description]"},
		},
		{
			name:   "a property with an empty description",
			schema: edit(t, goodSchema, `"The floor of the room."`, `" "`),
			want:   []string{fixtureSchema + ":13: [events/description]"},
		},
		{
			name: "a property of the items of an array without a description",
			schema: edit(t, goodSchema, `"type": "integer",
			"minimum": 0`, `"type": "array",
			"items": {"type": "object", "properties": {"level": {"type": "integer"}}}`),
			want: []string{fixtureSchema + ":16: [events/description]", fixtureSchema + ":20: [events/examples]"},
		},
		{
			name:   "a type other than object",
			schema: edit(t, goodSchema, `"type": "object"`, `"type": ["object", "null"]`),
			want:   []string{fixtureSchema + ":5: [events/type]"},
		},
		{
			name:   "a closed object",
			schema: edit(t, goodSchema, `"type": "object",`, `"type": "object", "additionalProperties": false,`),
			want:   []string{fixtureSchema + ":5: [events/closed]"},
		},
		{
			name:   "a closed object inside a property",
			schema: edit(t, goodSchema, `"minimum": 0`, `"minimum": 0, "unevaluatedProperties": false`),
			want:   []string{fixtureSchema + ":16: [events/closed]"},
		},
		{
			name:   "an open object with a schema for other properties passes",
			schema: edit(t, goodSchema, `"type": "object",`, `"type": "object", "additionalProperties": true,`),
		},
		{
			name:   "a required property that properties does not define",
			schema: edit(t, goodSchema, `"required": ["roomId"]`, `"required": ["roomId", "wing"]`),
			want:   []string{fixtureSchema + ":6: [events/required]", fixtureSchema + ":20: [events/examples]"},
		},
		{
			name:   "a schema that breaks the rules of JSON Schema",
			schema: edit(t, goodSchema, `"type": "integer"`, `"type": "integr"`),
			want:   []string{fixtureSchema + ":15: [events/compile]"},
		},
		{
			name:   "a pattern that is not a regular expression",
			schema: edit(t, goodSchema, `"format": "uuid"`, `"pattern": "["`),
			want:   []string{fixtureSchema + ":11: [events/compile]"},
		},
		{
			name: "a reference to a file that does not exist",
			schema: edit(t, goodSchema, `"type": "integer",
			"minimum": 0`, `"$ref": "Floor.json"`),
			want: []string{fixtureSchema + ":1: [events/compile]"},
		},
		{
			name: "a reference to the network",
			schema: edit(t, goodSchema, `"type": "integer",
			"minimum": 0`, `"$ref": "https://example.com/floor.json"`),
			want: []string{fixtureSchema + ":1: [events/compile]"},
		},
		{
			name:   "no examples",
			schema: edit(t, goodSchema, `{"roomId": "0193a6f0-1a2b-7c3d-8e4f-5a6b7c8d9e0f", "floor": 2}`, ``),
			want:   []string{fixtureSchema + ":19: [events/examples]"},
		},
		{
			name:   "an example that breaks a bound",
			schema: edit(t, goodSchema, `"floor": 2}`, `"floor": -1}`),
			want:   []string{fixtureSchema + ":20: [events/examples]"},
		},
		{
			name:   "an example that breaks a format",
			schema: edit(t, goodSchema, `"roomId": "0193a6f0-1a2b-7c3d-8e4f-5a6b7c8d9e0f"`, `"roomId": "room 1"`),
			want:   []string{fixtureSchema + ":20: [events/examples]"},
		},
		{
			name: "an example on several lines names the line of the value",
			schema: edit(t, goodSchema, `{"roomId": "0193a6f0-1a2b-7c3d-8e4f-5a6b7c8d9e0f", "floor": 2}`, `{
			"roomId": "0193a6f0-1a2b-7c3d-8e4f-5a6b7c8d9e0f",
			"floor": 2.5
		}`),
			want: []string{fixtureSchema + ":22: [events/examples]"},
		},
		{
			name:     "no AsyncAPI document",
			asyncAPI: "-",
			want:     []string{fixtureSchema + ":1: [events/asyncapi]"},
		},
		{
			name:     "no message with the name of the event",
			asyncAPI: edit(t, goodAsyncAPI, `name: facility.RoomCleaned.v1`, `name: facility.RoomCleaned.v2`),
			want:     []string{fixtureSchema + ":1: [events/asyncapi]"},
		},
		{
			name:     "a message whose payload does not refer to the schema",
			asyncAPI: edit(t, goodAsyncAPI, `$ref: ./facility/RoomCleaned.v1.json`, `$ref: ./facility/RoomCleaned.v2.json`),
			want:     []string{fixtureAsyncAPI + ":8: [events/asyncapi]"},
		},
		{
			name:     "an AsyncAPI document that does not parse",
			asyncAPI: goodAsyncAPI + "  - broken\n",
			want:     []string{fixtureAsyncAPI + ":19: [events/asyncapi]"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema := cmpOr(tt.schema, goodSchema)
			p := cmpOr(tt.path, fixtureSchema)
			files := map[string]string{p: schema}
			if tt.asyncAPI != "-" {
				files[fixtureAsyncAPI] = cmpOr(tt.asyncAPI, goodAsyncAPI)
			}
			for name, content := range tt.other {
				files[name] = content
			}
			findings, err := Lint(repository(files), []string{p})
			if err != nil {
				t.Fatal(err)
			}
			if got := places(findings); !slices.Equal(got, tt.want) {
				t.Errorf("Lint() found %q, want %q%s", got, tt.want, lines(findings))
			}
		})
	}
}

// cmpOr returns s, or fallback when s is empty.
func cmpOr(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

func TestLintMessages(t *testing.T) {
	tests := []struct {
		name   string
		schema string
		want   string
	}{
		{
			name:   "a missing description names the property",
			schema: edit(t, goodSchema, `"description": "The floor of the room.",`, ``),
			want: fixtureSchema + ":13: [events/description] the property floor at /properties/floor has no description. " +
				"Fix: add a description that says what the value means to a consumer. See TPL-0013.",
		},
		{
			name:   "an example names the place and the reason",
			schema: edit(t, goodSchema, `"floor": 2}`, `"floor": -1}`),
			want: fixtureSchema + ":20: [events/examples] example 0 does not match the schema at /floor: minimum: got -1, want 0. " +
				"Fix: correct the example, or the schema if the example shows data that the event really carries. See TPL-0013.",
		},
		{
			name:   "a closed object says why it is not allowed",
			schema: edit(t, goodSchema, `"type": "object",`, `"type": "object", "additionalProperties": false,`),
			want: fixtureSchema + ":5: [events/closed] /additionalProperties is false, which closes the object. " +
				"Fix: remove it: a consumer ignores the properties that it does not know, " +
				"and a closed object would make every added property a breaking change. See TPL-0013.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files := repository(map[string]string{fixtureSchema: tt.schema, fixtureAsyncAPI: goodAsyncAPI})
			findings, err := Lint(files, []string{fixtureSchema})
			if err != nil {
				t.Fatal(err)
			}
			if len(findings) != 1 || findings[0].String() != tt.want {
				t.Errorf("Lint() found%s\nwant\n\t%s", lines(findings), tt.want)
			}
		})
	}
}

func TestLintNamesTheFixOfAMissingMessage(t *testing.T) {
	files := repository(map[string]string{fixtureSchema: goodSchema})
	findings, err := Lint(files, []string{fixtureSchema})
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 || !strings.Contains(findings[0].Fix, "a message named facility.RoomCleaned.v1") ||
		!strings.Contains(findings[0].Fix, "./facility/RoomCleaned.v1.json") {
		t.Errorf("Lint() found%s\nwant one finding whose fix names the message and the reference", lines(findings))
	}
}

func TestLintReadsEachAsyncAPIDocumentOnce(t *testing.T) {
	// Two schemas of a context whose document does not parse: one finding
	// about the document, not one for each schema.
	second := "contracts/events/facility/RoomBooked.v1.json"
	files := repository(map[string]string{
		fixtureSchema:   goodSchema,
		second:          edit(t, goodSchema, `"title": "RoomCleaned"`, `"title": "RoomBooked"`),
		fixtureAsyncAPI: "asyncapi: [\n",
	})
	findings, err := Lint(files, []string{fixtureSchema, second})
	if err != nil {
		t.Fatal(err)
	}
	if got := places(findings); !slices.Equal(got, []string{fixtureAsyncAPI + ":2: [events/asyncapi]"}) {
		t.Errorf("Lint() found %q%s", got, lines(findings))
	}
}

func TestLintFailsOnAMissingFile(t *testing.T) {
	if _, err := Lint(repository(nil), []string{fixtureSchema}); err == nil {
		t.Error("Lint() of a file that does not exist returned no error")
	}
}

func TestParsePath(t *testing.T) {
	ev, ok := parsePath("contracts/events/reservation/ReservationConfirmed.v12.json")
	if !ok || ev.eventType() != "reservation.ReservationConfirmed.v12" || ev.next() != "ReservationConfirmed.v13.json" {
		t.Errorf("parsePath() = %+v, %v", ev, ok)
	}
	for _, p := range []string{
		"contracts/events/reservation/ReservationConfirmed.v1.yaml",
		"contracts/events/reservation/sub/ReservationConfirmed.v1.json",
		"contracts/events/reservation-2/ReservationConfirmed.v1.json",
		"contracts/events/reservation/Reservation_Confirmed.v1.json",
		"contracts/http/reservation/ReservationConfirmed.v1.json",
	} {
		if _, ok := parsePath(p); ok {
			t.Errorf("parsePath(%q) accepted the path", p)
		}
	}
}

func TestReadPaths(t *testing.T) {
	in := strings.Join([]string{
		"contracts/events/reservation/B.v1.json",
		"./contracts/events/reservation/A.v1.json",
		"contracts/events/reservation.asyncapi.yaml",
		"contracts/http/reservation.openapi.yaml",
		"",
		"contracts/events/reservation/A.v1.json",
	}, "\n")
	got, err := ReadPaths(strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"contracts/events/reservation/A.v1.json", "contracts/events/reservation/B.v1.json"}
	if !slices.Equal(got, want) {
		t.Errorf("ReadPaths() = %q, want %q", got, want)
	}
	if _, err := ReadPaths(strings.NewReader("contracts/events/../x.json\n")); err == nil {
		t.Error("ReadPaths() accepted a path that leaves the repository")
	}
}
