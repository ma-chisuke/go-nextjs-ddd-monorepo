// Package adr checks the architecture decision records of the repository
// against TPL-0001, writes their index, and seals the template records with
// checksums.
//
// Template records live in docs/adr/template/ as TPL-NNNN-<slug>.md; project
// records live in docs/adr/ as ADR-NNNN-<slug>.md. Each record has a Japanese
// translation <ID>-<slug>.ja.md next to it.
package adr

import (
	"errors"
	"io/fs"
	"regexp"
	"slices"
	"strings"

	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/tools/internal/finding"
)

// Rule identifiers.
const (
	RuleUnexpectedFile  = "adr/unexpected-file"
	RuleFilename        = "adr/filename"
	RuleDuplicateID     = "adr/duplicate-id"
	RuleFrontMatter     = "adr/front-matter"
	RuleStatus          = "adr/status"
	RuleDate            = "adr/date"
	RuleSupersedes      = "adr/supersedes"
	RuleTitle           = "adr/title"
	RuleSections        = "adr/sections"
	RuleJaFrontMatter   = "adr/ja-front-matter"
	RuleJaSections      = "adr/ja-sections"
	RuleStaleIndex      = "adr/stale-index"
	RuleTemplateChanged = "adr/template-changed"
	RuleChecksumsFormat = "adr/checksums-format"
)

const (
	// see is the decision record that explains every rule of this package.
	see                  = "TPL-0001"
	projectDir           = "docs/adr"
	templateDir          = "docs/adr/template"
	templateSubdirectory = "template"
	indexEN              = "docs/adr/README.md"
	indexJA              = "docs/adr/README.ja.md"
	readmeEN             = "README.md"
	readmeJA             = "README.ja.md"
	checksumsName        = "checksums.txt"
	checksumsPath        = templateDir + "/" + checksumsName
)

// series is one sequence of record numbers: the template's or the project's.
type series struct {
	prefix string
	dir    string
}

var (
	project  = series{prefix: "ADR", dir: projectDir}
	template = series{prefix: "TPL", dir: templateDir}
	// allSeries is in the order of the index: project decisions first.
	allSeries = []series{project, template}
)

var filenameRE = regexp.MustCompile(`^([A-Z]+)-(\d{4})-([a-z0-9]+(?:-[a-z0-9]+)*)(\.ja)?\.md$`)

// file is one Markdown file of a record.
type file struct {
	path string
	data []byte
	doc  document
}

// record is a decision: its English file, its translation, or both.
type record struct {
	series series
	id     string // TPL-0001
	base   string // TPL-0001-record-decisions-as-adrs
	en     *file
	ja     *file
}

// repo is every record of the repository, read from disk.
type repo struct {
	records []*record
	// findings holds the problems found while reading the directories.
	findings []finding.Finding
	// templateMD holds the names of the Markdown files in docs/adr/template/
	// that checksums.txt covers: every one except the READMEs.
	templateMD []string
}

// load reads both directories of records from fsys.
func load(fsys fs.FS) (*repo, error) {
	r := &repo{}
	for _, s := range allSeries {
		entries, err := fs.ReadDir(fsys, s.dir)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		byBase := map[string]*record{}
		for _, e := range entries {
			if err := r.add(fsys, s, e, byBase); err != nil {
				return nil, err
			}
		}
		for _, rec := range byBase {
			r.records = append(r.records, rec)
		}
	}
	slices.SortFunc(r.records, func(a, b *record) int {
		return strings.Compare(a.series.prefix+a.base, b.series.prefix+b.base)
	})
	return r, nil
}

// add reads one entry of the directory of series s.
func (r *repo) add(fsys fs.FS, s series, e fs.DirEntry, byBase map[string]*record) error {
	name := e.Name()
	p := s.dir + "/" + name
	switch {
	case strings.HasPrefix(name, "."):
		// Files such as .DS_Store belong to the local machine, not to the repository.
		return nil
	case e.IsDir():
		if s == project && name == templateSubdirectory {
			return nil
		}
		r.unexpected(p, s)
		return nil
	case name == readmeEN || name == readmeJA:
		return nil
	case s == template && name == checksumsName:
		return nil
	case !strings.HasSuffix(name, ".md"):
		r.unexpected(p, s)
		return nil
	}
	if s == template {
		r.templateMD = append(r.templateMD, name)
	}
	other := project
	if s == project {
		other = template
	}
	m := filenameRE.FindStringSubmatch(name)
	switch {
	case m != nil && m[1] == other.prefix:
		r.findings = append(r.findings, finding.Finding{
			Path: p, Line: 1, Rule: RuleFilename, See: see,
			Message: "a " + other.prefix + " record belongs in " + other.dir + "/, and " + s.dir + "/ holds " + s.prefix + " records only",
			Fix:     "move the file to " + other.dir + "/, or number it in the " + s.prefix + " series",
		})
		return nil
	case m == nil || m[1] != s.prefix:
		r.findings = append(r.findings, finding.Finding{
			Path: p, Line: 1, Rule: RuleFilename, See: see,
			Message: "the name is not " + s.prefix + "-NNNN-<slug>.md or " + s.prefix + "-NNNN-<slug>.ja.md",
			Fix:     "rename the file, with a slug of lowercase letters, digits, and single hyphens, as in " + s.prefix + "-0007-use-postgresql.md",
		})
		return nil
	case m[2] == "0000":
		r.findings = append(r.findings, finding.Finding{
			Path: p, Line: 1, Rule: RuleFilename, See: see,
			Message: "record numbers start at 0001",
			Fix:     "take the next free number of the " + s.prefix + " series",
		})
		return nil
	}
	data, err := fs.ReadFile(fsys, p)
	if err != nil {
		return err
	}
	base := strings.TrimSuffix(strings.TrimSuffix(name, ".md"), ".ja")
	rec := byBase[base]
	if rec == nil {
		rec = &record{series: s, id: m[1] + "-" + m[2], base: base}
		byBase[base] = rec
	}
	f := &file{path: p, data: data, doc: parse(data)}
	if m[4] == "" {
		rec.en = f
	} else {
		rec.ja = f
	}
	return nil
}

// unexpected records a file that has no place in a directory of records.
func (r *repo) unexpected(p string, s series) {
	allowed := "records, " + readmeEN + ", and " + readmeJA
	if s == template {
		allowed = "records, " + readmeEN + ", " + readmeJA + ", and " + checksumsName
	}
	r.findings = append(r.findings, finding.Finding{
		Path: p, Line: 1, Rule: RuleUnexpectedFile, See: see,
		Message: s.dir + "/ holds only " + allowed,
		Fix:     "move this file out of " + s.dir + "/",
	})
}

// known returns the records that have an English file, by ID. Two records
// with the same ID keep the first.
func (r *repo) known() map[string]*record {
	ids := map[string]*record{}
	for _, rec := range r.records {
		if _, ok := ids[rec.id]; !ok && rec.en != nil {
			ids[rec.id] = rec
		}
	}
	return ids
}
