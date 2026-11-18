package events

import (
	"errors"
	"fmt"
	"io/fs"
	"reflect"
	"slices"

	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/tools/internal/finding"
)

// Base is the version of the event contracts that a change is compared with.
type Base struct {
	// Name names the base in findings, such as a short commit id.
	Name string
	// Files holds the content of each event schema at the base, by its path
	// from the root of the repository.
	Files map[string][]byte
}

// ReadBase reads the event schemas of a base from fsys, which holds the files
// of the base at their paths from the root of the repository: every file in
// contracts/events whose name ends in .json. fsys may lack the directory.
func ReadBase(fsys fs.FS, name string) (*Base, error) {
	base := &Base{Name: name, Files: map[string][]byte{}}
	if _, err := fs.Stat(fsys, contractsDir); errors.Is(err, fs.ErrNotExist) {
		return base, nil
	}
	err := fs.WalkDir(fsys, contractsDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !isSchema(p) {
			return nil
		}
		data, err := fs.ReadFile(fsys, p)
		if err != nil {
			return err
		}
		base.Files[p] = data
		return nil
	})
	if err != nil {
		return nil, err
	}
	return base, nil
}

// Breaking compares the event schemas at paths, relative to the root of fsys,
// with their version at base, and checks that no schema of base is gone from
// fsys. A schema that base does not have is new, and passes.
//
// A consumer that reads a version of an event was written against its schema
// at base, so the schema must still promise what it promised then: every
// property is still there, at every depth; every required property is still
// required; and no keyword that constrains a value changed, was added, or was
// removed. Annotations (freeKeywords) may change, and a property may be added.
func Breaking(fsys fs.FS, base *Base, paths []string) ([]finding.Finding, error) {
	var findings []finding.Finding
	for _, p := range paths {
		published, ok := base.Files[p]
		if !ok {
			continue
		}
		data, err := fs.ReadFile(fsys, p)
		if errors.Is(err, fs.ErrNotExist) {
			// Gone: the check of the files of the base below reports it.
			continue
		}
		if err != nil {
			return nil, err
		}
		// A file that does not parse has nothing to compare; Lint reports it.
		was, errWas := parseJSON(published)
		now, errNow := parseJSON(data)
		if errWas != nil || errNow != nil {
			continue
		}
		c := &comparer{base: base.Name}
		c.schema("", was.value, now.value)
		fix := newVersionFix(p, base.Name)
		for _, ch := range c.changes {
			findings = append(findings, newFinding(p, now.line(ch.ptr), ch.rule, ch.message, fix))
		}
	}
	for _, p := range sortedKeys(base.Files) {
		if _, err := fs.Stat(fsys, p); !errors.Is(err, fs.ErrNotExist) {
			if err != nil {
				return nil, err
			}
			continue
		}
		subject := "the schema"
		if ev, ok := parsePath(p); ok {
			subject = "the schema of " + ev.eventType()
		}
		findings = append(findings, newFinding(p, 1, RuleSchemaDeleted,
			fmt.Sprintf("%s existed at %s and is gone, but consumers of that version may still read it", subject, base.Name),
			fmt.Sprintf("restore the file (git checkout %s -- %s); publish a change as a new version in a new file next to it, "+
				"and keep this one, as %s describes", base.Name, p, guide)))
	}
	return findings, nil
}

// newVersionFix is the fix of a breaking change to the schema at p.
func newVersionFix(p, base string) string {
	version := "a new version of the event, in a new file"
	if ev, ok := parsePath(p); ok {
		version = "a new version, " + ev.next()
	}
	return fmt.Sprintf("publish the change as %s, and keep this file as it was at %s (%s)", version, base, guide)
}

// change is one difference between two versions of a schema that a consumer
// of the earlier version could not read.
type change struct {
	ptr     string
	rule    string
	message string
}

// comparer collects the changes between two versions of a schema.
type comparer struct {
	base    string
	changes []change
}

func (c *comparer) add(ptr, rule, message string) {
	c.changes = append(c.changes, change{ptr: ptr, rule: rule, message: message})
}

