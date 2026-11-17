// Command init turns a repository created from the template into a project
// of its own. It replaces the Go module path and the project name of the
// template in every tracked file, resets the changelog and the release
// manifest, and prints what changed and what to do next.
//
//	go run ./init -C <repository> -module github.com/<owner>/<repo> [-name <name>] [-dry-run]
//
// `make init` runs it. It refuses to run on a working tree with uncommitted
// changes, and a second run with the same arguments changes nothing.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"regexp"
	"slices"
	"strings"
)

// The names that the template uses for itself.
const (
	templateModule = "github.com/ma-chisuke/go-nextjs-ddd-monorepo"
	templateName   = "go-nextjs-ddd-monorepo"
)

// Paths that keep the words of the template:
//   - docs/adr/template/ holds the decisions of the template, checked against
//     docs/adr/template/checksums.txt; a project never edits them (TPL-0001).
//
// Files marked linguist-vendored in .gitattributes are third-party and are
// skipped as well.
var keptPrefixes = []string{"docs/adr/template/"}

// sampleContexts lists the paths of the sample bounded contexts, which a
// project removes. The template has none yet. When the samples arrive, list
// here every path that belongs only to them: the code under backend/ and
// frontend/, the contracts, the documents in docs/contexts/, and the database
// schemas.
var sampleContexts []string

const (
	changelogPath    = "CHANGELOG.md"
	manifestPath     = ".release-please-manifest.json"
	initialChangelog = "# Changelog\n"
	initialRelease   = "0.0.0"
)

var (
	modulePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]*\.[a-z]{2,}(/[A-Za-z0-9._~-]+)+$`)
	namePattern   = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,213}$`)
)

func main() {
	os.Exit(run(context.Background(), os.Args[1:], os.Stdout, os.Stderr))
}

type options struct {
	dir    string
	module string
	name   string
	dryRun bool
}

// A change is one file that init rewrote, or would rewrite in a dry run.
type change struct {
	path   string
	detail string
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("init", flag.ContinueOnError)
	flags.SetOutput(stderr)
	var o options
	flags.StringVar(&o.dir, "C", ".", "the repository to initialize")
	flags.StringVar(&o.module, "module", "", "the Go module path of the project, such as github.com/acme/shop")
	flags.StringVar(&o.name, "name", "", "the project name (default: the last element of -module)")
	flags.BoolVar(&o.dryRun, "dry-run", false, "print what would change, and change nothing")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	text, err := initialize(ctx, o)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "init: %v\n", err)
		return 1
	}
	if _, err := io.WriteString(stdout, text); err != nil {
		return 1
	}
	return 0
}

