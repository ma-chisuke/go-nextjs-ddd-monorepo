package events

import (
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/tools/internal/finding"
)

// Lint checks the event schemas at paths, relative to the root of fsys. The
// AsyncAPI document of the context of each schema is read from fsys too.
func Lint(fsys fs.FS, paths []string) ([]finding.Finding, error) {
	l := &linter{fsys: fsys, documents: map[string]*asyncAPI{}}
	for _, p := range paths {
		if err := l.lint(p); err != nil {
			return nil, err
		}
	}
	return l.findings, nil
}

// linter holds what the rules read, and collects their findings.
type linter struct {
	fsys fs.FS
	// documents holds the AsyncAPI documents read so far, by path.
	documents map[string]*asyncAPI
	findings  []finding.Finding
}

// schemaFile is one schema that the linter checks.
type schemaFile struct {
	path string
	doc  *document
	// schema is the root of the schema.
	schema map[string]any
}

func (l *linter) add(p string, line int, rule, message, fix string) {
	l.findings = append(l.findings, newFinding(p, line, rule, message, fix))
}

// at adds a finding at the place that ptr names in the schema.
func (l *linter) at(f *schemaFile, ptr, rule, message, fix string) {
	l.add(f.path, f.doc.line(ptr), rule, message, fix)
}

func (l *linter) lint(p string) error {
	ev, named := parsePath(p)
	if !named {
		l.add(p, 1, RuleFileName,
			"the path of an event schema is contracts/events/<context>/<Name>.v<N>.json: the context in lower case letters, "+
				"the name of the event in UpperCamelCase, and the version a positive integer without a leading zero",
			"move the file to the path of the type of its event, <context>.<Name>.v<N>, "+
				"such as contracts/events/reservation/ReservationConfirmed.v1.json for reservation.ReservationConfirmed.v1")
	}
	data, err := fs.ReadFile(l.fsys, p)
	if err != nil {
		return err
	}
	doc, err := parseJSON(data)
	var syntax *syntaxError
	if errors.As(err, &syntax) {
		l.add(p, syntax.line, RuleJSON, "the file is not one JSON value: "+oneLine(syntax.err.Error()), "correct the JSON")
		return nil
	}
	if err != nil {
		return err
	}
	f := &schemaFile{path: p, doc: doc}
	for _, d := range doc.duplicates {
		l.add(p, d.line, RuleJSON, fmt.Sprintf("%s appears twice in its object, and readers disagree on which one counts", d.ptr),
			"keep one of them")
	}
	schema, ok := doc.value.(map[string]any)
	if !ok {
		l.add(p, 1, RuleType, fmt.Sprintf("the schema is %s, not a JSON object", describe(doc.value)),
			`write the schema as an object with "type": "object"; the data of an event is a JSON object`)
		return nil
	}
	f.schema = schema
	l.checkRoot(f, ev, named)
	walkSchemas("", schema, func(ptr string, s map[string]any) {
		l.checkSubschema(f, ptr, s)
	})
	l.checkExamples(f)
	if named {
		return l.checkAsyncAPI(f, ev)
	}
	return nil
}

// checkRoot checks the keywords of the schema itself.
func (l *linter) checkRoot(f *schemaFile, ev event, named bool) {
	s := f.schema
	if got, ok := s["$schema"].(string); !ok || got != dialect {
		message := "the schema does not name its dialect in $schema"
		if _, present := s["$schema"]; present {
			message = fmt.Sprintf("$schema is %s, not the dialect of an event schema", compact(s["$schema"]))
		}
		l.at(f, "/$schema", RuleDialect, message, fmt.Sprintf(`set "$schema" to %q, JSON Schema 2020-12`, dialect))
	}
	if named {
		if got, _ := s["title"].(string); got != ev.name {
			message := fmt.Sprintf("the schema has no title; the title of an event schema is the name of its event, %q", ev.name)
			if _, present := s["title"]; present {
				message = fmt.Sprintf("title is %s; the title of an event schema is the name of its event, %q", compact(s["title"]), ev.name)
			}
			l.at(f, "/title", RuleTitle, message, fmt.Sprintf(`set "title" to %q`, ev.name))
		}
	}
	if !hasDescription(s) {
		l.at(f, "", RuleDescription, "the schema has no description",
			"add a description that says what happened, when the event is published, and that a consumer ignores the properties that it does not know")
	}
	if got, ok := s["type"].(string); !ok || got != "object" {
		message := "the schema has no type"
		if _, present := s["type"]; present {
			message = fmt.Sprintf("type is %s, not \"object\"", compact(s["type"]))
		}
		l.at(f, "/type", RuleType, message, `set "type" to "object"; the data of an event is a JSON object`)
	}
}

