package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	schemaPath   = "contracts/events/facility/RoomCleaned.v1.json"
	asyncAPIPath = "contracts/events/facility.asyncapi.yaml"
)

// schema returns an event schema whose property floor has the given type.
func schema(floorType string) string {
	return `{
	"$schema": "https://json-schema.org/draft/2020-12/schema",
	"title": "RoomCleaned",
	"description": "A room was cleaned.",
	"type": "object",
	"required": ["floor"],
	"properties": {
		"floor": {"description": "The floor of the room.", "type": "` + floorType + `"}
	},
	"examples": [{"floor": 2}]
}
`
}

const asyncAPI = "asyncapi: 3.0.0\ncomponents:\n  messages:\n    roomCleaned:\n" +
	"      name: facility.RoomCleaned.v1\n      payload:\n        $ref: ./facility/RoomCleaned.v1.json\n"

// repository writes the event of the fixtures, whose floor has the given
// type, into a new directory.
func repository(t *testing.T, floorType string) string {
	t.Helper()
	dir := t.TempDir()
	write(t, dir, schemaPath, schema(floorType))
	write(t, dir, asyncAPIPath, asyncAPI)
	return dir
}

func write(t *testing.T, dir, name, content string) {
	t.Helper()
	full := filepath.Join(dir, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestRun(t *testing.T) {
	good := repository(t, "integer")
	changed := repository(t, "string")
	deleted := t.TempDir()
	stdin := schemaPath + "\n" + asyncAPIPath + "\n"
	tests := []struct {
		name       string
		args       []string
		wantCode   int
		wantStdout string // the start of stdout
		wantStderr string // a substring of stderr
	}{
		{
			name: "schemas that keep the rules pass",
			args: []string{"-root", good, "lint"},
		},
		{
			name:       "a finding of lint fails",
			args:       []string{"-root", changed, "lint"},
			wantCode:   exitFindings,
			wantStdout: schemaPath + ":10: [events/examples] example 0 does not match the schema at /floor",
		},
		{
			name: "an unchanged schema passes",
			args: []string{"-root", good, "-base", good, "breaking"},
		},
		{
			name:       "a breaking change fails",
			args:       []string{"-root", changed, "-base", good, "-base-name", "abc1234", "breaking"},
			wantCode:   exitFindings,
			wantStdout: schemaPath + `:8: [events/keyword-changed] /properties/floor/type changed from "integer" to "string" since abc1234.`,
		},
		{
			name:       "a deleted schema fails",
			args:       []string{"-root", deleted, "-base", good, "breaking"},
			wantCode:   exitFindings,
			wantStdout: schemaPath + ":1: [events/schema-deleted] the schema of facility.RoomCleaned.v1 existed at the base and is gone",
		},
		{
			name: "a base without event contracts has nothing to compare",
			args: []string{"-root", good, "-base", deleted, "breaking"},
		},
		{
			name:       "breaking without a base is a usage error",
			args:       []string{"-root", good, "breaking"},
			wantCode:   exitError,
			wantStderr: "events: breaking needs -base",
		},
		{
			name:       "a missing base is an I/O error",
			args:       []string{"-root", good, "-base", filepath.Join(good, "missing"), "breaking"},
			wantCode:   exitError,
			wantStderr: "events: breaking: ",
		},
		{
			name:       "help is not an error",
			args:       []string{"-h"},
			wantStderr: "usage: events",
		},
		{
			name:       "no command is a usage error",
			args:       []string{"-root", good},
			wantCode:   exitError,
			wantStderr: `unknown command ""`,
		},
		{
			name:       "an unknown command is a usage error",
			args:       []string{"-root", good, "check"},
			wantCode:   exitError,
			wantStderr: `unknown command "check"`,
		},
		{
			name:       "an unknown flag is a usage error",
			args:       []string{"-verbose", "lint"},
			wantCode:   exitError,
			wantStderr: "flag provided but not defined",
		},
		{
			name:       "a missing root is an I/O error",
			args:       []string{"-root", filepath.Join(good, "missing"), "lint"},
			wantCode:   exitError,
			wantStderr: "events: lint: ",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(tt.args, strings.NewReader(stdin), &stdout, &stderr)
			okStdout := strings.HasPrefix(stdout.String(), tt.wantStdout) && (tt.wantStdout != "" || stdout.Len() == 0)
			if code != tt.wantCode || !okStdout || !strings.Contains(stderr.String(), tt.wantStderr) {
				t.Errorf("exit %d, stdout %q, stderr %q; want exit %d, stdout from %q, stderr with %q",
					code, stdout.String(), stderr.String(), tt.wantCode, tt.wantStdout, tt.wantStderr)
			}
		})
	}
}

func TestRunRejectsAPathOutsideTheRepository(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"-root", t.TempDir(), "lint"}, strings.NewReader("contracts/events/../../x.json\n"), &stdout, &stderr)
	if code != exitError || !strings.Contains(stderr.String(), "is not a path relative to the repository root") {
		t.Errorf("exit %d, stderr %q; want exit %d and the path named", code, stderr.String(), exitError)
	}
}
