package arch

import (
	"bytes"
	"io/fs"
	"path"
	"slices"
	"strings"

	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/tools/internal/finding"
)

// checkSQL applies the rule schema-reference to the SQL files of each
// context. The schema of a context has the name of its directory (TPL-0009).
//
//   - schema.sql and the files under queries/ must not name the schema of
//     another context, such as reservation.bookings in Facility (TPL-0005);
//   - no SQL file of a context names its own schema: the search path of the
//     roles of the context decides it.
//
// A name counts as a schema when it qualifies another name: "facility." in
// "facility.rooms". The scanner skips comments, string constants, and
// dollar-quoted bodies. A relation that the SQL of the context creates, or an
// alias that a file introduces with AS, may share the name of a schema; a
// name qualified by it is a column, not a table, and is left alone.
func (c *checker) checkSQL() error {
	for _, context := range c.contexts {
		files, err := sqlFiles(c.fsys, path.Join(contextsDir, context))
		if err != nil {
			return err
		}
		scanned := map[string][]sqlToken{}
		relations := map[string]bool{}
		for _, file := range files {
			src, err := fs.ReadFile(c.fsys, file)
			if err != nil {
				return err
			}
			scanned[file] = scanSQL(src)
			for _, name := range createdRelations(scanned[file]) {
				relations[name] = true
			}
		}
		for _, file := range files {
			c.checkSQLFile(context, file, scanned[file], relations)
		}
	}
	return nil
}

// sqlFiles lists the SQL files under dir, in order.
func sqlFiles(fsys fs.FS, dir string) ([]string, error) {
	var files []string
	err := fs.WalkDir(fsys, dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(p, ".sql") {
			files = append(files, p)
		}
		return nil
	})
	return files, err
}

// checkSQLFile applies the rule schema-reference to one file of a context.
func (c *checker) checkSQLFile(context, file string, tokens []sqlToken, relations map[string]bool) {
	aliases := aliasNames(tokens)
	// Only the schema and the queries are code that runs as the context; a
	// fixture of a test may read another schema on purpose, to show that the
	// role cannot.
	runs := path.Base(file) == "schema.sql" || slices.Contains(strings.Split(path.Dir(file), "/"), "queries")
	for i := 0; i+2 < len(tokens); i++ {
		qualifier, dot, name := tokens[i], tokens[i+1], tokens[i+2]
		if qualifier.kind != sqlIdent || dot.kind != sqlDot || (name.kind != sqlIdent && name.kind != sqlStar) {
			continue
		}
		if relations[qualifier.name] || aliases[qualifier.name] {
			continue
		}
		written := qualifier.text + "." + name.text
		switch {
		case qualifier.name == context:
			c.add(finding.Finding{
				Path: file, Line: qualifier.line, Rule: RuleSchemaReference, See: seeSchemas,
				Message: written + " names the schema of its own context",
				Fix: "write " + name.text + " without the schema: the search path of the roles of the context " +
					"picks the schema (db/bootstrap.sql), so the SQL stays the same if the context moves",
			})
		case runs && slices.Contains(c.contexts, qualifier.name):
			c.add(finding.Finding{
				Path: file, Line: qualifier.line, Rule: RuleSchemaReference, See: seeContexts,
				Message: written + " names the schema of the context " + qualifier.name,
				Fix: "never read, write, or refer to the tables of another context: get the data of " +
					qualifier.name + " through its HTTP API or its events, and keep a copy in this context " +
					"if it must be joined",
			})
		}
	}
}

// createdRelations returns the names of the tables, views, and types that a
// file creates, as in "CREATE TABLE rooms" or "CREATE VIEW x.free_rooms".
func createdRelations(tokens []sqlToken) []string {
	var names []string
	for i, t := range tokens {
		if t.kind != sqlIdent || t.quoted || (t.name != "table" && t.name != "view" && t.name != "type") {
			continue
		}
		if !createsAt(tokens, i) {
			continue
		}
		j := i + 1
		for _, word := range []string{"if", "not", "exists"} {
			if j < len(tokens) && tokens[j].kind == sqlIdent && !tokens[j].quoted && tokens[j].name == word {
				j++
			}
		}
		// The last part of a qualified name is the relation.
		for j+2 < len(tokens) && tokens[j].kind == sqlIdent && tokens[j+1].kind == sqlDot && tokens[j+2].kind == sqlIdent {
			j += 2
		}
		if j < len(tokens) && tokens[j].kind == sqlIdent {
			names = append(names, tokens[j].name)
		}
	}
	return names
}