// checkSubschema checks one schema in the file, the root included.
func (l *linter) checkSubschema(f *schemaFile, ptr string, s map[string]any) {
	properties, _ := s["properties"].(map[string]any)
	for _, name := range sortedKeys(properties) {
		property, _ := properties[name].(map[string]any)
		if !hasDescription(property) {
			l.at(f, ptr+"/properties/"+escape(name), RuleDescription,
				fmt.Sprintf("the property %s at %s has no description", name, pointer(ptr+"/properties/"+escape(name))),
				"add a description that says what the value means to a consumer")
		}
	}
	for _, keyword := range []string{"additionalProperties", "unevaluatedProperties"} {
		if closed, ok := s[keyword].(bool); ok && !closed {
			l.at(f, ptr+"/"+keyword, RuleClosed, fmt.Sprintf("%s is false, which closes the object", pointer(ptr+"/"+keyword)),
				"remove it: a consumer ignores the properties that it does not know, "+
					"and a closed object would make every added property a breaking change")
		}
	}
	required, _ := s["required"].([]any)
	for i, item := range required {
		name, _ := item.(string)
		if _, ok := properties[name]; !ok {
			l.at(f, ptr+"/required/"+strconv.Itoa(i), RuleRequired,
				fmt.Sprintf("%s names %s, which %s does not define", pointer(ptr+"/required"), compact(item), pointer(ptr+"/properties")),
				"define the property in properties next to required, or remove the name from required")
		}
	}
}

// hasDescription tells whether a schema has a description with text.
func hasDescription(s map[string]any) bool {
	description, _ := s["description"].(string)
	return strings.TrimSpace(description) != ""
}

// checkExamples checks that the schema compiles, and that each example
// matches it, formats included.
func (l *linter) checkExamples(f *schemaFile) {
	examples, _ := f.schema["examples"].([]any)
	if len(examples) == 0 {
		l.at(f, "/examples", RuleExamples, "the schema has no examples",
			"add at least one example of the data of the event to examples, the data of a real event as a consumer would read it")
	}
	schema, err := compile(l.fsys, f.path, f.schema)
	if err != nil {
		l.reportCompile(f, err)
		return
	}
	for i, example := range examples {
		err := schema.Validate(example)
		var invalid *jsonschema.ValidationError
		if !errors.As(err, &invalid) {
			continue
		}
		for _, p := range problems(invalid) {
			message := fmt.Sprintf("example %d does not match the schema: %s", i, p.reasons)
			if p.ptr != "" {
				message = fmt.Sprintf("example %d does not match the schema at %s: %s", i, p.ptr, p.reasons)
			}
			l.at(f, "/examples/"+strconv.Itoa(i)+p.ptr, RuleExamples, message,
				"correct the example, or the schema if the example shows data that the event really carries")
		}
	}
}

// reportCompile turns the error of the compiler into findings: one for each
// place where the schema breaks the rules of JSON Schema, or one for the file.
func (l *linter) reportCompile(f *schemaFile, err error) {
	const fix = "correct the schema; the message names the place and the rule of JSON Schema 2020-12 that it breaks"
	var invalid *jsonschema.SchemaValidationError
	var cause *jsonschema.ValidationError
	if errors.As(err, &invalid) && errors.As(invalid.Err, &cause) {
		for _, p := range problems(cause) {
			message := "the schema breaks the rules of JSON Schema 2020-12: " + p.reasons
			if p.ptr != "" {
				message = fmt.Sprintf("the schema breaks the rules of JSON Schema 2020-12 at %s: %s", p.ptr, p.reasons)
			}
			l.at(f, p.ptr, RuleCompile, message, fix)
		}
		return
	}
	l.at(f, "", RuleCompile, "the schema does not compile: "+oneLine(err.Error()), fix)
}

// problem is the place of a value that breaks a schema, and the reasons.
type problem struct {
	ptr     string
	reasons string
}

