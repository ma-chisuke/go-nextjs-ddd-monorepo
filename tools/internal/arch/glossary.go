package arch

import (
	"errors"
	"fmt"
	"go/ast"
	"go/token"
	"io/fs"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/tools/internal/finding"
)

// glossaryLanguage is one of the two files of a glossary.
type glossaryLanguage struct {
	name   string
	header []string
}

var (
	glossaryEN = glossaryLanguage{name: "glossary.md", header: []string{"Identifier", "Term", "Meaning"}}
	glossaryJA = glossaryLanguage{name: "glossary.ja.md", header: []string{"識別子", "用語", "意味"}}
)

// glossaryRow is one term of a glossary.
type glossaryRow struct {
	ident string
	line  int
}

// glossary is a parsed glossary file.
type glossary struct {
	path string
	// headerLine is the line of the header of the table, or 1.
	headerLine int
	rows       []glossaryRow
	// usable is false when the table is missing or has the wrong header; the
	// rules that compare identifiers then skip the file.
	usable bool
}

// exportedAPI is what a package exports: its types, with the place of their
// declarations, and every exported name, including "Type.Field" and
// "Type.Method".
type exportedAPI struct {
	dir   string
	types map[string]string
	names map[string]bool
}

// checkGlossaries applies the rule glossary to each context: the glossary
// and its translation exist, each is one table, the English table lists every
// exported type of the domain package (of the app package in a context
// without a domain layer) and names nothing else, and both tables list the
// same identifiers.
func (c *checker) checkGlossaries() error {
	for _, context := range c.contexts {
		layer := layerApp
		if c.hasDomain[context] {
			layer = layerDomain
		}
		api := c.exported(layerDir(context, layer))
		en, err := c.readGlossary(context, glossaryEN, api)
		if err != nil {
			return err
		}
		ja, err := c.readGlossary(context, glossaryJA, api)
		if err != nil {
			return err
		}
		if en.usable {
			c.checkCoverage(en, api)
			if ja.usable {
				c.compareGlossaries(en, ja)
			}
		}
	}
	return nil
}

// glossaryFinding returns a finding of the rule glossary.
func glossaryFinding(file string, line int, message, fix string) finding.Finding {
	return finding.Finding{Path: file, Line: line, Rule: RuleGlossary, See: seeSplit, Message: message, Fix: fix}
}

// readGlossary reads and checks the form of one file of the glossary of a
// context. A file that does not exist is not usable.
func (c *checker) readGlossary(context string, lang glossaryLanguage, api exportedAPI) (*glossary, error) {
	file := path.Join(docsDir, context, lang.name)
	g := &glossary{path: file, headerLine: 1}
	data, err := fs.ReadFile(c.fsys, file)
	if errors.Is(err, fs.ErrNotExist) {
		fix := "write " + file + ": one table with the header " + tableRow(lang.header) +
			" and a row for each exported type of " + api.dir + ", with the identifier in backticks"
		if lang.name == glossaryJA.name {
			fix = "translate " + glossaryEN.name + " into " + file + ": the same identifiers, under the header " +
				tableRow(lang.header) + ", then run `make i18n-stamp`"
		}
		c.add(glossaryFinding(file, 1, "the context "+context+" has no "+lang.name, fix))
		return g, nil
	}
	if err != nil {
		return nil, err
	}
	tables := markdownTables(string(data))
	if len(tables) == 0 {
		c.add(glossaryFinding(file, 1, "the glossary has no table",
			"write one table with the header "+tableRow(lang.header)))
		return g, nil
	}
	for _, extra := range tables[1:] {
		c.add(glossaryFinding(file, extra[0].line, "the glossary has more than one table",
			"put every term in the first table"))
	}
	table := tables[0]
	g.headerLine = table[0].line
	if !slices.Equal(table[0].cells, lang.header) {
		c.add(glossaryFinding(file, table[0].line, "the header of the table is "+tableRow(table[0].cells),
			"write the header "+tableRow(lang.header)))
		return g, nil
	}
	g.usable = true
	body := table[1:]
	if len(body) == 0 || !isDelimiterRow(body[0].cells) {
		c.add(glossaryFinding(file, table[0].line, "the header of the table has no delimiter row under it",
			"write |---|---|---| under the header"))
	} else {
		body = body[1:]
	}
	seen := map[string]int{}
	for _, row := range body {
		ident, ok := c.glossaryIdent(file, row, len(lang.header))
		if !ok {
			// Until the row names its identifier, a comparison would report
			// the same mistake again, as a missing or an extra row.
			g.usable = false
			continue
		}
		if first, dup := seen[ident]; dup {
			c.add(glossaryFinding(file, row.line, fmt.Sprintf("`%s` is in the table already, on line %d", ident, first),
				"keep one row for each identifier"))
			continue
		}
		seen[ident] = row.line
		g.rows = append(g.rows, glossaryRow{ident: ident, line: row.line})
	}
	return g, nil
}

