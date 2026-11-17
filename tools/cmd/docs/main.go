// Command docs checks the decision records (TPL-0001) and the Japanese
// translations (TPL-0021) of the repository, and writes the files that those
// checks compare against.
//
// Usage:
//
//	docs [-root dir] adr check    validate records, the index, and the template checksums
//	docs [-root dir] adr index    write docs/adr/README.md and docs/adr/README.ja.md
//	docs [-root dir] adr seal     write docs/adr/template/checksums.txt
//	docs [-root dir] i18n check   validate the translations of the Markdown files on stdin
//	docs [-root dir] i18n stamp   write the i18n-source marker into the translations on stdin
//
// Findings go to stdout. The exit status is 0 without findings, 1 with
// findings, and 2 on a usage or I/O error.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/tools/internal/adr"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/tools/internal/finding"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/tools/internal/i18n"
)

const usage = "usage: docs [-root dir] <command>\n" +
	"\n" +
	"commands:\n" +
	"  adr check    validate records, the index, and the template checksums\n" +
	"  adr index    write docs/adr/README.md and docs/adr/README.ja.md\n" +
	"  adr seal     write docs/adr/template/checksums.txt\n" +
	"  i18n check   validate the translations of the Markdown files listed on stdin\n" +
	"  i18n stamp   write the i18n-source marker into the translations listed on stdin\n"

// Exit statuses.
const (
	exitOK       = 0
	exitFindings = 1
	exitError    = 2
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// run executes one command and returns the exit status.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("docs", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() { _, _ = io.WriteString(stderr, usage) }
	rootDir := flags.String("root", ".", "the root of the repository")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitOK
		}
		return exitError
	}
	command := strings.Join(flags.Args(), " ")
	switch command {
	case "adr check", "adr index", "adr seal", "i18n check", "i18n stamp":
	default:
		_, _ = fmt.Fprintf(stderr, "docs: unknown command %q\n%s", command, usage)
		return exitError
	}

	root, err := os.OpenRoot(*rootDir)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "docs: %v\n", err)
		return exitError
	}
	// Closing a directory that was only read and written through cannot lose data.
	defer func() { _ = root.Close() }()

	findings, err := execute(command, root, stdin, stdout)
	if err == nil {
		err = finding.Write(stdout, findings)
	}
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "docs: %s: %v\n", command, err)
		return exitError
	}
	if len(findings) > 0 {
		return exitFindings
	}
	return exitOK
}

// execute runs a valid command against root.
func execute(command string, root *os.Root, stdin io.Reader, stdout io.Writer) ([]finding.Finding, error) {
	switch command {
	case "adr check":
		return adr.Check(root.FS())
	case "adr index":
		return nil, adr.WriteIndex(root, stdout)
	case "adr seal":
		return nil, adr.Seal(root, stdout)
	}
	paths, err := i18n.ReadPaths(stdin)
	if err != nil {
		return nil, err
	}
	if command == "i18n check" {
		return i18n.Check(root.FS(), paths)
	}
	return nil, i18n.Stamp(root, paths, stdout)
}