// problems returns the places that a validation error names, each once, with
// their reasons: the errors at the ends of its tree. A value that matches
// none of several alternatives has a reason for each.
func problems(err *jsonschema.ValidationError) []problem {
	var out []problem
	var visit func(e *jsonschema.ValidationError)
	visit = func(e *jsonschema.ValidationError) {
		if len(e.Causes) > 0 {
			for _, cause := range e.Causes {
				visit(cause)
			}
			return
		}
		ptr := jsonPointer(e.InstanceLocation)
		// The error starts with its place, "at '<pointer>': ", which the
		// finding names on its own.
		reason := oneLine(e.Error())
		if strings.HasPrefix(reason, "at '") {
			if _, after, ok := strings.Cut(reason, "': "); ok {
				reason = after
			}
		}
		for i := range out {
			if out[i].ptr == ptr {
				out[i].reasons += "; " + reason
				return
			}
		}
		out = append(out, problem{ptr: ptr, reasons: reason})
	}
	visit(err)
	return out
}

// jsonPointer writes the tokens of a location as a JSON pointer.
func jsonPointer(tokens []string) string {
	var b strings.Builder
	for _, t := range tokens {
		b.WriteString("/" + escape(t))
	}
	return b.String()
}

// compile compiles the schema of the file at p. A reference to another file
// is read from fsys; the dialect comes with the compiler, so nothing is read
// from the network.
func compile(fsys fs.FS, p string, schema map[string]any) (*jsonschema.Schema, error) {
	c := jsonschema.NewCompiler()
	c.DefaultDraft(jsonschema.Draft2020)
	c.AssertFormat()
	c.UseLoader(fsLoader{fsys: fsys})
	location := fileURL(p)
	if err := c.AddResource(location, schema); err != nil {
		return nil, err
	}
	return c.Compile(location)
}

// fileURL is the URL of a file of the repository for the compiler.
func fileURL(p string) string {
	return "file:///" + p
}

// fsLoader loads the files that a schema refers to from the repository.
type fsLoader struct {
	fsys fs.FS
}

// Load reads the JSON file at a file URL.
func (l fsLoader) Load(location string) (any, error) {
	u, err := url.Parse(location)
	if err != nil {
		return nil, err
	}
	if u.Scheme != "file" {
		return nil, fmt.Errorf("%s is not a file of the repository; an event schema refers only to files next to it", location)
	}
	f, err := l.fsys.Open(strings.TrimPrefix(u.Path, "/"))
	if err != nil {
		return nil, err
	}
	// Closing a file that was only read cannot lose data.
	defer func() { _ = f.Close() }()
	return jsonschema.UnmarshalJSON(f)
}

// checkAsyncAPI checks that a message of the AsyncAPI document of the context
// has the type of the event as its name and refers to the schema.
func (l *linter) checkAsyncAPI(f *schemaFile, ev event) error {
	docPath := ev.asyncAPIPath()
	doc, ok := l.documents[docPath]
	if !ok {
		var err error
		doc, err = loadAsyncAPI(l.fsys, docPath)
		if err != nil {
			return err
		}
		l.documents[docPath] = doc
		if doc.parseError != "" {
			l.add(docPath, doc.parseLine, RuleAsyncAPI, "the AsyncAPI document does not parse: "+doc.parseError,
				"correct the YAML; `make lint-contracts` names the problem too")
		}
	}
	if doc.parseError != "" {
		return nil
	}
	ref := "./" + ev.context + "/" + ev.file()
	if doc.missing {
		l.at(f, "", RuleAsyncAPI, fmt.Sprintf("no AsyncAPI document %s describes the events of %s", docPath, ev.context),
			fmt.Sprintf("write %s, with a message named %s whose payload refers to %s, "+
				"as contracts/events/reservation.asyncapi.yaml does", docPath, ev.eventType(), ref))
		return nil
	}
	var named []message
	for _, m := range doc.messages {
		if m.name != ev.eventType() {
			continue
		}
		if slices.Contains(m.refs, f.path) {
			return nil
		}
		named = append(named, m)
	}
	if len(named) == 0 {
		l.at(f, "", RuleAsyncAPI, fmt.Sprintf("no message of %s is named %s, so no channel publishes this schema", docPath, ev.eventType()),
			fmt.Sprintf("add a message named %s to %s whose payload refers to %s, as its other messages do, "+
				"and the operation that sends it", ev.eventType(), docPath, ref))
		return nil
	}
	for _, m := range named {
		l.add(docPath, m.line, RuleAsyncAPI, fmt.Sprintf("the message %s does not refer to %s in its payload", m.name, ref),
			fmt.Sprintf("refer to the schema of the event in the payload, as in data: {$ref: %s}", ref))
	}
	return nil
}

// describe names the kind of a JSON value.
func describe(v any) string {
	switch v.(type) {
	case []any:
		return "a JSON array"
	case string:
		return "a JSON string"
	case bool:
		return "a JSON boolean"
	case nil:
		return "null"
	default:
		return "a JSON number"
	}
}