// identCell is the first cell of a row: an identifier in backticks.
var identCell = regexp.MustCompile("^`([^`]+)`$")

// glossaryIdent checks the cells of one row of a glossary, and returns its
// identifier.
func (c *checker) glossaryIdent(file string, row tableLine, width int) (string, bool) {
	m := identCell.FindStringSubmatch(row.cells[0])
	ok := m != nil && isExportedName(m[1])
	switch {
	case len(row.cells) != width:
		c.add(glossaryFinding(file, row.line, fmt.Sprintf("the row has %d cells, not %d", len(row.cells), width),
			"write the identifier, the term, and its meaning, and escape a | inside a cell as \\|"))
	case !ok:
		c.add(glossaryFinding(file, row.line, "the first cell "+row.cells[0]+" is not an exported Go identifier in backticks",
			"write the identifier as the code spells it, such as `Room`, or `Room.Capacity` for a field or a method"))
	case slices.Contains(row.cells[1:], ""):
		c.add(glossaryFinding(file, row.line, "the row of `"+m[1]+"` has an empty cell",
			"write the term of the business and what it means"))
	}
	if !ok {
		return "", false
	}
	return m[1], true
}

// isExportedName reports whether s is an exported Go identifier, or two of
// them joined by a dot: a type and its field or method.
func isExportedName(s string) bool {
	parts := strings.Split(s, ".")
	if len(parts) > 2 {
		return false
	}
	for _, p := range parts {
		if !token.IsIdentifier(p) || !token.IsExported(p) {
			return false
		}
	}
	return true
}

// checkCoverage reports the exported types that the English glossary lacks,
// and the identifiers that it lists but the code does not export.
func (c *checker) checkCoverage(g *glossary, api exportedAPI) {
	listed := map[string]bool{}
	for _, row := range g.rows {
		listed[row.ident] = true
		if !api.names[row.ident] {
			c.add(glossaryFinding(g.path, row.line, "`"+row.ident+"` is not an exported identifier of "+api.dir,
				"write the identifier as the code spells it, or remove its row from both files of the glossary "+
					"if the code no longer has it"))
		}
	}
	types := make([]string, 0, len(api.types))
	for name := range api.types {
		types = append(types, name)
	}
	slices.Sort(types)
	for _, name := range types {
		if listed[name] {
			continue
		}
		c.add(glossaryFinding(g.path, g.headerLine,
			"the exported type `"+name+"` ("+api.types[name]+") is not in the glossary",
			"add a row for `"+name+"` to "+path.Base(g.path)+" and to its translation: the identifier, "+
				"the term of the business, and what it means"))
	}
}

// compareGlossaries reports the identifiers that only one of the two files
// of a glossary lists, in the translation.
func (c *checker) compareGlossaries(en, ja *glossary) {
	inEN := map[string]bool{}
	for _, row := range en.rows {
		inEN[row.ident] = true
	}
	inJA := map[string]bool{}
	for _, row := range ja.rows {
		inJA[row.ident] = true
		if !inEN[row.ident] {
			c.add(glossaryFinding(ja.path, row.line, "`"+row.ident+"` is in the translation but not in "+path.Base(en.path),
				"list the same identifiers in both files of the glossary"))
		}
	}
	for _, row := range en.rows {
		if !inJA[row.ident] {
			c.add(glossaryFinding(ja.path, ja.headerLine, "`"+row.ident+"` is in "+path.Base(en.path)+" but not in the translation",
				"list the same identifiers in both files of the glossary"))
		}
	}
}

// exported collects what the package in dir exports, from its non-test files.
func (c *checker) exported(dir string) exportedAPI {
	api := exportedAPI{dir: dir, types: map[string]string{}, names: map[string]bool{}}
	for _, p := range c.module.packages {
		if p.dir != dir {
			continue
		}
		for _, f := range p.files {
			if !f.test {
				c.collectExported(f, &api)
			}
		}
	}
	return api
}

