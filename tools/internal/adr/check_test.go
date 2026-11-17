package adr

import (
	"os"
	"slices"
	"strings"
	"testing"
)

const (
	cks = checksumsPath
	// tpl3 is a record that the fixture does not have.
	tpl3 = "docs/adr/template/TPL-0003-use-redis"
	adr2 = "docs/adr/ADR-0002-use-go-1-27"
)

// both makes the same edit to the English file and the translation of a
// record (given without ".md").
func both(base, old, replacement string) change {
	return all(edit(base+".md", old, replacement), edit(base+".ja.md", old, replacement))
}

// appendTo adds text at the end of a file.
func appendTo(name, text string) change {
	return func(t *testing.T, dir string) {
		t.Helper()
		writeFile(t, dir, name, readFile(t, dir, name)+text)
	}
}

// newRecord adds a record with a translation.
func newRecord(base, id, extra string) change {
	return all(
		put(base+".md", recordText(enRecord, id, "A new decision", extra)),
		put(base+".ja.md", recordText(jaRecord, id, "新しい決定", extra)),
	)
}

func TestCheck(t *testing.T) {
	const (
		enConsidered   = "## Considered Options\n\n- First option\n- Second option\n\n"
		jaConsidered   = "## 検討した選択肢\n\n- 1つ目の選択肢\n- 2つ目の選択肢\n\n"
		enConsequences = "### Consequences\n\n- Good, because it is simple.\n\n"
		enConfirmation = "### Confirmation\n\nReview enforces it.\n"
		enProsCons     = "## Pros and Cons of the Options\n\n### First option\n\n- Good, because it is simple.\n\n" +
			"### Second option\n\n- Bad, because it is complex.\n"
		jaProsCons = "## 選択肢の長所と短所\n\n### 1つ目の選択肢\n\n- 良い点：単純である。\n\n" +
			"### 2つ目の選択肢\n\n- 悪い点：複雑である。\n"
		enDrivers = "## Decision Drivers\n\n- Speed.\n\n"
		jaDrivers = "## 判断の基準\n\n- 速さ。\n\n"
		enLast    = "- Bad, because it is complex.\n"
		jaLast    = "- 悪い点：複雑である。\n"
		front     = "---\nstatus: accepted\ndate: 2026-11-17\n---\n\n"
	)
	tests := []struct {
		name   string
		change change
		// keepIndex and keepChecksums leave the generated files as they were
		// before the change; otherwise the test regenerates them.
		keepIndex     bool
		keepChecksums bool
		want          []string
	}{
		{name: "the fixture passes, with number 0001 in both series"},

		// adr/unexpected-file
		{
			name:   "unexpected-file fires on a file that is not Markdown",
			change: put("docs/adr/notes.txt", "notes\n"),
			want:   []string{"docs/adr/notes.txt:1: [adr/unexpected-file]"},
		},
		{
			name:   "unexpected-file fires on a directory",
			change: put("docs/adr/images/diagram.svg", "<svg/>\n"),
			want:   []string{"docs/adr/images:1: [adr/unexpected-file]"},
		},
		{
			name:   "unexpected-file fires on checksums.txt outside the template",
			change: put("docs/adr/checksums.txt", ""),
			want:   []string{"docs/adr/checksums.txt:1: [adr/unexpected-file]"},
		},
		{
			name:   "unexpected-file passes READMEs and files of the local machine",
			change: all(put("docs/adr/.DS_Store", ""), put("docs/adr/template/README.md", "# Template\n")),
		},

		// adr/filename
		{
			name:   "filename fires on a slug that is not lowercase",
			change: put("docs/adr/ADR-0002-Use_Go.md", recordText(enRecord, "ADR-0002", "Use Go", "")),
			want:   []string{"docs/adr/ADR-0002-Use_Go.md:1: [adr/filename]"},
		},
		{
			name:   "filename fires on a name without an ID",
			change: put("docs/adr/notes.md", "# Notes\n"),
			want:   []string{"docs/adr/notes.md:1: [adr/filename]"},
		},
		{
			name:   "filename fires on a short number",
			change: put("docs/adr/ADR-12-short.md", recordText(enRecord, "ADR-12", "Short", "")),
			want:   []string{"docs/adr/ADR-12-short.md:1: [adr/filename]"},
		},
		{
			name:   "filename fires on number 0000",
			change: put("docs/adr/ADR-0000-zero.md", recordText(enRecord, "ADR-0000", "Zero", "")),
			want:   []string{"docs/adr/ADR-0000-zero.md:1: [adr/filename]"},
		},
		{
			name:   "filename fires on a template record among the project records",
			change: put("docs/adr/TPL-0003-misplaced.md", recordText(enRecord, "TPL-0003", "Misplaced", "")),
			want:   []string{"docs/adr/TPL-0003-misplaced.md:1: [adr/filename]"},
		},
		{
			name:   "filename fires on a project record among the template records",
			change: put("docs/adr/template/ADR-0002-misplaced.md", recordText(enRecord, "ADR-0002", "Misplaced", "")),
			want:   []string{"docs/adr/template/ADR-0002-misplaced.md:1: [adr/filename]"},
		},
		{
			name:   "filename passes a slug with digits",
			change: newRecord(adr2, "ADR-0002", ""),
		},

		// adr/duplicate-id
		{
			name:   "duplicate-id fires on two records with one ID",
			change: newRecord("docs/adr/ADR-0001-use-mysql", "ADR-0001", ""),
			want: []string{
				"docs/adr/ADR-0001-use-mysql.md:1: [adr/duplicate-id]",
				"docs/adr/ADR-0001-use-postgresql.md:1: [adr/duplicate-id]",
			},
		},

		// adr/front-matter
		{
			name:   "front-matter fires when there is none",
			change: both(tpl1, front, ""),
			want:   []string{tpl1EN + ":1: [adr/front-matter]"},
		},
		{
			name:   "front-matter fires when it is not closed",
			change: both(tpl1, "date: 2026-11-17\n---\n", "date: 2026-11-17\n"),
			want:   []string{tpl1EN + ":1: [adr/front-matter]"},
		},
		{
			name:   "front-matter fires on a line that is not key: value",
			change: both(tpl1, "date: 2026-11-17\n", "date: 2026-11-17\nauthor:\n"),
			want:   []string{tpl1EN + ":4: [adr/front-matter]"},
		},
		{
			name:   "front-matter fires on a blank line",
			change: both(tpl1, "status: accepted\n", "status: accepted\n\n"),
			want:   []string{tpl1EN + ":3: [adr/front-matter]"},
		},
		{
			name:   "front-matter fires on an unknown key",
			change: both(tpl1, "date: 2026-11-17\n", "date: 2026-11-17\nauthor: Alice\n"),
			want:   []string{tpl1EN + ":4: [adr/front-matter]"},
		},
		{
			name:   "front-matter fires on a repeated key",
			change: both(tpl1, "date: 2026-11-17\n", "date: 2026-11-17\ndate: 2026-11-18\n"),
			want:   []string{tpl1EN + ":4: [adr/front-matter]"},
		},
		{
			name:   "front-matter fires on a missing date",
			change: both(tpl1, "date: 2026-11-17\n", ""),
			want:   []string{tpl1EN + ":1: [adr/front-matter]"},
		},

		// adr/status
		{
			name:   "status fires on an unknown status",
			change: both(tpl1, "status: accepted", "status: done"),
			want:   []string{tpl1EN + ":2: [adr/status]"},
		},
		{name: "status passes proposed", change: both(tpl1, "status: accepted", "status: proposed")},
		{name: "status passes rejected", change: both(tpl1, "status: accepted", "status: rejected")},
		{name: "status passes deprecated", change: both(tpl1, "status: accepted", "status: deprecated")},

		// adr/date
		{
			name:   "date fires on a day that does not exist",
			change: both(tpl1, "date: 2026-11-17", "date: 2026-02-30"),
			want:   []string{tpl1EN + ":3: [adr/date]"},
		},
		{
			name:   "date fires on a date without leading zeros",
			change: both(tpl1, "date: 2026-11-17", "date: 2026-1-5"),
			want:   []string{tpl1EN + ":3: [adr/date]"},
		},
		{name: "date passes a leap day", change: both(tpl1, "date: 2026-11-17", "date: 2024-02-29")},

		// adr/supersedes
		{
			name:   "supersedes fires on a record that does not exist",
			change: both(adr1, "[TPL-0002]", "[TPL-0099]"),
			want:   []string{adr1EN + ":4: [adr/supersedes]"},
		},
		{
			name:   "supersedes fires on the record itself",
			change: both(adr1, "[TPL-0002]", "[ADR-0001]"),
			want:   []string{adr1EN + ":4: [adr/supersedes]"},
		},
		{
			name:   "supersedes fires on a repeated ID",
			change: both(adr1, "[TPL-0002]", "[TPL-0002, TPL-0002]"),
			want:   []string{adr1EN + ":4: [adr/supersedes]"},
		},
		{
			name:   "supersedes fires on a value that is not a list",
			change: both(adr1, "[TPL-0002]", "TPL-0002"),
			want:   []string{adr1EN + ":4: [adr/supersedes]"},
		},
		{
			name:   "supersedes fires on a list without spaces",
			change: both(adr1, "[TPL-0002]", "[TPL-0002,TPL-0001]"),
			want:   []string{adr1EN + ":4: [adr/supersedes]"},
		},
		{
			name:   "supersedes fires on an empty list",
			change: both(adr1, "[TPL-0002]", "[]"),
			want:   []string{adr1EN + ":4: [adr/supersedes]"},
		},
		{
			name:   "supersedes fires on an ID that is not an ID",
			change: both(adr1, "[TPL-0002]", "[tpl-0002]"),
			want:   []string{adr1EN + ":4: [adr/supersedes]"},
		},
		{
			name:   "supersedes passes two records of both series",
			change: all(both(adr1, "[TPL-0002]", "[TPL-0002, TPL-0001]"), newRecord(adr2, "ADR-0002", "supersedes: [ADR-0001]")),
		},

		// adr/title
		{
			name:   "title fires when there is no H1",
			change: edit(tpl1EN, "# TPL-0001: Record decisions\n\n", ""),
			want:   []string{tpl1EN + ":1: [adr/title]"},
		},
		{
			name:   "title fires on a second H1",
			change: edit(tpl1EN, "Why decide.\n", "Why decide.\n\n# Another title\n"),
			want:   []string{tpl1EN + ":12: [adr/title]"},
		},
		{
			name:   "title fires on the ID of another record",
			change: edit(tpl1EN, "# TPL-0001:", "# TPL-0002:"),
			want:   []string{tpl1EN + ":6: [adr/title]"},
		},
		{
			name:   "title fires on an empty title",
			change: edit(tpl1EN, "# TPL-0001: Record decisions", "# TPL-0001:"),
			want:   []string{tpl1EN + ":6: [adr/title]"},
		},
		{
			name:   "title fires without the colon",
			change: edit(tpl1EN, "# TPL-0001: Record", "# TPL-0001 Record"),
			want:   []string{tpl1EN + ":6: [adr/title]"},
		},
		{
			name:   "title fires on the translation",
			change: edit(tpl1JA, "# TPL-0001:", "# ADR-0001:"),
			want:   []string{tpl1JA + ":6: [adr/title]"},
		},
		{
			name: "title and sections pass headings inside fenced code blocks",
			change: edit(tpl1EN, "Why decide.\n", "Why decide.\n\n```markdown\n# Not a title\n\n## Not a section\n```\n\n"+
				"~~~~\n```\n# Still code\n~~~~\n"),
		},

		// adr/sections
		{
			name:   "sections fires on a missing section",
			change: all(edit(tpl1EN, enConsidered, ""), edit(tpl1JA, jaConsidered, "")),
			want:   []string{tpl1JA + ":6: [adr/sections]", tpl1EN + ":6: [adr/sections]"},
		},
		{
			name:   "sections fires on a section out of order",
			change: edit(tpl1EN, "## Decision Outcome", enDrivers+"## Decision Outcome"),
			want:   []string{tpl1EN + ":17: [adr/sections]"},
		},
		{
			name:   "sections fires on an unknown section",
			change: edit(tpl1EN, "## Context and Problem Statement", "## Background"),
			want:   []string{tpl1EN + ":6: [adr/sections]", tpl1EN + ":8: [adr/sections]"},
		},
		{
			name:   "sections fires on a repeated section",
			change: edit(tpl1EN, "## Decision Outcome", "## Considered Options\n\n## Decision Outcome"),
			want:   []string{tpl1EN + ":17: [adr/sections]"},
		},
		{
			name:   "sections fires on a missing Consequences",
			change: edit(tpl1EN, enConsequences, ""),
			want:   []string{tpl1EN + ":17: [adr/sections]"},
		},
		{
			name:   "sections fires on Confirmation before Consequences",
			change: edit(tpl1EN, enConsequences+enConfirmation, enConfirmation+"\n"+strings.TrimSuffix(enConsequences, "\n")),
			want:   []string{tpl1EN + ":25: [adr/sections]"},
		},
		{
			name:   "sections fires on another H3 under Decision Outcome",
			change: edit(tpl1EN, "### Confirmation", "### Rules\n\n### Confirmation"),
			want:   []string{tpl1EN + ":25: [adr/sections]"},
		},
		{
			name:   "sections fires on an English heading in the translation",
			change: edit(tpl1JA, "## 背景と課題", "## Context and Problem Statement"),
			want:   []string{tpl1JA + ":6: [adr/sections]", tpl1JA + ":8: [adr/sections]"},
		},
		{
			name:   "sections fires on a missing 確認の方法 in the translation",
			change: edit(tpl1JA, "### 確認の方法\n\nレビューで守る。\n\n", ""),
			want:   []string{tpl1JA + ":17: [adr/sections]"},
		},
		{
			name:   "sections passes without the optional sections",
			change: all(edit(tpl1EN, enProsCons, ""), edit(tpl1JA, jaProsCons, "")),
		},
		{
			name: "sections passes every optional section",
			change: all(
				edit(tpl1EN, "## Considered Options", enDrivers+"## Considered Options"),
				edit(tpl1EN, enLast, enLast+"\n## More Information\n\n- A link.\n"),
				edit(tpl1JA, "## 検討した選択肢", jaDrivers+"## 検討した選択肢"),
				edit(tpl1JA, jaLast, jaLast+"\n## 補足\n\n- リンク。\n"),
			),
		},
		{
			name:   "sections passes an H4 under Decision Outcome",
			change: edit(tpl1EN, enConsequences+"### Confirmation", enConsequences+"#### Detail\n\n### Confirmation"),
		},

		// adr/ja-front-matter
		{
			name:   "ja-front-matter fires on another date",
			change: edit(tpl1JA, "date: 2026-11-17", "date: 2026-11-18"),
			want:   []string{tpl1JA + ":3: [adr/ja-front-matter]"},
		},
		{
			name:   "ja-front-matter fires when the translation has none",
			change: edit(tpl1JA, front, ""),
			want:   []string{tpl1JA + ":1: [adr/ja-front-matter]"},
		},

		// adr/ja-sections
		{
			name:   "ja-sections fires on a section that the translation lacks",
			change: edit(tpl1EN, "## Considered Options", enDrivers+"## Considered Options"),
			want:   []string{tpl1JA + ":12: [adr/ja-sections]"},
		},
		{
			name:   "ja-sections fires on a section that only the translation has",
			change: edit(tpl1JA, jaLast, jaLast+"\n## 補足\n\n- リンク。\n"),
			want:   []string{tpl1JA + ":39: [adr/ja-sections]"},
		},
		{
			name:   "ja-sections fires on fewer options in the translation",
			change: edit(tpl1JA, "\n### 2つ目の選択肢\n\n"+jaLast, ""),
			want:   []string{tpl1JA + ":29: [adr/ja-sections]"},
		},

		// adr/stale-index
		{
			name:      "stale-index fires on an edited index",
			change:    edit(indexEN, "# Architecture decision records", "# Decisions"),
			keepIndex: true,
			want:      []string{indexEN + ":1: [adr/stale-index]"},
		},
		{
			name:      "stale-index fires on a missing index",
			change:    remove(indexJA),
			keepIndex: true,
			want:      []string{indexJA + ":1: [adr/stale-index]"},
		},
		{
			name:      "stale-index fires on a changed status",
			change:    both(adr1, "status: accepted", "status: deprecated"),
			keepIndex: true,
			want:      []string{indexJA + ":11: [adr/stale-index]", indexEN + ":11: [adr/stale-index]"},
		},
		{
			name:      "stale-index fires on a changed Japanese title",
			change:    edit(adr1JA, "PostgreSQL を使う", "PostgreSQL を採る"),
			keepIndex: true,
			want:      []string{indexJA + ":11: [adr/stale-index]"},
		},
		{
			name:   "stale-index passes a new record after make gen",
			change: newRecord(adr2, "ADR-0002", ""),
		},

		// adr/template-changed
		{
			name:          "template-changed fires on an edited template record",
			change:        edit(tpl1EN, "Why decide.", "Why we decide."),
			keepChecksums: true,
			want:          []string{tpl1EN + ":1: [adr/template-changed]"},
		},
		{
			name:          "template-changed fires on a new template record",
			change:        newRecord(tpl3, "TPL-0003", ""),
			keepChecksums: true,
			want:          []string{tpl3 + ".ja.md:1: [adr/template-changed]", tpl3 + ".md:1: [adr/template-changed]"},
		},
		{
			name:          "template-changed fires on a deleted template record",
			change:        all(remove(tpl1EN), remove(tpl1JA)),
			keepChecksums: true,
			want:          []string{cks + ":1: [adr/template-changed]", cks + ":2: [adr/template-changed]"},
		},
		{
			name:          "template-changed passes an edited project record",
			change:        edit(adr1EN, "Why decide.", "Why we decide."),
			keepChecksums: true,
		},
		{
			name:   "template-changed passes an edited template record after make adr-seal",
			change: edit(tpl1EN, "Why decide.", "Why we decide."),
		},

		// adr/checksums-format
		{
			name:          "checksums-format fires on a line that is not sha256sum",
			change:        appendTo(cks, "garbage\n"),
			keepChecksums: true,
			want:          []string{cks + ":5: [adr/checksums-format]"},
		},
		{
			name: "checksums-format fires on unsorted lines",
			change: func(t *testing.T, dir string) {
				t.Helper()
				lines := strings.SplitAfter(readFile(t, dir, cks), "\n")
				lines[0], lines[1] = lines[1], lines[0]
				writeFile(t, dir, cks, strings.Join(lines, ""))
			},
			keepChecksums: true,
			want:          []string{cks + ":2: [adr/checksums-format]"},
		},
		{
			name: "checksums-format fires on a repeated file",
			change: func(t *testing.T, dir string) {
				t.Helper()
				text := readFile(t, dir, cks)
				writeFile(t, dir, cks, text+strings.SplitAfter(text, "\n")[0])
			},
			keepChecksums: true,
			want:          []string{cks + ":5: [adr/checksums-format]"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := newRepo(t)
			if tt.change != nil {
				tt.change(t, dir)
			}
			if !tt.keepIndex {
				writeIndex(t, dir)
			}
			if !tt.keepChecksums {
				seal(t, dir)
			}
			if got := check(t, dir); !slices.Equal(got, tt.want) {
				t.Errorf("findings:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(tt.want, "\n"))
			}
		})
	}
}

func TestCheckMessages(t *testing.T) {
	dir := newRepo(t)
	edit(tpl1EN, "Why decide.", "Why we decide.")(t, dir)
	findings, err := Check(os.DirFS(dir))
	if err != nil {
		t.Fatal(err)
	}
	want := tpl1EN + ":1: [adr/template-changed] this template record differs from its checksum in " + cks + ". " +
		"Fix: if you use the template, do not edit template records: restore the file, and write a project record in " +
		"docs/adr/ whose front matter has `supersedes: [TPL-0001]`; if you maintain the template and edited the file " +
		"on purpose, run `make adr-seal`. See TPL-0001."
	if len(findings) != 1 || findings[0].String() != want {
		t.Errorf("findings = %v\nwant %s", findings, want)
	}
}

func TestCheckEmptyRepository(t *testing.T) {
	got := check(t, t.TempDir())
	want := []string{indexJA + ":1: [adr/stale-index]", indexEN + ":1: [adr/stale-index]"}
	if !slices.Equal(got, want) {
		t.Errorf("findings = %q, want %q", got, want)
	}
}
