package arch

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// module is the Go module of the backend, with its packages parsed.
type module struct {
	path     string
	fset     *token.FileSet
	packages []*goPackage
}

// goPackage is one package of the backend module.
type goPackage struct {
	// dir is the directory, relative to the root of the repository.
	dir   string
	loc   location
	name  string
	files []*goFile
}

// goFile is one Go file of a package.
type goFile struct {
	// path is relative to the root of the repository.
	path      string
	test      bool
	generated bool
	syntax    *ast.File
}

// listedPackage is the part of the output of `go list -json` that the rules
// read.
type listedPackage struct {
	ImportPath     string   `json:"ImportPath"`
	Name           string   `json:"Name"`
	GoFiles        []string `json:"GoFiles"`
	CgoFiles       []string `json:"CgoFiles"`
	TestGoFiles    []string `json:"TestGoFiles"`
	XTestGoFiles   []string `json:"XTestGoFiles"`
	IgnoredGoFiles []string `json:"IgnoredGoFiles"`
}

// files returns every Go file of the package, including the test files and
// the files that build constraints exclude.
func (l *listedPackage) files() []string {
	return slices.Concat(l.GoFiles, l.CgoFiles, l.TestGoFiles, l.XTestGoFiles, l.IgnoredGoFiles)
}

// loadModule lists the packages of the backend module with the go command,
// and parses their files. dir is the root of the repository, and fsys reads
// it.
func loadModule(ctx context.Context, dir string, fsys fs.FS) (*module, error) {
	modulePath, err := readModulePath(fsys)
	if err != nil {
		return nil, err
	}
	listed, err := goList(ctx, filepath.Join(dir, backendDir))
	if err != nil {
		return nil, err
	}
	// go list leaves out a directory whose files the build constraints
	// exclude, such as one that holds only integration tests. The rules read
	// those files too.
	more, err := unlistedPackages(fsys, modulePath, listed)
	if err != nil {
		return nil, err
	}
	m := &module{path: modulePath, fset: token.NewFileSet()}
	for _, l := range slices.Concat(listed, more) {
		rel, ok := m.relative(l.ImportPath)
		if !ok {
			continue
		}
		p := &goPackage{dir: path.Join(backendDir, rel), loc: locate(rel), name: l.Name}
		if err := m.parse(fsys, p, l.files()); err != nil {
			return nil, err
		}
		m.packages = append(m.packages, p)
	}
	slices.SortFunc(m.packages, func(a, b *goPackage) int { return strings.Compare(a.dir, b.dir) })
	return m, nil
}

// relative returns an import path relative to the root of the module, and
// whether the path is in the module at all.
func (m *module) relative(importPath string) (string, bool) {
	if importPath == m.path {
		return "", true
	}
	rel, ok := strings.CutPrefix(importPath, m.path+"/")
	return rel, ok
}

// parse reads and parses the files of p. A file that is not valid Go keeps
// what the parser could read; the compiler and the linters report the rest.
// A file of another package, such as a program behind `//go:build ignore`, is
// not part of p and is left out.
func (m *module) parse(fsys fs.FS, p *goPackage, names []string) error {
	slices.Sort(names)
	for _, name := range slices.Compact(names) {
		file := &goFile{
			path:      path.Join(p.dir, name),
			test:      strings.HasSuffix(name, "_test.go"),
			generated: strings.HasSuffix(name, ".gen.go"),
		}
		src, err := fs.ReadFile(fsys, file.path)
		if err != nil {
			return err
		}
		// The partial tree of a file with syntax errors still holds its imports.
		file.syntax, _ = parser.ParseFile(m.fset, file.path, src, parser.SkipObjectResolution)
		if file.syntax == nil {
			continue
		}
		name := file.syntax.Name.Name
		if p.name == "" && !file.test {
			p.name = name
		}
		if p.name != "" && name != p.name && name != p.name+"_test" {
			continue
		}
		p.files = append(p.files, file)
	}
	return nil
}

// line returns the line of a position in a parsed file.
func (m *module) line(pos token.Pos) int {
	return m.fset.Position(pos).Line
}

// readModulePath reads the module path from backend/go.mod.
func readModulePath(fsys fs.FS) (string, error) {
	f, err := fsys.Open(backendDir + "/go.mod")
	if err != nil {
		return "", err
	}
	// Closing a file that was only read cannot lose data.
	defer func() { _ = f.Close() }()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line, _, _ := strings.Cut(scanner.Text(), "//")
		fields := strings.Fields(line)
		if len(fields) != 2 || fields[0] != "module" {
			continue
		}
		if unquoted, err := strconv.Unquote(fields[1]); err == nil {
			return unquoted, nil
		}
		return fields[1], nil
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return "", fmt.Errorf("%s/go.mod has no module directive", backendDir)
}

// goList runs `go list -e -json ./...` in moduleDir. The flag -e lists a
// package with errors, such as an import that breaks a rule of the compiler,
// instead of failing; the rules judge such a package too. It needs no network
// once the modules are downloaded (`make setup`).
func goList(ctx context.Context, moduleDir string) ([]listedPackage, error) {
	cmd := exec.CommandContext(ctx, "go", "list", "-e", "-json", "./...")
	cmd.Dir = moduleDir
	// The backend is one module (TPL-0002); a go.work elsewhere must not
	// change what it holds.
	cmd.Env = append(os.Environ(), "GOWORK=off")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("go list in %s: %w: %s", moduleDir, err, strings.TrimSpace(stderr.String()))
	}
	var listed []listedPackage
	dec := json.NewDecoder(bytes.NewReader(out))
	for {
		var l listedPackage
		err := dec.Decode(&l)
		if errors.Is(err, io.EOF) {
			return listed, nil
		}
		if err != nil {
			return nil, fmt.Errorf("go list in %s: read its output: %w", moduleDir, err)
		}
		listed = append(listed, l)
	}
}

// unlistedPackages finds the directories of the module that hold Go files but
// that go list left out, and lists all of their Go files.
func unlistedPackages(fsys fs.FS, modulePath string, listed []listedPackage) ([]listedPackage, error) {
	known := map[string]bool{}
	for _, l := range listed {
		known[l.ImportPath] = true
	}
	var more []listedPackage
	err := fs.WalkDir(fsys, backendDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		if p != backendDir {
			if ignoredDir(d.Name()) {
				return fs.SkipDir
			}
			// A nested module is not part of the backend module.
			if _, err := fs.Stat(fsys, p+"/go.mod"); err == nil {
				return fs.SkipDir
			}
		}
		importPath := modulePath + strings.TrimPrefix(p, backendDir)
		if known[importPath] {
			return nil
		}
		entries, err := fs.ReadDir(fsys, p)
		if err != nil {
			return err
		}
		l := listedPackage{ImportPath: importPath}
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".go") && !ignoredDir(e.Name()) {
				l.IgnoredGoFiles = append(l.IgnoredGoFiles, e.Name())
			}
		}
		if len(l.IgnoredGoFiles) > 0 {
			more = append(more, l)
		}
		return nil
	})
	return more, err
}