// initialize makes the changes, or only lists them in a dry run, and returns
// the report.
func initialize(ctx context.Context, o options) (string, error) {
	if o.name == "" {
		o.name = path.Base(o.module)
	}
	if !modulePattern.MatchString(o.module) {
		return "", fmt.Errorf("MODULE=%q is not a module path such as github.com/<owner>/<repo>", o.module)
	}
	if !namePattern.MatchString(o.name) {
		return "", fmt.Errorf("NAME=%q must be lowercase letters, digits, '.', '_', or '-'", o.name)
	}

	top, err := git(ctx, o.dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	top = strings.TrimSpace(top)
	status, err := git(ctx, top, "status", "--porcelain")
	if err != nil {
		return "", err
	}
	warning := ""
	if status != "" {
		if !o.dryRun {
			return "", errors.New("the working tree has uncommitted changes; commit or stash them, then run init again")
		}
		warning = "The working tree has uncommitted changes; a real run would refuse to start.\n"
	}

	root, err := os.OpenRoot(top)
	if err != nil {
		return "", err
	}
	defer func() { _ = root.Close() }()

	files, err := filesToRename(ctx, top)
	if err != nil {
		return "", err
	}
	replacer := newReplacer(o.module, o.name)
	var changes []change
	for _, file := range files {
		c, err := rewrite(root, file, o.dryRun, func(text string) string { return replacer.Replace(text) })
		if err != nil {
			return "", err
		}
		changes = append(changes, c...)
	}
	for _, step := range []func(*os.Root, bool) ([]change, error){resetChangelog, resetManifest, removeSampleContexts} {
		c, err := step(root, o.dryRun)
		if err != nil {
			return "", err
		}
		changes = append(changes, c...)
	}
	return warning + report(o, changes), nil
}

// filesToRename lists the tracked files whose text init rewrites.
func filesToRename(ctx context.Context, top string) ([]string, error) {
	listed, err := git(ctx, top, "ls-files", "-z")
	if err != nil {
		return nil, err
	}
	files := strings.Split(strings.TrimSuffix(listed, "\x00"), "\x00")
	attributes, err := gitInput(ctx, top, strings.Join(files, "\x00"), "check-attr", "-z", "--stdin", "linguist-vendored")
	if err != nil {
		return nil, err
	}
	// check-attr -z prints <path> NUL <attribute> NUL <value> NUL per path.
	vendored := map[string]bool{}
	fields := strings.Split(attributes, "\x00")
	for i := 0; i+2 < len(fields); i += 3 {
		if fields[i+2] == "set" || fields[i+2] == "true" {
			vendored[fields[i]] = true
		}
	}
	return slices.DeleteFunc(files, func(file string) bool {
		if file == "" || vendored[file] || file == changelogPath || file == manifestPath {
			return true
		}
		return slices.ContainsFunc(keptPrefixes, func(prefix string) bool { return strings.HasPrefix(file, prefix) })
	}), nil
}

// newReplacer replaces the module path first, then the name, in one pass.
// A new value that contains an old one maps to itself and is matched first,
// so that a second run does not replace it again.
func newReplacer(module, name string) *strings.Replacer {
	var pairs []string
	for _, value := range []string{module, name} {
		if value != templateModule && value != templateName &&
			(strings.Contains(value, templateModule) || strings.Contains(value, templateName)) {
			pairs = append(pairs, value, value)
		}
	}
	pairs = append(pairs, templateModule, module, templateName, name)
	return strings.NewReplacer(pairs...)
}

// rewrite applies edit to a text file and writes the result when it differs.
func rewrite(root *os.Root, file string, dryRun bool, edit func(string) string) ([]change, error) {
	info, err := root.Lstat(file)
	if errors.Is(err, fs.ErrNotExist) || (err == nil && !info.Mode().IsRegular()) {
		// Deleted in the working tree, or a symbolic link: nothing to rewrite.
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	data, err := root.ReadFile(file)
	if err != nil {
		return nil, err
	}
	if bytes.IndexByte(data, 0) >= 0 {
		return nil, nil // binary
	}
	before := string(data)
	after := edit(before)
	if after == before {
		return nil, nil
	}
	if !dryRun {
		if err := root.WriteFile(file, []byte(after), info.Mode().Perm()); err != nil {
			return nil, err
		}
	}
	return []change{{path: file, detail: describeLines(before, after)}}, nil
}

func describeLines(before, after string) string {
	a, b := strings.Split(before, "\n"), strings.Split(after, "\n")
	changed := 0
	for i := range min(len(a), len(b)) {
		if a[i] != b[i] {
			changed++
		}
	}
	if changed == 1 {
		return "1 line"
	}
	return fmt.Sprintf("%d lines", changed)
}

// resetChangelog starts the changelog of the project from nothing.
func resetChangelog(root *os.Root, dryRun bool) ([]change, error) {
	c, err := rewrite(root, changelogPath, dryRun, func(string) string { return initialChangelog })
	return withDetail(c, "reset"), err
}

// resetManifest sets every version in the release-please manifest to 0.0.0.
func resetManifest(root *os.Root, dryRun bool) ([]change, error) {
	var failure error
	c, err := rewrite(root, manifestPath, dryRun, func(text string) string {
		versions := map[string]string{}
		if err := json.Unmarshal([]byte(text), &versions); err != nil {
			failure = fmt.Errorf("%s: %w", manifestPath, err)
			return text
		}
		for key := range versions {
			versions[key] = initialRelease
		}
		reset, err := json.MarshalIndent(versions, "", "  ")
		if err != nil {
			failure = err
			return text
		}
		return string(reset) + "\n"
	})
	if failure != nil {
		return nil, failure
	}
	return withDetail(c, "reset to "+initialRelease), err
}

// removeSampleContexts deletes the sample bounded contexts of the template.
func removeSampleContexts(root *os.Root, dryRun bool) ([]change, error) {
	var changes []change
	for _, sample := range sampleContexts {
		if _, err := root.Lstat(sample); errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if !dryRun {
			if err := root.RemoveAll(sample); err != nil {
				return nil, err
			}
		}
		changes = append(changes, change{path: sample, detail: "removed (sample bounded context)"})
	}
	return changes, nil
}

func withDetail(changes []change, detail string) []change {
	for i := range changes {
		changes[i].detail = detail
	}
	return changes
}

// nextSteps follows the report of a real run.
var nextSteps = []string{
	"Review the changes: git diff",
	"Run make setup, then make check.",
	"Set the copyright holder in LICENSE, or choose another license.",
	`Commit: git commit --all --message "chore: initialize from the template"`,
}

func report(o options, changes []change) string {
	var b strings.Builder
	if len(changes) == 0 {
		fmt.Fprintf(&b, "Nothing to change: the repository already uses %s (%s).\n", o.module, o.name)
		return b.String()
	}
	if o.dryRun {
		fmt.Fprintf(&b, "Dry run: init would change these files for %s (%s). Nothing was written.\n", o.module, o.name)
	} else {
		fmt.Fprintf(&b, "Initialized %s (%s). Changed files:\n", o.module, o.name)
	}
	for _, c := range changes {
		fmt.Fprintf(&b, "  %-40s %s\n", c.path, c.detail)
	}
	if !o.dryRun {
		b.WriteString("\nNext:\n")
		for i, step := range nextSteps {
			fmt.Fprintf(&b, "  %d. %s\n", i+1, step)
		}
	}
	return b.String()
}

// git runs git in dir and returns its standard output. Git's variables that
// select a repository (GIT_DIR, GIT_INDEX_FILE, and the others) are dropped,
// so that -C alone decides which repository git works on, even under a Git
// hook.
func git(ctx context.Context, dir string, args ...string) (string, error) {
	return gitInput(ctx, dir, "", args...)
}

func gitInput(ctx context.Context, dir, input string, args ...string) (string, error) {
	//nolint:gosec // The program is always git; its arguments are flags and paths, never a shell string.
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	cmd.Env = slices.DeleteFunc(os.Environ(), func(v string) bool { return strings.HasPrefix(v, "GIT_") })
	cmd.Stdin = strings.NewReader(input)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return string(output), nil
}
