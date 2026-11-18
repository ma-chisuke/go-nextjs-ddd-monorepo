package events

import (
	"slices"
	"strings"
	"testing"
)

func TestBreaking(t *testing.T) {
	tests := []struct {
		name string
		// now replaces goodSchema in the working tree; was replaces it at
		// the base.
		now  string
		was  string
		want []string
	}{
		{
			name: "an unchanged schema passes",
		},
		{
			name: "changed annotations pass",
			now: strings.NewReplacer(
				`"title": "RoomCleaned"`, `"title": "RoomCleaned", "$comment": "Reviewed.", "deprecated": true`,
				`A room was cleaned.`, `A room was cleaned by the staff.`,
				`"The floor of the room."`, `"The floor of the room, 0 for the ground floor."`,
				`"floor": 2}`, `"floor": 3}`,
			).Replace(goodSchema),
		},
		{
			name: "an added optional property passes",
			now: edit(t, goodSchema, `"properties": {`, `"properties": {
		"wing": {"description": "The wing of the building.", "type": "string"},`),
		},
		{
			name: "an added required property passes",
			now: edit(t, edit(t, goodSchema, `"properties": {`, `"properties": {
		"wing": {"description": "The wing of the building.", "type": "string"},`), `"required": ["roomId"]`, `"required": ["roomId", "wing"]`),
		},
		{
			name: "a property that becomes required passes",
			now:  edit(t, goodSchema, `"required": ["roomId"]`, `"required": ["roomId", "floor"]`),
		},
		{
			name: "new definitions pass",
			now:  edit(t, goodSchema, `"type": "object",`, `"type": "object", "$defs": {"level": {"type": "integer"}},`),
		},
		{
			name: "a removed property",
			now: edit(t, goodSchema, `,
		"floor": {
			"description": "The floor of the room.",
			"type": "integer",
			"minimum": 0
		}`, ``),
			want: []string{fixtureSchema + ":7: [events/property-removed]"},
		},
		{
			name: "a renamed property",
			now:  edit(t, goodSchema, `"floor": {`, `"level": {`),
			want: []string{fixtureSchema + ":7: [events/property-removed]"},
		},
		{
			name: "a property removed from the items of an array",
			was: edit(t, goodSchema, `"minimum": 0`, `"minimum": 0,
			"items": {"type": "object", "properties": {"level": {"type": "integer"}}}`),
			now: edit(t, goodSchema, `"minimum": 0`, `"minimum": 0,
			"items": {"type": "object", "properties": {}}`),
			want: []string{fixtureSchema + ":17: [events/property-removed]"},
		},
		{
			name: "a property dropped from required",
			now:  edit(t, goodSchema, `"required": ["roomId"]`, `"required": []`),
			want: []string{fixtureSchema + ":6: [events/required-removed]"},
		},
		{
			name: "required removed",
			now:  edit(t, goodSchema, `"required": ["roomId"],`, ``),
			want: []string{fixtureSchema + ":1: [events/required-removed]"},
		},
		{
			name: "a changed type",
			now:  edit(t, goodSchema, `"type": "integer"`, `"type": "number"`),
			want: []string{fixtureSchema + ":15: [events/keyword-changed]"},
		},
		{
			name: "a narrowed bound",
			now:  edit(t, goodSchema, `"minimum": 0`, `"minimum": 1`),
			want: []string{fixtureSchema + ":16: [events/keyword-changed]"},
		},
		{
			name: "an added bound",
			now:  edit(t, goodSchema, `"minimum": 0`, `"minimum": 0, "maximum": 99`),
			want: []string{fixtureSchema + ":16: [events/keyword-changed]"},
		},
		{
			name: "a removed format",
			now: edit(t, goodSchema, `"type": "string",
			"format": "uuid"`, `"type": "string"`),
			want: []string{fixtureSchema + ":8: [events/keyword-changed]"},
		},
		{
			name: "a changed enum",
			was:  edit(t, goodSchema, `"minimum": 0`, `"enum": [0, 1, 2]`),
			now:  edit(t, goodSchema, `"minimum": 0`, `"enum": [0, 1]`),
			want: []string{fixtureSchema + ":16: [events/keyword-changed]"},
		},
		{
			name: "a changed $schema",
			was:  edit(t, goodSchema, `draft/2020-12/schema`, `draft/2019-09/schema`),
			want: []string{fixtureSchema + ":2: [events/keyword-changed]"},
		},
		{
			name: "a changed schema inside allOf",
			was:  edit(t, goodSchema, `"minimum": 0`, `"allOf": [{"minimum": 0}]`),
			now:  edit(t, goodSchema, `"minimum": 0`, `"allOf": [{"minimum": 1}]`),
			want: []string{fixtureSchema + ":16: [events/keyword-changed]"},
		},
		{
			name: "a schema added to allOf",
			was:  edit(t, goodSchema, `"minimum": 0`, `"allOf": [{"minimum": 0}]`),
			now:  edit(t, goodSchema, `"minimum": 0`, `"allOf": [{"minimum": 0}, {"maximum": 9}]`),
			want: []string{fixtureSchema + ":16: [events/keyword-changed]"},
		},
		{
			name: "a description changed inside allOf passes",
			was:  edit(t, goodSchema, `"minimum": 0`, `"allOf": [{"minimum": 0, "description": "a"}]`),
			now:  edit(t, goodSchema, `"minimum": 0`, `"allOf": [{"minimum": 0, "description": "b"}]`),
		},
		{
			name: "an added pattern property",
			now:  edit(t, goodSchema, `"minimum": 0`, `"minimum": 0, "patternProperties": {"^x-": {"type": "string"}}`),
			want: []string{fixtureSchema + ":16: [events/keyword-changed]"},
		},
		{
			name: "a changed definition",
			was:  edit(t, goodSchema, `"type": "object",`, `"type": "object", "$defs": {"level": {"type": "integer"}},`),
			now:  edit(t, goodSchema, `"type": "object",`, `"type": "object", "$defs": {"level": {"type": "string"}},`),
			want: []string{fixtureSchema + ":5: [events/keyword-changed]"},
		},
		{
			name: "a schema that does not parse has nothing to compare",
			now:  "{",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.now == goodSchema || tt.was == goodSchema {
				t.Fatal("the fixture does not change the schema")
			}
			base := &Base{Name: "abc1234", Files: map[string][]byte{fixtureSchema: []byte(cmpOr(tt.was, goodSchema))}}
			files := repository(map[string]string{fixtureSchema: cmpOr(tt.now, goodSchema)})
			findings, err := Breaking(files, base, []string{fixtureSchema})
			if err != nil {
				t.Fatal(err)
			}
			if got := places(findings); !slices.Equal(got, tt.want) {
				t.Errorf("Breaking() found %q, want %q%s", got, tt.want, lines(findings))
			}
		})
	}
}

