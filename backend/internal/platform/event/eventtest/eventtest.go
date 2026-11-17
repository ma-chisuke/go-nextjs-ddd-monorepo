// Package eventtest checks events against their contracts in tests: the JSON
// Schema of each type of event in contracts/events/ (TPL-0013). The tests of
// a publisher pass every event that they produce through AssertMatchesContract,
// so a payload that leaves its schema fails a test, not a consumer.
package eventtest

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/event"
)

// contractsDir is the directory of the event contracts, from the root of the
// repository.
const contractsDir = "contracts/events"

// AssertMatchesContract fails the test unless e is a valid envelope whose data
// matches the JSON Schema of its type, formats included. The schema of the
// type "<context>.<Name>.v<N>" is contracts/events/<context>/<Name>.v<N>.json.
func AssertMatchesContract(tb testing.TB, e event.Envelope) {
	tb.Helper()
	if err := e.Validate(); err != nil {
		tb.Errorf("%v", err)
		return
	}
	path, err := SchemaPath(e.Type)
	if err != nil {
		tb.Errorf("%v", err)
		return
	}
	compiler := jsonschema.NewCompiler()
	compiler.AssertFormat()
	schema, err := compiler.Compile(path)
	if err != nil {
		tb.Errorf("event %s: its JSON Schema does not compile: %v", e.Type, err)
		return
	}
	data, err := jsonschema.UnmarshalJSON(bytes.NewReader(e.Data))
	if err != nil {
		tb.Errorf("event %s: data: %v", e.Type, err)
		return
	}
	if err := schema.Validate(data); err != nil {
		tb.Errorf("event %s: the data does not match %s: %v\ndata: %s", e.Type, filepath.Base(path), err, e.Data)
	}
}

// SchemaPath returns the path of the JSON Schema of an event type. It fails
// when the type is not "<context>.<Name>.v<N>" or when no file defines it.
func SchemaPath(eventType string) (string, error) {
	contextName, file, ok := strings.Cut(eventType, ".")
	if !ok || contextName == "" || file == "" || strings.ContainsAny(eventType, `/\`) {
		return "", fmt.Errorf("event type %q is not <context>.<Name>.v<N>", eventType)
	}
	dir, err := findContracts()
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, contextName, file+".json")
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("event type %q has no JSON Schema: %w", eventType, err)
	}
	return path, nil
}

// findContracts looks for contracts/events in the working directory of the
// test and in the directories above it.
func findContracts() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		candidate := filepath.Join(dir, filepath.FromSlash(contractsDir))
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no %s above the working directory", contractsDir)
		}
		dir = parent
	}
}
