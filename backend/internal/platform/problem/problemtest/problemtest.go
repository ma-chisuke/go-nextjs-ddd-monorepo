// Package problemtest holds the checks that every API test suite runs on its
// kinds of problem.
package problemtest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/problem"
)

// AssertDocumented fails the test for every kind whose page is missing. The
// URI of a kind leads to docs/problems/<slug>.md, and the page has a Japanese
// translation next to it (TPL-0008, TPL-0021).
func AssertDocumented(t *testing.T, kinds ...problem.Type) {
	t.Helper()
	dir := filepath.Join(repositoryRoot(t), "docs", "problems")
	seen := map[string]bool{}
	for _, kind := range kinds {
		if kind.Slug == "" || kind.Title == "" || kind.Status < 400 || kind.Status > 599 {
			t.Errorf("the kind %+v needs a slug, a title, and a status from 400 to 599", kind)
			continue
		}
		if seen[kind.Slug] {
			t.Errorf("the slug %q is used by two kinds of problem", kind.Slug)
		}
		seen[kind.Slug] = true
		for _, name := range []string{kind.Slug + ".md", kind.Slug + ".ja.md"} {
			if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
				t.Errorf("docs/problems/%s is missing: the problem %q needs a page that says what it means and what a client can do", name, kind.Slug)
			}
		}
	}
}

// repositoryRoot walks up from the working directory of the test to the
// directory that holds AGENTS.md.
func repositoryRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "AGENTS.md")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("problemtest: no AGENTS.md above the working directory, so the repository root is unknown")
		}
		dir = parent
	}
}