func TestBreakingMessages(t *testing.T) {
	const fix = "Fix: publish the change as a new version, RoomCleaned.v2.json, and keep this file as it was at abc1234 " +
		"(docs/guides/event-changes.md). See TPL-0013."
	tests := []struct {
		name string
		now  string
		want string
	}{
		{
			name: "a changed value names the place, the old value, and the new one",
			now:  edit(t, goodSchema, `"type": "integer"`, `"type": "string"`),
			want: fixtureSchema + `:15: [events/keyword-changed] /properties/floor/type changed from "integer" to "string" since abc1234. ` + fix,
		},
		{
			name: "an added keyword names its value",
			now:  edit(t, goodSchema, `"minimum": 0`, `"minimum": 0, "maximum": 99`),
			want: fixtureSchema + `:16: [events/keyword-changed] /properties/floor/maximum was added since abc1234, as 99. ` + fix,
		},
		{
			name: "a removed keyword names its value",
			now:  edit(t, goodSchema, `"type": "integer",`, ``),
			want: fixtureSchema + `:13: [events/keyword-changed] /properties/floor/type was removed since abc1234; it was "integer". ` + fix,
		},
		{
			name: "a removed property names the property",
			now:  edit(t, goodSchema, `"floor": {`, `"level": {`),
			want: fixtureSchema + `:7: [events/property-removed] the property floor at /properties/floor was removed since abc1234. ` + fix,
		},
		{
			name: "a property dropped from required names it",
			now:  edit(t, goodSchema, `"required": ["roomId"]`, `"required": []`),
			want: fixtureSchema + `:6: [events/required-removed] /required no longer names "roomId", which it named at abc1234. ` + fix,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base := &Base{Name: "abc1234", Files: map[string][]byte{fixtureSchema: []byte(goodSchema)}}
			findings, err := Breaking(repository(map[string]string{fixtureSchema: tt.now}), base, []string{fixtureSchema})
			if err != nil {
				t.Fatal(err)
			}
			if len(findings) != 1 || findings[0].String() != tt.want {
				t.Errorf("Breaking() found%s\nwant\n\t%s", lines(findings), tt.want)
			}
		})
	}
}

