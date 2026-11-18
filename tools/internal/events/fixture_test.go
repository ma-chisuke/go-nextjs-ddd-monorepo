package events

import (
	_ "embed" // The YAML fixtures are files in testdata.
	"fmt"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/tools/internal/finding"
)

// The path of the schema of the fixtures, and of its AsyncAPI document.
const (
	fixtureSchema   = "contracts/events/facility/RoomCleaned.v1.json"
	fixtureAsyncAPI = "contracts/events/facility.asyncapi.yaml"
)

// goodSchema keeps every rule of Lint. The comments of the tests name its
// lines: properties on 7, roomId on 8, floor on 13, examples on 19.
const goodSchema = `{
	"$schema": "https://json-schema.org/draft/2020-12/schema",
	"title": "RoomCleaned",
	"description": "A room was cleaned. A consumer ignores the properties that it does not know.",
	"type": "object",
	"required": ["roomId"],
	"properties": {
		"roomId": {
			"description": "The id of the room.",
			"type": "string",
			"format": "uuid"
		},
		"floor": {
			"description": "The floor of the room.",
			"type": "integer",
			"minimum": 0
		}
	},
	"examples": [
		{"roomId": "0193a6f0-1a2b-7c3d-8e4f-5a6b7c8d9e0f", "floor": 2}
	]
}
`

// goodAsyncAPI has a message for goodSchema, which refers to the schema
// inside an allOf, as the documents of the repository do. The message is the
// mapping on line 8.
//
//go:embed testdata/facility.asyncapi.yaml
var goodAsyncAPI string

// edit replaces old with replacement in s, once, and fails the test when s
// does not hold old, so that a fixture never silently stays as it was.
func edit(t *testing.T, s, old, replacement string) string {
	t.Helper()
	if !strings.Contains(s, old) {
		t.Fatalf("the fixture does not hold %q", old)
	}
	return strings.Replace(s, old, replacement, 1)
}

// repository returns a file system with the given files.
func repository(files map[string]string) fstest.MapFS {
	fsys := fstest.MapFS{}
	for name, content := range files {
		fsys[name] = &fstest.MapFile{Data: []byte(content)}
	}
	return fsys
}

// places writes each finding as "path:line: [rule]".
func places(findings []finding.Finding) []string {
	finding.Sort(findings)
	out := make([]string, 0, len(findings))
	for _, f := range findings {
		out = append(out, fmt.Sprintf("%s:%d: [%s]", f.Path, f.Line, f.Rule))
	}
	return out
}

// lines writes each finding in full, for the message of a failed test.
func lines(findings []finding.Finding) string {
	var b strings.Builder
	for _, f := range findings {
		b.WriteString("\n\t" + f.String())
	}
	return b.String()
}
