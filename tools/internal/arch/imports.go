package arch

import (
	"path"
	"strconv"

	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/tools/internal/finding"
)

// pureStandardLibrary is the allow-list of the domain layer: packages of the
// standard library that do no I/O and read no clock or random source on
// their own (TPL-0004). Widening it needs a decision record.
var pureStandardLibrary = map[string]bool{
	"cmp":          true,
	"errors":       true,
	"fmt":          true,
	"maps":         true,
	"math":         true,
	"net/mail":     true,
	"regexp":       true,
	"slices":       true,
	"sort":         true,
	"strconv":      true,
	"strings":      true,
	"time":         true,
	"unicode":      true,
	"unicode/utf8": true,
}

// importEdge is one import of one file.
type importEdge struct {
	pkg  *goPackage
	file *goFile
	line int
	// target is the imported path; for a package of the module, dir and loc
	// say where it is.
	target string
	inside bool
	dir    string
	loc    location
}

// checkImports applies the rules on imports: context-import, module-import,
// platform-import, layer, and domain-purity. Each bad import gets one
// finding, from the first rule that it breaks in that order.
func (c *checker) checkImports() {
	for _, p := range c.module.packages {
		for _, f := range p.files {
			for _, spec := range f.syntax.Imports {
				target, err := strconv.Unquote(spec.Path.Value)
				if err != nil {
					continue
				}
				e := importEdge{pkg: p, file: f, line: c.module.line(spec.Pos()), target: target}
				var rel string
				if rel, e.inside = c.module.relative(target); e.inside {
					e.dir = path.Join(backendDir, rel)
					e.loc = locate(rel)
				}
				if !c.judgeBoundary(e) {
					c.judgePurity(e)
				}
			}
		}
	}
}

// judgeBoundary applies the rules on the boundaries of contexts and layers,
// and reports whether one of them fired.
func (c *checker) judgeBoundary(e importEdge) bool {
	src, dst := e.pkg.loc, e.loc
	if !e.inside || dst.area != areaContext {
		return false
	}
	same := src.area == areaContext && src.context == dst.context
	switch {
	case src.area == areaPlatform:
		c.add(finding.Finding{
			Path: e.file.path, Line: e.line, Rule: RulePlatformImport, See: seeLayers,
			Message: "this platform package imports " + e.dir + ", which belongs to the context " + dst.context,
			Fix: "keep platform free of business code: move this code into " + dst.context +
				", or let the context hand platform what it needs as a parameter or an interface",
		})
	case src.area == areaContext && !same:
		c.add(finding.Finding{
			Path: e.file.path, Line: e.line, Rule: RuleContextImport, See: seeContexts,
			Message: "a package of the context " + src.context + " imports " + e.dir +
				", which belongs to the context " + dst.context,
			Fix: "never import another context: call the HTTP API of " + dst.context +
				" through a client generated from contracts/http/" + dst.context + ".openapi.yaml" +
				", in an adapter behind a port of app, or subscribe to its events",
		})
	case same:
		if e.file.test {
			// A test may wire layers together, as module.go does.
			return false
		}
		return c.judgeLayer(e)
	case !dst.protected && (src.area != areaCmd || src.context != dst.context):
		what := e.dir + ", the package that wires the context " + dst.context + " together"
		if dst.layer != layerRoot {
			what = e.dir + ", a package of the context " + dst.context + " outside its internal directory"
		}
		c.add(finding.Finding{
			Path: e.file.path, Line: e.line, Rule: RuleModuleImport, See: seeContexts,
			Message: "this file imports " + what,
			Fix: "only backend/cmd/" + dst.context + " may import it; other code reaches " + dst.context +
				" through its HTTP API or its events",
		})
	default:
		// The command of the context imports its wiring, or the compiler
		// refuses an import of a package under internal/<context>/internal/.
		return false
	}
	return true
}

// judgeLayer applies the rule on the direction of dependencies inside one
// context to an import of a non-test file, and reports whether it fired.
func (c *checker) judgeLayer(e importEdge) bool {
	src, dst := e.pkg.loc, e.loc
	var fix string
	switch {
	case src.layer == layerDomain && (dst.layer == layerApp || dst.layer == layerAdapter):
		fix = "dependencies point inward, from adapter to app to domain: move what the domain needs " +
			"into the domain, or let the use case pass it in as a value"
	case src.layer == layerApp && dst.layer == layerAdapter:
		if !c.hasDomain[src.context] && e.dir == layerDir(src.context, "adapter/postgres") {
			// A transaction script calls the generated queries (TPL-0003).
			return false
		}
		fix = "declare a port, an interface in app, that the adapter implements, and let module.go connect " +
			"the two; only a context without a domain layer, a transaction script, may import its own " +
			"adapter/postgres from app"
	case src.layer == layerAdapter && dst.layer == layerAdapter && src.adapter != dst.adapter:
		fix = "adapters do not know each other: let each one depend on app and domain only, and let " +
			"module.go wire them together"
	default:
		return false
	}
	c.add(finding.Finding{
		Path: e.file.path, Line: e.line, Rule: RuleLayer, See: seeLayers,
		Message: describeLayer(src) + " of the context " + src.context + " imports " + e.dir + ", " +
			describeLayer(dst),
		Fix: fix,
	})
	return true
}

// describeLayer names the layer of a package in a context.
func describeLayer(loc location) string {
	if loc.layer == layerAdapter {
		return "the adapter " + loc.adapter
	}
	return "the " + loc.layer + " layer"
}

// judgePurity applies the allow-list of the domain layer to an import of a
// non-test file.
func (c *checker) judgePurity(e importEdge) {
	src := e.pkg.loc
	if src.layer != layerDomain || e.file.test {
		return
	}
	var what string
	switch {
	case !e.inside && pureStandardLibrary[e.target]:
		return
	case !e.inside:
		what = e.target + ", which is not on the allow-list of pure standard library packages"
	case e.loc.area == areaContext && e.loc.context == src.context && e.loc.layer == layerDomain:
		return
	default:
		what = e.dir + ", which is not part of the domain"
	}
	c.add(finding.Finding{
		Path: e.file.path, Line: e.line, Rule: RuleDomainPurity, See: seeLayers,
		Message: "the domain imports " + what,
		Fix: "do the I/O in the use case in app, and pass the result to the domain as a value: " +
			"the current time as a time.Time, a new ID, data that the use case loaded first; " +
			"widening the allow-list in tools/internal/arch needs a decision record (skill write-adr)",
	})
}
