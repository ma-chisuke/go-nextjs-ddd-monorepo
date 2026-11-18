package events

import (
	"errors"
	"io/fs"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v4"
)

// asyncAPI is an AsyncAPI document, as far as Lint reads it: its messages and
// the files that their payloads refer to.
type asyncAPI struct {
	path string
	// missing tells that the document does not exist.
	missing bool
	// parseError says why the document does not parse, at parseLine.
	parseError string
	parseLine  int
	messages   []message
}

// message is a message of an AsyncAPI document: a mapping with a name and a
// payload, wherever it is in the document.
type message struct {
	name string
	line int
	// refs holds the files that the payload refers to, directly or through a
	// reference inside the document, as paths from the root of the repository.
	refs []string
}

// errorLine finds the line in an error of the YAML parser.
var errorLine = regexp.MustCompile(`line (\d+)`)

// loadAsyncAPI reads the AsyncAPI document at p.
func loadAsyncAPI(fsys fs.FS, p string) (*asyncAPI, error) {
	doc := &asyncAPI{path: p}
	data, err := fs.ReadFile(fsys, p)
	if errors.Is(err, fs.ErrNotExist) {
		doc.missing = true
		return doc, nil
	}
	if err != nil {
		return nil, err
	}
	top, err := parseYAML(data)
	if err != nil {
		doc.parseError = oneLine(err.Error())
		doc.parseLine = 1
		if m := errorLine.FindStringSubmatch(err.Error()); m != nil {
			doc.parseLine, _ = strconv.Atoi(m[1])
		}
		return doc, nil
	}
	dir := path.Dir(p)
	visitNodes(top, map[*yaml.Node]bool{}, func(n *yaml.Node) {
		name, payload := member(n, "name"), member(n, "payload")
		if n.Kind != yaml.MappingNode || name == nil || name.Kind != yaml.ScalarNode || payload == nil {
			return
		}
		m := message{name: name.Value, line: n.Line}
		visitNodes(payload, map[*yaml.Node]bool{}, func(n *yaml.Node) {
			ref := member(n, "$ref")
			if ref == nil || ref.Kind != yaml.ScalarNode {
				return
			}
			if file, ok := refFile(dir, ref.Value); ok {
				m.refs = append(m.refs, file)
			}
		}, top)
		doc.messages = append(doc.messages, m)
	}, nil)
	return doc, nil
}

// parseYAML parses a YAML document and returns its top node.
func parseYAML(data []byte) (*yaml.Node, error) {
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, err
	}
	if root.Kind == yaml.DocumentNode && len(root.Content) > 0 {
		return root.Content[0], nil
	}
	return &root, nil
}

// visitNodes calls visit for n and every node inside it, each once. With a
// document, it also follows the references inside that document ("#/...").
func visitNodes(n *yaml.Node, seen map[*yaml.Node]bool, visit func(*yaml.Node), document *yaml.Node) {
	if n == nil || seen[n] {
		return
	}
	seen[n] = true
	if n.Kind == yaml.AliasNode {
		visitNodes(n.Alias, seen, visit, document)
		return
	}
	visit(n)
	if document != nil {
		if ref := member(n, "$ref"); ref != nil && strings.HasPrefix(ref.Value, "#") {
			visitNodes(lookup(document, ref.Value[1:]), seen, visit, document)
		}
	}
	for _, child := range n.Content {
		visitNodes(child, seen, visit, document)
	}
}

// member returns the value of key in a mapping node, or nil.
func member(n *yaml.Node, key string) *yaml.Node {
	if n == nil || n.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			return n.Content[i+1]
		}
	}
	return nil
}

// lookup returns the node at a JSON pointer (RFC 6901) in the fragment of a
// URI, or nil when there is none.
func lookup(document *yaml.Node, fragment string) *yaml.Node {
	ptr, err := url.PathUnescape(fragment)
	if err != nil || (ptr != "" && !strings.HasPrefix(ptr, "/")) {
		return nil
	}
	n := document
	for _, token := range strings.Split(ptr, "/")[1:] {
		token = strings.ReplaceAll(strings.ReplaceAll(token, "~1", "/"), "~0", "~")
		if n != nil && n.Kind == yaml.AliasNode {
			n = n.Alias
		}
		switch {
		case n == nil:
			return nil
		case n.Kind == yaml.MappingNode:
			n = member(n, token)
		case n.Kind == yaml.SequenceNode:
			i, err := strconv.Atoi(token)
			if err != nil || i < 0 || i >= len(n.Content) {
				return nil
			}
			n = n.Content[i]
		default:
			return nil
		}
	}
	return n
}

// refFile returns the file that a reference names, as a path from the root
// of the repository, when it names a file next to the document at dir.
func refFile(dir, ref string) (string, bool) {
	file, _, _ := strings.Cut(ref, "#")
	if file == "" || strings.Contains(file, ":") || strings.HasPrefix(file, "/") {
		return "", false
	}
	return path.Join(dir, file), true
}
