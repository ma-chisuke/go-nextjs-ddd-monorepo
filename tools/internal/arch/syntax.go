package arch

import (
	"go/ast"
	"go/types"
	"path"
	"strconv"
	"strings"

	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/tools/internal/finding"
)

// clockFunctions are the functions of package time that read the clock or
// wait for it. The domain takes the time as a value instead (TPL-0004).
var clockFunctions = map[string]string{
	"Now":       "reads the clock",
	"Since":     "reads the clock",
	"Until":     "reads the clock",
	"After":     "waits for the clock",
	"AfterFunc": "waits for the clock",
	"Tick":      "waits for the clock",
	"NewTimer":  "waits for the clock",
	"NewTicker": "waits for the clock",
	"Sleep":     "waits for the clock",
}

// routeMethods are the names of the functions and methods that register an
// HTTP route, as on http.ServeMux and in package http.
var routeMethods = map[string]bool{"Handle": true, "HandleFunc": true}

// routeOwner is the one package, besides the generated code, that registers
// routes: the server that every context runs in (TPL-0007, layer 3).
var routeOwner = path.Join(contextsDir, platformName, "httpserver")

// checkSyntax applies the rules that read the code itself, not only its
// imports: domain-clock and route-registration. Test files are left out: a
// test may fix the clock or serve a fake API.
func (c *checker) checkSyntax() {
	for _, p := range c.module.packages {
		routes := p.dir != routeOwner && !strings.HasPrefix(p.dir, routeOwner+"/")
		for _, f := range p.files {
			if f.test {
				continue
			}
			if p.loc.area == areaContext && p.loc.layer == layerDomain {
				c.checkClock(f)
			}
			if routes && !f.generated {
				c.checkRoutes(f)
			}
		}
	}
}

// checkClock reports each use of a function of package time that reads the
// clock or waits for it, whether the code calls it or passes it on.
func (c *checker) checkClock(f *goFile) {
	name := importName(f.syntax, "time")
	if name == "" {
		return
	}
	ast.Inspect(f.syntax, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		x, ok := sel.X.(*ast.Ident)
		does, clock := clockFunctions[sel.Sel.Name]
		if !ok || x.Name != name || !clock {
			return true
		}
		c.add(finding.Finding{
			Path: f.path, Line: c.module.line(sel.Pos()), Rule: RuleDomainClock, See: seeLayers,
			Message: "the domain uses time." + sel.Sel.Name + ", which " + does,
			Fix: "take the current time as a time.Time argument, which the use case gets from its clock port; " +
				"the package time stays for its types, such as time.Time and time.Duration",
		})
		return true
	})
}

// importName returns the name under which a file refers to an imported
// package: its declared name, or the last element of its path. It returns ""
// when the file does not import the package by name.
func importName(file *ast.File, importPath string) string {
	for _, spec := range file.Imports {
		if p, err := strconv.Unquote(spec.Path.Value); err != nil || p != importPath {
			continue
		}
		if spec.Name == nil {
			return path.Base(importPath)
		}
		if spec.Name.Name == "_" || spec.Name.Name == "." {
			return ""
		}
		return spec.Name.Name
	}
	return ""
}

// checkRoutes reports each call of a function or a method named Handle or
// HandleFunc.
func (c *checker) checkRoutes(f *goFile) {
	ast.Inspect(f.syntax, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || !routeMethods[sel.Sel.Name] {
			return true
		}
		c.add(finding.Finding{
			Path: f.path, Line: c.module.line(call.Pos()), Rule: RuleRouteRegistration, See: seeConformance,
			Message: "this file registers an HTTP route with " + types.ExprString(sel) +
				"; routes come only from the code generated from the API document and from " + routeOwner,
			Fix: "add the operation to the OpenAPI document of the context in contracts/http/ and run `make gen`, " +
				"so that the generated code registers it; if the call registers no HTTP route, give the method " +
				"another name",
		})
		return true
	})
}