// createsAt reports whether the keyword at i belongs to a CREATE statement:
// CREATE comes before it, with only modifiers in between.
func createsAt(tokens []sqlToken, i int) bool {
	modifiers := map[string]bool{
		"or": true, "replace": true, "temp": true, "temporary": true, "unlogged": true,
		"global": true, "local": true, "materialized": true, "recursive": true, "foreign": true,
	}
	for j := i - 1; j >= 0; j-- {
		t := tokens[j]
		if t.kind != sqlIdent || t.quoted {
			return false
		}
		if t.name == "create" {
			return true
		}
		if !modifiers[t.name] {
			return false
		}
	}
	return false
}

// aliasNames returns the names that a file introduces with AS: aliases of
// tables and columns ("FROM bookings AS b"), and common table expressions
// ("WITH free AS (").
func aliasNames(tokens []sqlToken) map[string]bool {
	names := map[string]bool{}
	for i, t := range tokens {
		if t.kind != sqlIdent || t.quoted || t.name != "as" {
			continue
		}
		if i+1 < len(tokens) && tokens[i+1].kind == sqlIdent {
			names[tokens[i+1].name] = true
		}
		if i > 0 && i+1 < len(tokens) && tokens[i-1].kind == sqlIdent && tokens[i+1].kind == sqlOther && tokens[i+1].text == "(" {
			names[tokens[i-1].name] = true
		}
	}
	return names
}

// Kinds of SQL tokens.
const (
	sqlIdent = iota // a name or a keyword
	sqlDot
	sqlStar
	sqlOther // any other symbol, a number, or a parameter
)

// sqlToken is one token of SQL, without comments, constants, and spaces.
type sqlToken struct {
	kind int
	// text is the token as written; name is the identifier that it denotes:
	// folded to lower case unless it was quoted.
	text   string
	name   string
	quoted bool
	line   int
}

// scanSQL splits PostgreSQL source into tokens. It skips spaces, comments
// ("--" and nested "/* */"), string constants ('...', E'...' with
// backslashes, and the B, X, N, and U& forms), and dollar-quoted strings
// ($tag$...$tag$). Quoted identifiers ("...") are names. It does not need to
// be a full lexer: the rules only look for names followed by a dot.
func scanSQL(src []byte) []sqlToken {
	s := &sqlScanner{src: src, line: 1}
	for s.pos < len(src) {
		s.next()
	}
	return s.tokens
}

type sqlScanner struct {
	src    []byte
	pos    int
	line   int
	tokens []sqlToken
}

// next reads one token, or skips one comment, constant, or space.
func (s *sqlScanner) next() {
	ch := s.src[s.pos]
	switch {
	case ch == '\n':
		s.line++
		s.pos++
	case ch == ' ' || ch == '\t' || ch == '\r' || ch == '\f':
		s.pos++
	case s.has("--"):
		s.skipLine()
	case s.has("/*"):
		s.skipBlockComment()
	case ch == '\'':
		s.skipString(false)
	case ch == '"':
		s.quotedIdent()
	case ch == '$' && s.pos+1 < len(s.src) && isDigit(s.src[s.pos+1]):
		// A parameter such as $1.
		start := s.pos
		s.pos++
		for s.pos < len(s.src) && isDigit(s.src[s.pos]) {
			s.pos++
		}
		s.emit(sqlOther, string(s.src[start:s.pos]))
	case ch == '$':
		s.dollar()
	case isIdentStart(ch):
		s.ident()
	case isDigit(ch):
		start := s.pos
		for s.pos < len(s.src) && (isDigit(s.src[s.pos]) || s.src[s.pos] == '.' || isIdentPart(s.src[s.pos])) {
			s.pos++
		}
		s.emit(sqlOther, string(s.src[start:s.pos]))
	case ch == '.':
		s.pos++
		s.emit(sqlDot, ".")
	case ch == '*':
		s.pos++
		s.emit(sqlStar, "*")
	default:
		s.pos++
		s.emit(sqlOther, string(ch))
	}
}

func (s *sqlScanner) has(prefix string) bool {
	return bytes.HasPrefix(s.src[s.pos:], []byte(prefix))
}

func (s *sqlScanner) emit(kind int, text string) {
	s.tokens = append(s.tokens, sqlToken{kind: kind, text: text, name: text, line: s.line})
}