// collectExported adds the exported declarations of one file to api.
func (c *checker) collectExported(f *goFile, api *exportedAPI) {
	for _, decl := range f.syntax.Decls {
		switch decl := decl.(type) {
		case *ast.FuncDecl:
			name := decl.Name.Name
			if decl.Recv != nil && len(decl.Recv.List) > 0 {
				name = receiverName(decl.Recv.List[0].Type) + "." + name
			}
			if isExportedName(name) {
				api.names[name] = true
			}
		case *ast.GenDecl:
			for _, spec := range decl.Specs {
				switch spec := spec.(type) {
				case *ast.TypeSpec:
					if !spec.Name.IsExported() {
						continue
					}
					api.names[spec.Name.Name] = true
					api.types[spec.Name.Name] = path.Base(f.path) + ":" + strconv.Itoa(c.module.line(spec.Pos()))
					for _, member := range memberNames(spec.Type) {
						api.names[spec.Name.Name+"."+member] = true
					}
				case *ast.ValueSpec:
					for _, n := range spec.Names {
						if n.IsExported() {
							api.names[n.Name] = true
						}
					}
				}
			}
		}
	}
}

// receiverName returns the name of the type of a method receiver, without a
// pointer or type parameters.
func receiverName(expr ast.Expr) string {
	for {
		switch e := expr.(type) {
		case *ast.StarExpr:
			expr = e.X
		case *ast.ParenExpr:
			expr = e.X
		case *ast.IndexExpr:
			expr = e.X
		case *ast.IndexListExpr:
			expr = e.X
		case *ast.Ident:
			return e.Name
		default:
			return ""
		}
	}
}

// memberNames returns the exported fields of a struct type, or the exported
// methods of an interface type. An embedded field is named after its type.
func memberNames(expr ast.Expr) []string {
	var fields *ast.FieldList
	switch t := expr.(type) {
	case *ast.StructType:
		fields = t.Fields
	case *ast.InterfaceType:
		fields = t.Methods
	default:
		return nil
	}
	var names []string
	for _, field := range fields.List {
		if len(field.Names) == 0 {
			if name := receiverName(field.Type); token.IsExported(name) {
				names = append(names, name)
			}
			continue
		}
		for _, n := range field.Names {
			if n.IsExported() {
				names = append(names, n.Name)
			}
		}
	}
	return names
}

// tableLine is one line of a Markdown table, split into cells.
type tableLine struct {
	line  int
	cells []string
}

// markdownTables returns the tables of a Markdown text: runs of lines that
// start with "|", outside fenced code blocks.
func markdownTables(text string) [][]tableLine {
	var tables [][]tableLine
	var current []tableLine
	fence := ""
	flush := func() {
		if len(current) > 0 {
			tables = append(tables, current)
			current = nil
		}
	}
	for i, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if marker := fenceMarker(trimmed); marker != "" && (fence == "" || strings.HasPrefix(marker, fence)) {
			flush()
			if fence == "" {
				fence = marker
			} else {
				fence = ""
			}
			continue
		}
		if fence != "" || !strings.HasPrefix(trimmed, "|") {
			flush()
			continue
		}
		current = append(current, tableLine{line: i + 1, cells: splitRow(trimmed)})
	}
	flush()
	return tables
}

// fenceMarker returns the run of backticks or tildes that opens or closes a
// fenced code block, or "".
func fenceMarker(line string) string {
	for _, ch := range []string{"`", "~"} {
		n := 0
		for n < len(line) && line[n:n+1] == ch {
			n++
		}
		if n >= 3 {
			return line[:n]
		}
	}
	return ""
}

// splitRow splits a table row into trimmed cells, at each "|" that no
// backslash escapes.
func splitRow(row string) []string {
	row = strings.TrimPrefix(row, "|")
	if strings.HasSuffix(row, "|") && !strings.HasSuffix(row, "\\|") {
		row = strings.TrimSuffix(row, "|")
	}
	var cells []string
	var cell strings.Builder
	for i := 0; i < len(row); i++ {
		switch {
		case row[i] == '\\' && i+1 < len(row) && row[i+1] == '|':
			cell.WriteByte('|')
			i++
		case row[i] == '|':
			cells = append(cells, strings.TrimSpace(cell.String()))
			cell.Reset()
		default:
			cell.WriteByte(row[i])
		}
	}
	return append(cells, strings.TrimSpace(cell.String()))
}

// isDelimiterRow reports whether cells form the row under a header, such as
// |---|:--:|.
func isDelimiterRow(cells []string) bool {
	for _, cell := range cells {
		if strings.Trim(cell, "-:") != "" || !strings.Contains(cell, "-") {
			return false
		}
	}
	return len(cells) > 0
}

// tableRow writes cells as a row of a Markdown table.
func tableRow(cells []string) string {
	return "| " + strings.Join(cells, " | ") + " |"
}
