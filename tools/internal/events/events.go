// Package events checks the contracts of the integration events (TPL-0013):
// the JSON Schema of each event in contracts/events/<context>/<Name>.v<N>.json,
// and the AsyncAPI document of its context, contracts/events/<context>.asyncapi.yaml.
//
// Lint checks that each schema is well formed: its file name, its dialect,
// its descriptions, an open shape, and examples that match it. Breaking
// compares each schema with its version at a base commit, and fails on a
// change that a consumer of that version could not read.
// docs/guides/event-changes.md describes the rules for the people who meet
// a finding.
package events

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/tools/internal/finding"
)

// Rule identifiers of Lint.
const (
	RuleFileName    = "events/file-name"
	RuleJSON        = "events/json"
	RuleDialect     = "events/dialect"
	RuleCompile     = "events/compile"
	RuleTitle       = "events/title"
	RuleDescription = "events/description"
	RuleType        = "events/type"
	RuleClosed      = "events/closed"
	RuleRequired    = "events/required"
	RuleExamples    = "events/examples"
	RuleAsyncAPI    = "events/asyncapi"
)

// Rule identifiers of Breaking.
const (
	RulePropertyRemoved = "events/property-removed"
	RuleRequiredRemoved = "events/required-removed"
	RuleKeywordChanged  = "events/keyword-changed"
	RuleSchemaDeleted   = "events/schema-deleted"
)

const (
	// see names the decision record that explains the rules.
	see = "TPL-0013"
	// guide describes the rules for the people who meet a finding.
	guide = "docs/guides/event-changes.md"
	// contractsDir holds the event contracts.
	contractsDir = "contracts/events"
	// dialect is the only JSON Schema dialect of an event schema.
	dialect = "https://json-schema.org/draft/2020-12/schema"
)

// schemaPath is the path of an event schema: the context in lower case, the
// name of the event in UpperCamelCase, and its version, a positive integer.
var schemaPath = regexp.MustCompile(`^contracts/events/([a-z]+)/([A-Z][A-Za-z0-9]*)\.v([1-9][0-9]*)\.json$`)

// event is what the path of a schema says about its event.
type event struct {
	context string
	name    string
	version int
}

// parsePath returns the event of a schema path, and false when the path does
// not follow the naming rule.
func parsePath(p string) (event, bool) {
	m := schemaPath.FindStringSubmatch(p)
	if m == nil {
		return event{}, false
	}
	version, err := strconv.Atoi(m[3])
	if err != nil {
		return event{}, false
	}
	return event{context: m[1], name: m[2], version: version}, true
}

// eventType is the type of the event in its CloudEvents envelope.
func (e event) eventType() string {
	return fmt.Sprintf("%s.%s.v%d", e.context, e.name, e.version)
}

// file is the name of the schema file.
func (e event) file() string {
	return fmt.Sprintf("%s.v%d.json", e.name, e.version)
}

// next is the name of the schema file of the next version.
func (e event) next() string {
	return fmt.Sprintf("%s.v%d.json", e.name, e.version+1)
}

// asyncAPIPath is the AsyncAPI document of the context of the event.
func (e event) asyncAPIPath() string {
	return contractsDir + "/" + e.context + ".asyncapi.yaml"
}

// isSchema tells whether a path is in the directory of the event contracts
// and names a JSON file, whatever its name.
func isSchema(p string) bool {
	return strings.HasPrefix(p, contractsDir+"/") && strings.HasSuffix(p, ".json")
}

// ReadPaths reads the paths on r, one per line, and keeps the JSON files in
// contracts/events, sorted and without duplicates.
func ReadPaths(r io.Reader) ([]string, error) {
	scanner := bufio.NewScanner(r)
	var paths []string
	for n := 1; scanner.Scan(); n++ {
		p := strings.TrimPrefix(strings.TrimSpace(scanner.Text()), "./")
		if p == "" || !isSchema(p) {
			continue
		}
		if !fs.ValidPath(p) {
			return nil, fmt.Errorf("stdin line %d: %q is not a path relative to the repository root", n, p)
		}
		paths = append(paths, p)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read the list of files: %w", err)
	}
	slices.Sort(paths)
	return slices.Compact(paths), nil
}

// newFinding returns a finding of the rules of this package.
func newFinding(p string, line int, rule, message, fix string) finding.Finding {
	return finding.Finding{Path: p, Line: line, Rule: rule, Message: message, Fix: fix, See: see}
}

// pointer shows a JSON pointer in a message.
func pointer(ptr string) string {
	if ptr == "" {
		return "the root of the schema"
	}
	return ptr
}

// maxValue is the length after which compact shortens a value.
const maxValue = 80

// compact writes a JSON value on one line, shortened when it is long.
func compact(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprint(v)
	}
	s := string(data)
	if runes := []rune(s); len(runes) > maxValue {
		return string(runes[:maxValue-3]) + "..."
	}
	return s
}

// oneLine joins the lines of a message, so that a finding stays on one line.
func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
