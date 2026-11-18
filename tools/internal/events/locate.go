package events

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// document is a JSON file, parsed, with the line of each of its values.
type document struct {
	// value is the parsed JSON, with numbers as json.Number.
	value any
	// lines maps the JSON pointer of each value to its line: the line of the
	// key for a member of an object, and of the value for an item of an array.
	lines map[string]int
	// duplicates holds the members whose key appears again in their object.
	duplicates []duplicate
}

// duplicate is a key that appears a second time in an object.
type duplicate struct {
	ptr  string
	line int
}

// syntaxError is a JSON file that does not parse.
type syntaxError struct {
	line int
	err  error
}

func (e *syntaxError) Error() string {
	return fmt.Sprintf("line %d: %v", e.line, e.err)
}

// parseJSON parses one JSON value and indexes the lines of its values. The
// error is a *syntaxError when the data is not one JSON value.
func parseJSON(data []byte) (*document, error) {
	lines, duplicates, err := index(data)
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var value any
	if err := dec.Decode(&value); err != nil {
		return nil, &syntaxError{line: 1, err: err}
	}
	return &document{value: value, lines: lines, duplicates: duplicates}, nil
}

// line returns the line of the value at ptr, or of its nearest ancestor that
// the document holds, and 1 when there is none.
func (d *document) line(ptr string) int {
	for {
		if n, ok := d.lines[ptr]; ok {
			return n
		}
		i := strings.LastIndexByte(ptr, '/')
		if i < 0 {
			return 1
		}
		ptr = ptr[:i]
	}
}

// frame is an object or an array that the indexer is inside of.
type frame struct {
	ptr    string
	object bool
	// key is the key of the member whose value comes next; hasKey tells
	// whether the indexer has read it.
	key    string
	hasKey bool
	// index is the index of the next item of an array.
	index int
	// keys holds the keys that the object has had so far.
	keys map[string]bool
}

// index maps the JSON pointer of each value of data to its line. It reads the
// tokens of the data one by one, so it knows the offset of each.
func index(data []byte) (map[string]int, []duplicate, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	lines := map[string]int{}
	var duplicates []duplicate
	var stack []*frame
	cursor := lineCursor{data: data, line: 1}
	done := false
	for {
		offset := dec.InputOffset()
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			if !done {
				return nil, nil, &syntaxError{line: cursor.lineOf(int64(len(data))), err: errors.New("unexpected end of the file")}
			}
			break
		}
		if err != nil {
			return nil, nil, &syntaxError{line: cursor.lineOf(errorOffset(err, offset)), err: err}
		}
		line := cursor.lineOf(tokenStart(data, offset))
		if done {
			return nil, nil, &syntaxError{line: line, err: errors.New("more than one JSON value")}
		}
		var top *frame
		if len(stack) > 0 {
			top = stack[len(stack)-1]
		}
		if delim, ok := tok.(json.Delim); ok && (delim == '}' || delim == ']') {
			stack = stack[:len(stack)-1]
			done = len(stack) == 0
			if len(stack) > 0 {
				stack[len(stack)-1].next()
			}
			continue
		}
		if top != nil && top.object && !top.hasKey {
			key, _ := tok.(string)
			top.key, top.hasKey = key, true
			ptr := top.ptr + "/" + escape(key)
			if top.keys[key] {
				duplicates = append(duplicates, duplicate{ptr: ptr, line: line})
				continue
			}
			top.keys[key] = true
			lines[ptr] = line
			continue
		}
		ptr := ""
		if top != nil {
			ptr = top.child()
			if !top.object {
				lines[ptr] = line
			}
		} else {
			lines[ptr] = line
		}
		if delim, ok := tok.(json.Delim); ok {
			stack = append(stack, &frame{ptr: ptr, object: delim == '{', keys: map[string]bool{}})
			continue
		}
		if top == nil {
			done = true
		} else {
			top.next()
		}
	}
	return lines, duplicates, nil
}

// child returns the pointer of the value that comes next in f.
func (f *frame) child() string {
	if f.object {
		return f.ptr + "/" + escape(f.key)
	}
	return f.ptr + "/" + strconv.Itoa(f.index)
}

// next moves f past one complete value.
func (f *frame) next() {
	if f.object {
		f.hasKey = false
	} else {
		f.index++
	}
}

// tokenStart returns the offset of the token that follows offset: the
// decoder reports the end of the previous token, before the whitespace and
// the separators.
func tokenStart(data []byte, offset int64) int64 {
	for offset < int64(len(data)) {
		switch data[offset] {
		case ' ', '\t', '\n', '\r', ',', ':':
			offset++
		default:
			return offset
		}
	}
	return offset
}

// errorOffset returns the offset that a decoding error names, or fallback.
func errorOffset(err error, fallback int64) int64 {
	var syntax *json.SyntaxError
	if errors.As(err, &syntax) {
		return syntax.Offset
	}
	return fallback
}

// lineCursor turns offsets into lines. It counts each line break once, so
// offsets must not decrease, except that an earlier offset restarts the count.
type lineCursor struct {
	data   []byte
	offset int64
	line   int
}

func (c *lineCursor) lineOf(offset int64) int {
	offset = min(offset, int64(len(c.data)))
	if offset < c.offset {
		c.offset, c.line = 0, 1
	}
	c.line += bytes.Count(c.data[c.offset:offset], []byte{'\n'})
	c.offset = offset
	return c.line
}

// escape writes a key as a token of a JSON pointer (RFC 6901).
func escape(key string) string {
	return strings.ReplaceAll(strings.ReplaceAll(key, "~", "~0"), "/", "~1")
}