// schema compares two versions of the schema at ptr, keyword by keyword.
func (c *comparer) schema(ptr string, was, now any) {
	wasSchema, ok1 := was.(map[string]any)
	nowSchema, ok2 := now.(map[string]any)
	if !ok1 || !ok2 {
		c.value(ptr, was, now)
		return
	}
	keys := sortedKeys(wasSchema)
	for _, k := range sortedKeys(nowSchema) {
		if _, ok := wasSchema[k]; !ok {
			keys = append(keys, k)
		}
	}
	slices.Sort(keys)
	for _, k := range keys {
		if slices.Contains(freeKeywords, k) {
			continue
		}
		child := ptr + "/" + escape(k)
		w, inWas := wasSchema[k]
		n, inNow := nowSchema[k]
		switch {
		case k == "properties":
			c.properties(child, w, n)
		case k == "required":
			c.required(child, w, n)
		case !inWas && k == "$defs":
			// Definitions constrain nothing until a reference uses them, and
			// a new reference is a change of its own.
		case !inWas:
			c.add(child, RuleKeywordChanged, fmt.Sprintf("%s was added since %s, as %s", child, c.base, compact(n)))
		case !inNow:
			c.add(child, RuleKeywordChanged, fmt.Sprintf("%s was removed since %s; it was %s", child, c.base, compact(w)))
		case slices.Contains(schemaKeywords, k):
			c.schema(child, w, n)
		case slices.Contains(schemaMapKeywords, k):
			c.schemaMap(child, k, w, n)
		case slices.Contains(schemaArrayKeywords, k):
			c.schemaArray(child, w, n)
		default:
			c.value(child, w, n)
		}
	}
}

// properties compares two versions of the properties of a schema: each
// property must still be there, with a schema that compares equal. A new
// property passes. Either version may lack the keyword.
func (c *comparer) properties(ptr string, was, now any) {
	wasProperties, ok1 := was.(map[string]any)
	nowProperties, ok2 := now.(map[string]any)
	if (was != nil && !ok1) || (now != nil && !ok2) {
		c.value(ptr, was, now)
		return
	}
	for _, name := range sortedKeys(wasProperties) {
		child := ptr + "/" + escape(name)
		n, ok := nowProperties[name]
		if !ok {
			c.add(child, RulePropertyRemoved, fmt.Sprintf("the property %s at %s was removed since %s", name, child, c.base))
			continue
		}
		c.schema(child, wasProperties[name], n)
	}
}

// required compares two versions of the required properties of a schema:
// each name must still be there. A new name passes. Either version may lack
// the keyword.
func (c *comparer) required(ptr string, was, now any) {
	wasNames, ok1 := was.([]any)
	nowNames, ok2 := now.([]any)
	if (was != nil && !ok1) || (now != nil && !ok2) {
		c.value(ptr, was, now)
		return
	}
	for _, name := range wasNames {
		if !slices.Contains(nowNames, name) {
			c.add(ptr, RuleRequiredRemoved, fmt.Sprintf("%s no longer names %s, which it named at %s", ptr, compact(name), c.base))
		}
	}
}

// schemaMap compares two versions of a keyword that maps names to schemas.
// The names must be the same, except that $defs may gain one.
func (c *comparer) schemaMap(ptr, keyword string, was, now any) {
	wasMap, ok1 := was.(map[string]any)
	nowMap, ok2 := now.(map[string]any)
	if !ok1 || !ok2 {
		c.value(ptr, was, now)
		return
	}
	for _, name := range sortedKeys(wasMap) {
		child := ptr + "/" + escape(name)
		n, ok := nowMap[name]
		if !ok {
			c.add(child, RuleKeywordChanged, fmt.Sprintf("%s was removed since %s; it was %s", child, c.base, compact(wasMap[name])))
			continue
		}
		c.schema(child, wasMap[name], n)
	}
	if keyword == "$defs" {
		return
	}
	for _, name := range sortedKeys(nowMap) {
		if _, ok := wasMap[name]; !ok {
			child := ptr + "/" + escape(name)
			c.add(child, RuleKeywordChanged, fmt.Sprintf("%s was added since %s, as %s", child, c.base, compact(nowMap[name])))
		}
	}
}

// schemaArray compares two versions of a keyword that holds a list of
// schemas: the lists must be as long, and their schemas compare equal in order.
func (c *comparer) schemaArray(ptr string, was, now any) {
	wasItems, ok1 := was.([]any)
	nowItems, ok2 := now.([]any)
	if !ok1 || !ok2 || len(wasItems) != len(nowItems) {
		c.value(ptr, was, now)
		return
	}
	for i := range wasItems {
		c.schema(fmt.Sprintf("%s/%d", ptr, i), wasItems[i], nowItems[i])
	}
}

// value compares two versions of any other value: they must be equal.
func (c *comparer) value(ptr string, was, now any) {
	if !reflect.DeepEqual(was, now) {
		c.add(ptr, RuleKeywordChanged, fmt.Sprintf("%s changed from %s to %s since %s", pointer(ptr), compact(was), compact(now), c.base))
	}
}
