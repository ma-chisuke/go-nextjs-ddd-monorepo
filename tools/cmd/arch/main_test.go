package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// repository writes a backend with one context that keeps every rule into a
// new directory.
func repository(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"backend/go.mod": "module example.com/shop/backend\n\ngo 1.24\n",
		"backend/internal/shop/internal/app/app.go": "package app\n\n" +
			"// Order is what a customer buys.\ntype Order struct{}\n",
		"docs/contexts/shop/glossary.md": "# Shop glossary\n\n| Identifier | Term | Meaning |\n|---|---|---|\n" +
			"| `Order` | order | What a customer buys. |\n",
		"docs/contexts/shop/glossary.ja.md": "# shop の用語集\n\n| 識別子 | 用語 | 意味 |\n|---|---|---|\n" +
			"| `Order` | 注文 | 客が買うもの。 |\n",
	}
	for name, content := range files {
		write(t, dir, name, content)
	}
	return dir
}

func write(t *testing.T, dir, name, content string) {
	t.Helper()
	full := filepath.Join(dir, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// runArch runs the command and returns its exit status, stdout, and stderr.
func runArch(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := run(t.Context(), args, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func TestRun(t *testing.T) {
	good := repository(t)
	bad := repository(t)
	write(t, bad, "backend/internal/shop/internal/app/routes.go",
		"package app\n\nimport \"net/http\"\n\nfunc register() {\n\thttp.HandleFunc(\"/orders\", nil)\n}\n")
	tests := []struct {
		name       string
		args       []string
		wantCode   int
		wantStdout string // the whole of stdout
		wantStderr string // a substring of stderr
	}{
		{
			name: "a repository that keeps the rules passes",
			args: []string{"-root", good},
		},
		{
			name:     "a finding fails",
			args:     []string{"-root", bad},
			wantCode: exitFindings,
			wantStdout: "backend/internal/shop/internal/app/routes.go:6: [arch/route-registration] " +
				"this file registers an HTTP route with http.HandleFunc; routes come only from the code generated " +
				"from the API document and from backend/internal/platform/httpserver. " +
				"Fix: add the operation to the OpenAPI document of the context in contracts/http/ and run `make gen`, " +
				"so that the generated code registers it; if the call registers no HTTP route, give the method " +
				"another name. See TPL-0007.\n",
		},
		{
			name: "a repository without a backend passes",
			args: []string{"-root", t.TempDir()},
		},
		{
			name: "help is not an error",
			args: []string{"-h"},
			// The usage goes to stderr.
			wantStderr: "usage: arch",
		},
		{
			name:       "an argument is a usage error",
			args:       []string{"-root", good, "check"},
			wantCode:   exitError,
			wantStderr: `unexpected argument "check"`,
		},
		{
			name:       "an unknown flag is a usage error",
			args:       []string{"-verbose"},
			wantCode:   exitError,
			wantStderr: "flag provided but not defined",
		},
		{
			name:       "a missing root is an I/O error",
			args:       []string{"-root", filepath.Join(good, "missing")},
			wantCode:   exitError,
			wantStderr: "arch: ",
		},
		{
			name:       "a module that the go command cannot read is an error",
			args:       []string{"-root", brokenModule(t)},
			wantCode:   exitError,
			wantStderr: "go list in ",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, stdout, stderr := runArch(t, tt.args...)
			if code != tt.wantCode || stdout != tt.wantStdout || !strings.Contains(stderr, tt.wantStderr) {
				t.Errorf("exit %d, stdout %q, stderr %q; want exit %d, stdout %q, stderr with %q",
					code, stdout, stderr, tt.wantCode, tt.wantStdout, tt.wantStderr)
			}
		})
	}
}

// brokenModule returns a repository whose go.mod the go command rejects.
func brokenModule(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	write(t, dir, "backend/go.mod", "module example.com/shop/backend\n\ngo 1.24\n\nrequire (\n")
	return dir
}
