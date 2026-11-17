// Package arch checks that the Go backend keeps the boundaries that the
// decision records define: bounded contexts that do not depend on each other
// (TPL-0005), layers whose dependencies point inward around a domain without
// I/O (TPL-0004), HTTP routes that come only from generated code (TPL-0007),
// SQL that stays in the schema of its own context (TPL-0009), and a glossary
// that ties each context to the words of its business (TPL-0003).
//
// Every rule derives from the directory layout, so a new context needs no
// configuration:
//
//	backend/
//	├── cmd/<context>/              the only importer of internal/<context>
//	└── internal/
//	    ├── platform/...            technical packages; they know no context
//	    └── <context>/              every other directory is a context
//	        ├── module.go           the wiring of the context
//	        └── internal/
//	            ├── domain/         core contexts only; no I/O
//	            ├── app/            use cases and ports
//	            └── adapter/<name>/ http, postgres, event, or a client
//
// The packages come from `go list -json ./...` in backend/, and the imports,
// calls, and declarations from go/parser.
package arch

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path"
	"slices"
	"strings"

	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/tools/internal/finding"
)

// Rule identifiers.
const (
	RuleContextImport     = "arch/context-import"
	RuleModuleImport      = "arch/module-import"
	RulePlatformImport    = "arch/platform-import"
	RuleLayer             = "arch/layer"
	RuleDomainPurity      = "arch/domain-purity"
	RuleDomainClock       = "arch/domain-clock"
	RuleRouteRegistration = "arch/route-registration"
	RuleSchemaReference   = "arch/schema-reference"
	RuleGlossary          = "arch/glossary"
)

// The decision records that explain the rules.
const (
	seeSplit       = "TPL-0003"
	seeLayers      = "TPL-0004"
	seeContexts    = "TPL-0005"
	seeConformance = "TPL-0007"
	seeSchemas     = "TPL-0009"
)

const (
	// backendDir holds the Go module of the backend.
	backendDir = "backend"
	// contextsDir holds the code of the contexts and of platform.
	contextsDir = backendDir + "/internal"
	// docsDir holds the documents of each context.
	docsDir = "docs/contexts"
	// platformName is the directory of the technical packages.
	platformName = "platform"
)

// checker holds what the rules read, and collects their findings.
type checker struct {
	fsys     fs.FS
	module   *module
	contexts []string
	// hasDomain tells the contexts that have a domain layer: the core
	// contexts. The others are transaction scripts (TPL-0003).
	hasDomain map[string]bool
	findings  []finding.Finding
}

// Check judges the backend of the repository whose root is dir. A repository
// without backend/go.mod has nothing to judge.
func Check(ctx context.Context, dir string) ([]finding.Finding, error) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, err
	}
	// Closing a directory that was only read cannot lose data.
	defer func() { _ = root.Close() }()
	fsys := root.FS()

	if _, err := fs.Stat(fsys, backendDir+"/go.mod"); errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	m, err := loadModule(ctx, dir, fsys)
	if err != nil {
		return nil, err
	}
	contexts, err := listContexts(fsys)
	if err != nil {
		return nil, err
	}
	c := &checker{fsys: fsys, module: m, contexts: contexts, hasDomain: map[string]bool{}}
	for _, p := range m.packages {
		if p.loc.area == areaContext && p.loc.layer == layerDomain {
			c.hasDomain[p.loc.context] = true
		}
	}
	c.checkImports()
	c.checkSyntax()
	if err := c.checkSQL(); err != nil {
		return nil, err
	}
	if err := c.checkGlossaries(); err != nil {
		return nil, err
	}
	return c.findings, nil
}

// add records a finding.
func (c *checker) add(f finding.Finding) {
	c.findings = append(c.findings, f)
}

// listContexts returns the directories under backend/internal/ other than
// platform, in order. Go ignores directories whose names start with "." or
// "_", and testdata; so does this list.
func listContexts(fsys fs.FS) ([]string, error) {
	entries, err := fs.ReadDir(fsys, contextsDir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var contexts []string
	for _, e := range entries {
		if e.IsDir() && e.Name() != platformName && !ignoredDir(e.Name()) {
			contexts = append(contexts, e.Name())
		}
	}
	slices.Sort(contexts)
	return contexts, nil
}

// ignoredDir reports whether the go command skips a directory of this name
// when it matches "./...".
func ignoredDir(name string) bool {
	return strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") || name == "testdata" || name == "vendor"
}

// Areas of the backend module.
const (
	areaOutside  = ""         // anything outside cmd/ and internal/
	areaCmd      = "cmd"      // backend/cmd/<name>/...
	areaPlatform = "platform" // backend/internal/platform/...
	areaContext  = "context"  // backend/internal/<context>/...
)

// Layers inside a context.
const (
	layerOther   = ""        // any other package of the context
	layerRoot    = "root"    // backend/internal/<context>: module.go
	layerDomain  = "domain"  // backend/internal/<context>/internal/domain/...
	layerApp     = "app"     // backend/internal/<context>/internal/app/...
	layerAdapter = "adapter" // backend/internal/<context>/internal/adapter/<name>/...
)

// location says where a package sits in the layout.
type location struct {
	area string
	// context is the context of a package in a context, or the directory under
	// cmd/ of a command.
	context string
	layer   string
	// adapter is the name of the adapter of a package in the adapter layer.
	adapter string
	// protected is true under backend/internal/<context>/internal/, which the
	// compiler keeps from the code outside the context.
	protected bool
}

// locate places a directory of the backend module, given relative to the
// module root ("" for the root, "internal/facility" for a context).
func locate(rel string) location {
	parts := strings.Split(rel, "/")
	switch {
	case len(parts) < 2:
		return location{area: areaOutside}
	case parts[0] == "cmd":
		return location{area: areaCmd, context: parts[1]}
	case parts[0] != "internal":
		return location{area: areaOutside}
	case parts[1] == platformName:
		return location{area: areaPlatform}
	}
	loc := location{area: areaContext, context: parts[1]}
	if len(parts) == 2 {
		loc.layer = layerRoot
		return loc
	}
	loc.protected = parts[2] == "internal"
	if !loc.protected || len(parts) < 4 {
		return loc
	}
	switch parts[3] {
	case layerDomain, layerApp:
		loc.layer = parts[3]
	case layerAdapter:
		if len(parts) >= 5 {
			loc.layer = layerAdapter
			loc.adapter = parts[4]
		}
	}
	return loc
}

// layerDir returns the directory of a layer of a context, relative to the
// root of the repository.
func layerDir(context, layer string) string {
	return path.Join(contextsDir, context, "internal", layer)
}
