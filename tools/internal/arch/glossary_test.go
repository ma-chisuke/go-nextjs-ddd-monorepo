package arch

import (
	"slices"
	"testing"
)

// both makes the same edit to both files of the booking glossary.
func both(old, replacement string) change {
	return all(edit(bookingGlossary, old, replacement), edit(bookingJA, old, replacement))
}

// rows adds rows to the end of the table in both files of the booking
// glossary.
func rows(text string) change {
	return all(
		edit(bookingGlossary, "| `New` | book | Book a room \\| or try to. |\n", "| `New` | book | Book a room \\| or try to. |\n"+text),
		edit(bookingJA, "| `New` | 予約する | 部屋を予約する。 |\n", "| `New` | 予約する | 部屋を予約する。 |\n"+text),
	)
}

func TestCheckGlossaries(t *testing.T) {
	runCases(t, []testCase{
		// The glossary and its translation exist.
		{
			name:   "glossary fires on a missing glossary",
			change: remove(bookingGlossary),
			want:   []string{bookingGlossary + ":1: [arch/glossary]"},
		},
		{
			name:   "glossary fires on a missing translation",
			change: remove(bookingJA),
			want:   []string{bookingJA + ":1: [arch/glossary]"},
		},
		{
			name:   "glossary fires on a new context without documents",
			change: put("backend/internal/billing/internal/app/invoice.go", "package app\n\n// Invoice is a bill.\ntype Invoice struct{}\n"),
			want: []string{
				"docs/contexts/billing/glossary.ja.md:1: [arch/glossary]",
				"docs/contexts/billing/glossary.md:1: [arch/glossary]",
			},
		},

		// Each file is one table.
		{
			name:   "glossary fires on a file without a table",
			change: put(bookingGlossary, "# Booking glossary\n\nNo table yet.\n"),
			want:   []string{bookingGlossary + ":1: [arch/glossary]"},
		},
		{
			name:   "glossary fires on a second table",
			change: edit(bookingGlossary, "or try to. |\n", "or try to. |\n\n| A | B |\n|---|---|\n"),
			want:   []string{bookingGlossary + ":13: [arch/glossary]"},
		},
		{
			name:   "glossary fires on a wrong header",
			change: edit(bookingGlossary, "| Identifier | Term | Meaning |", "| Name | Term | Meaning |"),
			want:   []string{bookingGlossary + ":5: [arch/glossary]"},
		},
		{
			name:   "glossary fires on an English header in the translation",
			change: edit(bookingJA, "| 識別子 | 用語 | 意味 |", "| Identifier | Term | Meaning |"),
			want:   []string{bookingJA + ":3: [arch/glossary]"},
		},
		{
			name:   "glossary fires on a missing delimiter row",
			change: edit(bookingGlossary, "|---|---|---|\n", ""),
			want:   []string{bookingGlossary + ":5: [arch/glossary]"},
		},
		{
			name:   "glossary fires on a row with too few cells, and still reads its identifier",
			change: edit(bookingGlossary, "| `Status` | status | Where a booking stands. |", "| `Status` | status |"),
			want:   []string{bookingGlossary + ":9: [arch/glossary]"},
		},
		{
			name:   "glossary fires on an identifier without backticks, and compares nothing until it is fixed",
			change: edit(bookingGlossary, "| `Status` |", "| Status |"),
			want:   []string{bookingGlossary + ":9: [arch/glossary]"},
		},
		{
			name:   "glossary fires on an identifier that is not exported",
			change: both("| `Status` |", "| `status` |"),
			want:   []string{bookingJA + ":7: [arch/glossary]", bookingGlossary + ":9: [arch/glossary]"},
		},
		{
			name:   "glossary fires on an empty term",
			change: edit(bookingGlossary, "| `Status` | status |", "| `Status` |  |"),
			want:   []string{bookingGlossary + ":9: [arch/glossary]"},
		},
		{
			name:   "glossary fires on an identifier listed twice",
			change: edit(bookingGlossary, "or try to. |\n", "or try to. |\n| `Booking` | booking | Again. |\n"),
			want:   []string{bookingGlossary + ":12: [arch/glossary]"},
		},

		// The English glossary matches the code.
		{
			name:   "glossary fires on an exported type that it lacks",
			change: put(bookingDomain+"/room.go", "package domain\n\n// Room is a place.\ntype Room struct{}\n\ntype hidden struct{}\n"),
			want:   []string{bookingGlossary + ":5: [arch/glossary]"},
		},
		{
			name:   "glossary fires on an identifier that the code does not export",
			change: both("`ErrPast`", "`ErrLate`"),
			want:   []string{bookingGlossary + ":10: [arch/glossary]"},
		},
		{
			name:   "glossary fires on a field that the type does not have",
			change: both("`Booking.Room`", "`Booking.Price`"),
			want:   []string{bookingGlossary + ":8: [arch/glossary]"},
		},
		{
			name: "glossary passes methods, fields, embedded fields, interface methods, and generic types",
			change: all(
				put(bookingDomain+"/policy.go", "package domain\n\n"+
					"// Policy decides.\ntype Policy interface {\n\tAllows(b Booking) bool\n}\n\n"+
					"// Rule is a named policy.\ntype Rule struct {\n\tPolicy\n\tName string\n\tnote string\n}\n\n"+
					"// Set is a set.\ntype Set[T comparable] map[T]bool\n\n"+
					"// Has reports whether v is in s.\nfunc (s Set[T]) Has(v T) bool { return s[v] }\n"),
				rows("| `Booking.Expires` | expiry | When a booking lapses. |\n"+
					"| `Policy` | policy | A rule. |\n| `Policy.Allows` | allow | Say yes. |\n"+
					"| `Rule` | rule | A named policy. |\n| `Rule.Policy` | policy | Its policy. |\n"+
					"| `Rule.Name` | name | Its name. |\n| `Set` | set | A set. |\n| `Set.Has` | has | Is in. |\n"),
			),
		},
		{
			name:   "glossary fires on an unexported field",
			change: rows("| `Room.created` | created | When. |\n"),
			want:   []string{bookingJA + ":10: [arch/glossary]", bookingGlossary + ":12: [arch/glossary]"},
		},

		// The translation lists the same identifiers.
		{
			name:   "glossary fires on a row that the translation lacks",
			change: edit(bookingJA, "| `Status` | 状態 | 予約の状態。 |\n", ""),
			want:   []string{bookingJA + ":3: [arch/glossary]"},
		},
		{
			name:   "glossary fires on a row that only the translation has",
			change: edit(bookingJA, "| `New` | 予約する | 部屋を予約する。 |\n", "| `New` | 予約する | 部屋を予約する。 |\n| `Booking.Period` | 期間 | 予約の期間。 |\n"),
			want:   []string{bookingJA + ":10: [arch/glossary]"},
		},
	})
}

func TestSplitRow(t *testing.T) {
	tests := []struct {
		row  string
		want []string
	}{
		{row: "| a | b |", want: []string{"a", "b"}},
		{row: "|a|b", want: []string{"a", "b"}},
		{row: `| a \| b | c |`, want: []string{"a | b", "c"}},
		{row: `| a | b \|`, want: []string{"a", "b |"}},
		{row: "|  |", want: []string{""}},
	}
	for _, tt := range tests {
		if got := splitRow(tt.row); !slices.Equal(got, tt.want) {
			t.Errorf("splitRow(%q) = %q, want %q", tt.row, got, tt.want)
		}
	}
}

func TestMarkdownTables(t *testing.T) {
	text := "# T\n\n| a |\n|---|\n\n~~~~\n| code |\n```\n| still code |\n~~~~\n\n| b |\n"
	tables := markdownTables(text)
	if len(tables) != 2 || tables[0][0].line != 3 || len(tables[0]) != 2 || tables[1][0].line != 12 {
		t.Errorf("markdownTables() = %v, want tables on lines 3 (two lines) and 12", tables)
	}
}