func TestBreakingFiles(t *testing.T) {
	renamed := "contracts/events/facility/RoomTidied.v1.json"
	tests := []struct {
		name  string
		files map[string]string
		paths []string
		want  []string
	}{
		{
			name:  "a new schema has nothing to compare",
			files: map[string]string{fixtureSchema: goodSchema, renamed: "{}"},
			paths: []string{fixtureSchema, renamed},
		},
		{
			name:  "a schema out of scope is not compared",
			files: map[string]string{fixtureSchema: "{}"},
		},
		{
			name: "a deleted schema",
			want: []string{fixtureSchema + ":1: [events/schema-deleted]"},
		},
		{
			name:  "a renamed schema",
			files: map[string]string{renamed: goodSchema},
			paths: []string{renamed},
			want:  []string{fixtureSchema + ":1: [events/schema-deleted]"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base := &Base{Name: "abc1234", Files: map[string][]byte{fixtureSchema: []byte(goodSchema)}}
			findings, err := Breaking(repository(tt.files), base, tt.paths)
			if err != nil {
				t.Fatal(err)
			}
			if got := places(findings); !slices.Equal(got, tt.want) {
				t.Errorf("Breaking() found %q, want %q%s", got, tt.want, lines(findings))
			}
		})
	}
}

func TestBreakingNamesTheEventOfADeletedSchema(t *testing.T) {
	base := &Base{Name: "abc1234", Files: map[string][]byte{fixtureSchema: []byte(goodSchema)}}
	findings, err := Breaking(repository(nil), base, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := fixtureSchema + ":1: [events/schema-deleted] the schema of facility.RoomCleaned.v1 existed at abc1234 and is gone, " +
		"but consumers of that version may still read it. Fix: restore the file (git checkout abc1234 -- " + fixtureSchema +
		"); publish a change as a new version in a new file next to it, and keep this one, as docs/guides/event-changes.md describes. See TPL-0013."
	if len(findings) != 1 || findings[0].String() != want {
		t.Errorf("Breaking() found%s\nwant\n\t%s", lines(findings), want)
	}
}

func TestReadBase(t *testing.T) {
	fsys := repository(map[string]string{
		fixtureSchema: goodSchema,
		"contracts/events/facility/Empty.v1.json": "",
		"contracts/events/Loose.json":             "{}",
		fixtureAsyncAPI:                           goodAsyncAPI,
		"contracts/http/facility.openapi.yaml":    "openapi: 3.1.0\n",
		"contracts/events-old/Other.v1.json":      "{}",
	})
	base, err := ReadBase(fsys, "abc1234")
	if err != nil {
		t.Fatal(err)
	}
	got := sortedKeys(base.Files)
	want := []string{"contracts/events/Loose.json", "contracts/events/facility/Empty.v1.json", fixtureSchema}
	if base.Name != "abc1234" || !slices.Equal(got, want) || string(base.Files[fixtureSchema]) != goodSchema {
		t.Errorf("ReadBase() = %q with %q, want abc1234 with %q", base.Name, got, want)
	}
}

func TestReadBaseWithoutContracts(t *testing.T) {
	base, err := ReadBase(repository(map[string]string{"README.md": "# A repository\n"}), "abc1234")
	if err != nil || len(base.Files) != 0 {
		t.Errorf("ReadBase() = %+v, %v; want no files", base, err)
	}
}
