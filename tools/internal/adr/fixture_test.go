package adr

import (
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// enRecord is a valid English record. Line numbers matter to the tests: the
// H1 is on line 6 (7 with a supersedes line), and the H2 sections start on
// lines 8, 12, 17, and 29.
const enRecord = `---
status: accepted
date: 2026-11-17
---

# {ID}: {TITLE}

## Context and Problem Statement

Why decide.

## Considered Options

- First option
- Second option

## Decision Outcome

Chosen option: "First option".

### Consequences

- Good, because it is simple.

### Confirmation

Review enforces it.

## Pros and Cons of the Options

### First option

- Good, because it is simple.

### Second option

- Bad, because it is complex.
`

// jaRecord is the translation of enRecord, line for line.
const jaRecord = `---
status: accepted
date: 2026-11-17
---

# {ID}: {TITLE}

## 背景と課題

なぜ決めるか。

## 検討した選択肢

- 1つ目の選択肢
- 2つ目の選択肢

## 決定

採用：「1つ目の選択肢」。

### 結果

- 良い点：単純である。

### 確認の方法

レビューで守る。

## 選択肢の長所と短所

### 1つ目の選択肢

- 良い点：単純である。

### 2つ目の選択肢

- 悪い点：複雑である。
`

// Paths of the fixture.
const (
	tpl1   = "docs/adr/template/TPL-0001-format"
	tpl2   = "docs/adr/template/TPL-0002-use-sqlite"
	adr1   = "docs/adr/ADR-0001-use-postgresql"
	tpl1EN = tpl1 + ".md"
	tpl1JA = tpl1 + ".ja.md"
	adr1EN = adr1 + ".md"
	adr1JA = adr1 + ".ja.md"
)

// recordText fills a record template with an ID, a title, and extra front
// matter lines (such as "supersedes: [TPL-0002]").
func recordText(text, id, title, extra string) string {
	text = strings.ReplaceAll(text, "{ID}", id)
	text = strings.ReplaceAll(text, "{TITLE}", title)
	if extra != "" {
		text = strings.Replace(text, "date: 2026-11-17\n", "date: 2026-11-17\n"+extra+"\n", 1)
	}
	return text
}

// fixtureFiles returns two template records and one project record that
// supersedes the second template record, each with a translation.
func fixtureFiles() map[string]string {
	return map[string]string{
		tpl1EN:          recordText(enRecord, "TPL-0001", "Record decisions", ""),
		tpl1JA:          recordText(jaRecord, "TPL-0001", "決定を記録する", ""),
		tpl2 + ".md":    recordText(enRecord, "TPL-0002", "Use SQLite", ""),
		tpl2 + ".ja.md": recordText(jaRecord, "TPL-0002", "SQLite を使う", ""),
		adr1EN:          recordText(enRecord, "ADR-0001", "Use PostgreSQL", "supersedes: [TPL-0002]"),
		adr1JA:          recordText(jaRecord, "ADR-0001", "PostgreSQL を使う", "supersedes: [TPL-0002]"),
	}
}

// writeFile writes one file under dir, creating its directory.
func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	full := filepath.Join(dir, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// openRoot opens dir as an os.Root that is closed when the test ends.
func openRoot(t *testing.T, dir string) *os.Root {
	t.Helper()
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = root.Close() })
	return root
}

// readFile reads one file under dir.
func readFile(t *testing.T, dir, name string) string {
	t.Helper()
	data, err := fs.ReadFile(os.DirFS(dir), name)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// newRepo writes the fixture, its index, and its checksums into a new
// directory.
func newRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range fixtureFiles() {
		writeFile(t, dir, name, content)
	}
	writeIndex(t, dir)
	seal(t, dir)
	return dir
}

func writeIndex(t *testing.T, dir string) {
	t.Helper()
	if err := WriteIndex(openRoot(t, dir), io.Discard); err != nil {
		t.Fatal(err)
	}
}

func seal(t *testing.T, dir string) {
	t.Helper()
	if err := Seal(openRoot(t, dir), io.Discard); err != nil {
		t.Fatal(err)
	}
}

// check runs Check on dir and returns "path:line: [rule]" of each finding.
func check(t *testing.T, dir string) []string {
	t.Helper()
	findings, err := Check(os.DirFS(dir))
	if err != nil {
		t.Fatalf("Check() error: %v", err)
	}
	got := make([]string, 0, len(findings))
	for _, f := range findings {
		s := f.String()
		got = append(got, s[:strings.Index(s, "]")+1])
	}
	slices.Sort(got)
	return got
}

// change edits the fixture of one test case.
type change func(t *testing.T, dir string)

// put writes a file.
func put(name, content string) change {
	return func(t *testing.T, dir string) {
		t.Helper()
		writeFile(t, dir, name, content)
	}
}

// remove deletes a file.
func remove(name string) change {
	return func(t *testing.T, dir string) {
		t.Helper()
		if err := os.Remove(filepath.Join(dir, filepath.FromSlash(name))); err != nil {
			t.Fatal(err)
		}
	}
}

// edit replaces the first old with replacement in a file, and fails when the
// file does not contain old.
func edit(name, old, replacement string) change {
	return func(t *testing.T, dir string) {
		t.Helper()
		text := readFile(t, dir, name)
		if !strings.Contains(text, old) {
			t.Fatalf("%s does not contain %q", name, old)
		}
		writeFile(t, dir, name, strings.Replace(text, old, replacement, 1))
	}
}

// all applies several changes in order.
func all(changes ...change) change {
	return func(t *testing.T, dir string) {
		t.Helper()
		for _, c := range changes {
			c(t, dir)
		}
	}
}