// advance moves to end, counting the lines in between.
func (s *sqlScanner) advance(end int) {
	s.line += bytes.Count(s.src[s.pos:end], []byte("\n"))
	s.pos = end
}

func (s *sqlScanner) skipLine() {
	end := bytes.IndexByte(s.src[s.pos:], '\n')
	if end < 0 {
		s.advance(len(s.src))
		return
	}
	s.advance(s.pos + end)
}

// skipBlockComment skips a block comment, which may nest in PostgreSQL.
func (s *sqlScanner) skipBlockComment() {
	depth := 0
	end := s.pos
	for end < len(s.src) {
		switch {
		case bytes.HasPrefix(s.src[end:], []byte("/*")):
			depth++
			end += 2
		case bytes.HasPrefix(s.src[end:], []byte("*/")):
			depth--
			end += 2
			if depth == 0 {
				s.advance(end)
				return
			}
		default:
			end++
		}
	}
	s.advance(len(s.src))
}

// skipString skips a string constant that starts at the current quote. A
// doubled quote stands for one quote; with backslashes (E'...'), a backslash
// escapes the next character.
func (s *sqlScanner) skipString(backslashes bool) {
	end := s.pos + 1
	for end < len(s.src) {
		switch {
		case backslashes && s.src[end] == '\\':
			end += 2
		case s.src[end] == '\'' && end+1 < len(s.src) && s.src[end+1] == '\'':
			end += 2
		case s.src[end] == '\'':
			s.advance(end + 1)
			return
		default:
			end++
		}
	}
	s.advance(len(s.src))
}

// quotedIdent reads a quoted identifier, in which a doubled quote stands for
// one quote.
func (s *sqlScanner) quotedIdent() {
	line := s.line
	var name strings.Builder
	end := s.pos + 1
	for end < len(s.src) {
		if s.src[end] == '"' {
			if end+1 < len(s.src) && s.src[end+1] == '"' {
				name.WriteByte('"')
				end += 2
				continue
			}
			break
		}
		name.WriteByte(s.src[end])
		end++
	}
	text := string(s.src[s.pos:min(end+1, len(s.src))])
	s.advance(min(end+1, len(s.src)))
	s.tokens = append(s.tokens, sqlToken{kind: sqlIdent, text: text, name: name.String(), quoted: true, line: line})
}

// dollar skips a dollar-quoted string, $$...$$ or $tag$...$tag$. A lone
// dollar sign is a symbol.
func (s *sqlScanner) dollar() {
	end := s.pos + 1
	for end < len(s.src) && isIdentPart(s.src[end]) && s.src[end] != '$' {
		end++
	}
	if end >= len(s.src) || s.src[end] != '$' {
		s.pos++
		s.emit(sqlOther, "$")
		return
	}
	tag := s.src[s.pos : end+1]
	closing := bytes.Index(s.src[end+1:], tag)
	if closing < 0 {
		s.advance(len(s.src))
		return
	}
	s.advance(end + 1 + closing + len(tag))
}

// ident reads a name or a keyword, or a string constant with a prefix such
// as E'...'.
func (s *sqlScanner) ident() {
	start := s.pos
	for s.pos < len(s.src) && isIdentPart(s.src[s.pos]) {
		s.pos++
	}
	text := string(s.src[start:s.pos])
	prefix := strings.ToLower(text)
	switch {
	case s.pos < len(s.src) && s.src[s.pos] == '\'' && (prefix == "e" || prefix == "b" || prefix == "x" || prefix == "n"):
		s.skipString(prefix == "e")
		return
	case prefix == "u" && s.has("&'"):
		s.pos++
		s.skipString(false)
		return
	case prefix == "u" && s.has("&\""):
		s.pos++
		s.quotedIdent()
		return
	}
	s.tokens = append(s.tokens, sqlToken{kind: sqlIdent, text: text, name: prefix, line: s.line})
}

func isDigit(ch byte) bool {
	return '0' <= ch && ch <= '9'
}

// isIdentStart reports whether a name can start with ch. Bytes of UTF-8
// sequences count as letters, as in PostgreSQL.
func isIdentStart(ch byte) bool {
	return 'a' <= ch && ch <= 'z' || 'A' <= ch && ch <= 'Z' || ch == '_' || ch >= 0x80
}

func isIdentPart(ch byte) bool {
	return isIdentStart(ch) || isDigit(ch) || ch == '$'
}
