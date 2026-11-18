package events

import (
	"errors"
	"maps"
	"testing"
)

func TestParseJSONIndexesLines(t *testing.T) {
	data := `{
	"a": 1,
	"b": {
		"c~d/e": [
			true,
			{"f": null}
		]
	},
	"g": [], "h": {}
}`
	doc, err := parseJSON([]byte(data))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]int{
		"":               1,
		"/a":             2,
		"/b":             3,
		"/b/c~0d~1e":     4,
		"/b/c~0d~1e/0":   5,
		"/b/c~0d~1e/1":   6,
		"/b/c~0d~1e/1/f": 6,
		"/g":             9,
		"/h":             9,
	}
	if !maps.Equal(doc.lines, want) {
		t.Errorf("lines = %v, want %v", doc.lines, want)
	}
	for ptr, line := range map[string]int{"/b/c~0d~1e/7/x": 4, "/missing": 1, "/a": 2} {
		if got := doc.line(ptr); got != line {
			t.Errorf("line(%q) = %d, want %d", ptr, got, line)
		}
	}
}

func TestParseJSONFindsDuplicateKeys(t *testing.T) {
	doc, err := parseJSON([]byte("{\n  \"a\": 1,\n  \"b\": {\"c\": 1,\n  \"c\": 2},\n  \"a\": 3\n}"))
	if err != nil {
		t.Fatal(err)
	}
	want := []duplicate{{ptr: "/b/c", line: 4}, {ptr: "/a", line: 5}}
	if len(doc.duplicates) != len(want) || doc.duplicates[0] != want[0] || doc.duplicates[1] != want[1] {
		t.Errorf("duplicates = %+v, want %+v", doc.duplicates, want)
	}
}

func TestParseJSONSyntaxErrors(t *testing.T) {
	tests := []struct {
		name string
		data string
		line int
	}{
		{name: "a bad value", data: "{\n  \"a\": 1,\n  \"b\": x\n}", line: 3},
		{name: "a missing end", data: "{\n  \"a\": 1,\n", line: 3},
		{name: "two values", data: "{}\n\n[]", line: 3},
		{name: "nothing", data: "", line: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseJSON([]byte(tt.data))
			var syntax *syntaxError
			if !errors.As(err, &syntax) {
				t.Fatalf("parseJSON() error = %v, want a syntax error", err)
			}
			if syntax.line != tt.line {
				t.Errorf("parseJSON() error on line %d, want line %d: %v", syntax.line, tt.line, err)
			}
		})
	}
}
