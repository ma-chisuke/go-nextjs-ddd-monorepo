package main

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A small copy of the template: the files where the names appear, and the
// files that init must leave alone.
var templateFiles = map[string]string{
	"tools/go.mod":     "module github.com/ma-chisuke/go-nextjs-ddd-monorepo/tools\n\ngo 1.27.4\n",
	"backend/main.go":  "package main\n\nimport _ \"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform\"\n",
	"package.json":     "{\n  \"name\": \"go-nextjs-ddd-monorepo\"\n}\n",
	"pyproject.toml":   "[project]\nname = \"go-nextjs-ddd-monorepo-tools\"\n",
	".golangci.yml":    "local-prefixes:\n  - github.com/ma-chisuke/go-nextjs-ddd-monorepo\n",
	".gitattributes":   "vendor/** linguist-vendored\n",
	"vendor/style.yml": "source: go-nextjs-ddd-monorepo\n",
	"docs/adr/template/TPL-0001-record-decisions-as-adrs.md": "# TPL-0001: in go-nextjs-ddd-monorepo\n",
	"tools/init/main.go":            "const templateModule = \"github.com/ma-chisuke/go-nextjs-ddd-monorepo\"\n",
	"CHANGELOG.md":                  "# Changelog\n\n## 1.2.3\n\n- go-nextjs-ddd-monorepo grew.\n",
	".release-please-manifest.json": "{\n  \".\": \"1.2.3\"\n}\n",
	"image.bin":                     "go-nextjs-ddd-monorepo\x00binary",
}

func newTemplate(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range templateFiles {
		file := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(file), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{
		{"init", "--quiet"},
		{"add", "--all"},
		{"-c", "user.name=test", "-c", "user.email=test@example.com", "-c", "commit.gpgsign=false", "commit", "--quiet", "--message", "template"},
	} {
		if _, err := git(t.Context(), dir, args...); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func initialized(t *testing.T, args ...string) (string, int, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := run(t.Context(), args, &stdout, &stderr)
	return stdout.String(), code, stderr.String()
}

func read(t *testing.T, dir, name string) string {
	t.Helper()
	data, err := fs.ReadFile(os.DirFS(dir), name)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func commitAll(t *testing.T, dir string) {
	t.Helper()
	for _, args := range [][]string{
		{"add", "--all"},
		{"-c", "user.name=test", "-c", "user.email=test@example.com", "-c", "commit.gpgsign=false", "commit", "--quiet", "--message", "init"},
	} {
		if _, err := git(t.Context(), dir, args...); err != nil {
			t.Fatal(err)
		}
	}
}

func TestInitRenamesTheProject(t *testing.T) {
	dir := newTemplate(t)
	out, code, stderr := initialized(t, "-C", dir, "-module", "github.com/acme/shop")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}

	want := map[string]string{
		"tools/go.mod":     "module github.com/acme/shop/tools\n\ngo 1.27.4\n",
		"backend/main.go":  "package main\n\nimport _ \"github.com/acme/shop/backend/internal/platform\"\n",
		"package.json":     "{\n  \"name\": \"shop\"\n}\n",
		"pyproject.toml":   "[project]\nname = \"shop-tools\"\n",
		".golangci.yml":    "local-prefixes:\n  - github.com/acme/shop\n",
		"CHANGELOG.md":     "# Changelog\n",
		"vendor/style.yml": templateFiles["vendor/style.yml"],
		"docs/adr/template/TPL-0001-record-decisions-as-adrs.md": templateFiles["docs/adr/template/TPL-0001-record-decisions-as-adrs.md"],
		"tools/init/main.go":            templateFiles["tools/init/main.go"],
		".release-please-manifest.json": "{\n  \".\": \"0.0.0\"\n}\n",
		"image.bin":                     templateFiles["image.bin"],
	}
	for name, content := range want {
		if got := read(t, dir, name); got != content {
			t.Errorf("%s:\n got %q\nwant %q", name, got, content)
		}
	}
	for _, line := range []string{"tools/go.mod", "CHANGELOG.md", "reset to 0.0.0", "Next:"} {
		if !strings.Contains(out, line) {
			t.Errorf("the report lacks %q:\n%s", line, out)
		}
	}
	if strings.Contains(out, "vendor/style.yml") || strings.Contains(out, "TPL-0001") || strings.Contains(out, "tools/init/") {
		t.Errorf("the report lists a file that init must keep:\n%s", out)
	}
}

func TestInitTwiceChangesNothing(t *testing.T) {
	for _, module := range []string{"github.com/acme/shop", "github.com/acme/go-nextjs-ddd-monorepo-fork"} {
		t.Run(module, func(t *testing.T) {
			dir := newTemplate(t)
			if _, code, stderr := initialized(t, "-C", dir, "-module", module); code != 0 {
				t.Fatalf("first run: exit %d: %s", code, stderr)
			}
			first := read(t, dir, "tools/go.mod")
			commitAll(t, dir)
			out, code, stderr := initialized(t, "-C", dir, "-module", module)
			if code != 0 {
				t.Fatalf("second run: exit %d: %s", code, stderr)
			}
			if !strings.HasPrefix(out, "Nothing to change") {
				t.Errorf("second run changed files:\n%s", out)
			}
			if got := read(t, dir, "tools/go.mod"); got != first || got != "module "+module+"/tools\n\ngo 1.27.4\n" {
				t.Errorf("tools/go.mod after two runs: %q", got)
			}
		})
	}
}

func TestInitKeepsAModulePathThatHoldsTheTemplateName(t *testing.T) {
	dir := newTemplate(t)
	if _, code, stderr := initialized(t, "-C", dir, "-module", "github.com/acme/go-nextjs-ddd-monorepo", "-name", "shop"); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	if got := read(t, dir, ".golangci.yml"); got != "local-prefixes:\n  - github.com/acme/go-nextjs-ddd-monorepo\n" {
		t.Errorf(".golangci.yml: %q", got)
	}
	if got := read(t, dir, "package.json"); got != "{\n  \"name\": \"shop\"\n}\n" {
		t.Errorf("package.json: %q", got)
	}
}

func TestInitDryRunWritesNothing(t *testing.T) {
	dir := newTemplate(t)
	out, code, stderr := initialized(t, "-C", dir, "-module", "github.com/acme/shop", "-dry-run")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	if !strings.HasPrefix(out, "Dry run") || !strings.Contains(out, "tools/go.mod") {
		t.Errorf("dry run report:\n%s", out)
	}
	if status, err := git(t.Context(), dir, "status", "--porcelain"); err != nil || status != "" {
		t.Errorf("dry run changed the working tree: %q %v", status, err)
	}
}

func TestInitRefusesUncommittedChanges(t *testing.T) {
	dir := newTemplate(t)
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("draft\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, code, stderr := initialized(t, "-C", dir, "-module", "github.com/acme/shop")
	if code == 0 || !strings.Contains(stderr, "uncommitted changes") {
		t.Errorf("exit %d, stderr %q; want a refusal", code, stderr)
	}
	if got := read(t, dir, "package.json"); got != templateFiles["package.json"] {
		t.Errorf("init changed package.json after refusing: %q", got)
	}
}

func TestInitRejectsBadArguments(t *testing.T) {
	dir := newTemplate(t)
	for _, args := range [][]string{
		{"-module", ""},
		{"-module", "shop"},
		{"-module", "github.com/acme/shop/"},
		{"-module", "github.com/acme/shop", "-name", "Shop App"},
	} {
		if _, code, _ := initialized(t, append([]string{"-C", dir}, args...)...); code == 0 {
			t.Errorf("%v: exit 0, want an error", args)
		}
	}
}
