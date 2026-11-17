package adr

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"maps"
	"os"
	"regexp"
	"slices"
	"strings"

	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/tools/internal/finding"
)

// checksumRE is one line of `sha256sum` output: the hash, two spaces, and
// the name of a file in docs/adr/template/.
var checksumRE = regexp.MustCompile(`^([0-9a-f]{64})  ([^\s/]+)$`)

// sums returns the SHA-256 of every Markdown file that checksums.txt covers,
// by file name.
func (r *repo) sums(fsys fs.FS) (map[string]string, error) {
	sums := map[string]string{}
	for _, name := range r.templateMD {
		data, err := fs.ReadFile(fsys, templateDir+"/"+name)
		if err != nil {
			return nil, err
		}
		sum := sha256.Sum256(data)
		sums[name] = hex.EncodeToString(sum[:])
	}
	return sums, nil
}

// Seal writes docs/adr/template/checksums.txt under root, in `sha256sum`
// format and sorted by file name, and reports a change on w. It does nothing
// when docs/adr/template/ does not exist.
func Seal(root *os.Root, w io.Writer) error {
	fsys := root.FS()
	if _, err := fs.Stat(fsys, templateDir); errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	r, err := load(fsys)
	if err != nil {
		return err
	}
	sums, err := r.sums(fsys)
	if err != nil {
		return err
	}
	var b strings.Builder
	for _, name := range slices.Sorted(maps.Keys(sums)) {
		b.WriteString(sums[name] + "  " + name + "\n")
	}
	return writeIfChanged(root, checksumsPath, []byte(b.String()), w)
}

// templateFix tells a user of the template and a maintainer of the template
// what to do about the template file name, which someone did (the verb, such
// as "edit") and so changed.
func templateFix(name, verb, past string) string {
	user := "do not " + verb + " template records: restore the file, and write a project record in " +
		projectDir + "/ whose front matter has `supersedes: [" + templateID(name) + "]`"
	if verb == "add" {
		user = "do not add template records: write the decision as a project record in " + projectDir + "/ instead"
	}
	return "if you use the template, " + user +
		"; if you maintain the template and " + past + " the file on purpose, run `make adr-seal`"
}

// templateID returns the ID in the template file name, or TPL-NNNN.
func templateID(name string) string {
	if m := filenameRE.FindStringSubmatch(name); m != nil && m[1] == template.prefix {
		return m[1] + "-" + m[2]
	}
	return template.prefix + "-NNNN"
}

// checkChecksums compares the template records with checksums.txt.
func (r *repo) checkChecksums(fsys fs.FS) ([]finding.Finding, error) {
	data, err := fs.ReadFile(fsys, checksumsPath)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	var out []finding.Finding
	formatFix := "if you maintain the template, run `make adr-seal`; otherwise restore the file from the template"
	type entry struct {
		sum  string
		line int
	}
	listed := map[string]entry{}
	previous := ""
	text := strings.TrimSuffix(string(data), "\n")
	if text != "" {
		for i, l := range strings.Split(text, "\n") {
			report := func(message string) {
				out = append(out, finding.Finding{
					Path: checksumsPath, Line: i + 1, Rule: RuleChecksumsFormat, Message: message, Fix: formatFix, See: see,
				})
			}
			m := checksumRE.FindStringSubmatch(l)
			if m == nil {
				report("the line is not `<sha256>  <file name>`")
				continue
			}
			if _, ok := listed[m[2]]; ok {
				report(m[2] + " is listed twice")
				continue
			}
			if m[2] < previous {
				report("the lines are not sorted by file name")
			}
			listed[m[2]] = entry{sum: m[1], line: i + 1}
			previous = m[2]
		}
	}

	sums, err := r.sums(fsys)
	if err != nil {
		return nil, err
	}
	for name, sum := range sums {
		p := templateDir + "/" + name
		e, ok := listed[name]
		switch {
		case !ok:
			out = append(out, finding.Finding{
				Path: p, Line: 1, Rule: RuleTemplateChanged, See: see,
				Message: "this template file is not listed in " + checksumsPath,
				Fix:     templateFix(name, "add", "added"),
			})
		case e.sum != sum:
			out = append(out, finding.Finding{
				Path: p, Line: 1, Rule: RuleTemplateChanged, See: see,
				Message: "this template record differs from its checksum in " + checksumsPath,
				Fix:     templateFix(name, "edit", "edited"),
			})
		}
	}
	for name, e := range listed {
		if _, ok := sums[name]; !ok {
			out = append(out, finding.Finding{
				Path: checksumsPath, Line: e.line, Rule: RuleTemplateChanged, See: see,
				Message: "the template record " + name + " is listed but does not exist",
				Fix:     templateFix(name, "delete", "deleted"),
			})
		}
	}
	return out, nil
}
