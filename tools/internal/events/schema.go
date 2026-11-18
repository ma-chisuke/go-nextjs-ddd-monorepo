package events

import (
	"slices"
	"strconv"
)

// The keywords of JSON Schema 2020-12 whose value is a schema, a map of names
// to schemas, or an array of schemas. Lint walks them to reach every schema in
// a file, and Breaking compares them schema by schema.
var (
	schemaKeywords = []string{
		"items", "additionalProperties", "unevaluatedProperties", "unevaluatedItems",
		"contains", "propertyNames", "not", "if", "then", "else", "contentSchema",
	}
	schemaMapKeywords   = []string{"properties", "patternProperties", "dependentSchemas", "$defs"}
	schemaArrayKeywords = []string{"allOf", "anyOf", "oneOf", "prefixItems"}
)

// freeKeywords annotate a schema and constrain no value, so a published
// schema may change them.
var freeKeywords = []string{"description", "title", "examples", "$comment", "deprecated"}

// walkSchemas calls visit for the schema at ptr and for every schema inside
// it, depth first. A schema that is not an object, such as true, is skipped.
func walkSchemas(ptr string, v any, visit func(ptr string, schema map[string]any)) {
	schema, ok := v.(map[string]any)
	if !ok {
		return
	}
	visit(ptr, schema)
	for _, k := range sortedKeys(schema) {
		child := ptr + "/" + escape(k)
		switch {
		case slices.Contains(schemaKeywords, k):
			walkSchemas(child, schema[k], visit)
		case slices.Contains(schemaMapKeywords, k):
			members, _ := schema[k].(map[string]any)
			for _, name := range sortedKeys(members) {
				walkSchemas(child+"/"+escape(name), members[name], visit)
			}
		case slices.Contains(schemaArrayKeywords, k):
			items, _ := schema[k].([]any)
			for i, item := range items {
				walkSchemas(child+"/"+strconv.Itoa(i), item, visit)
			}
		}
	}
}

// sortedKeys returns the keys of m in order, so that results do not depend on
// the order of a map.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}
