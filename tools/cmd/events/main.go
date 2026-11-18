// Command events checks the contracts of the integration events (TPL-0013):
// the JSON Schema of each event in contracts/events/<context>/<Name>.v<N>.json.
// The rules are in package tools/internal/events, and
// docs/guides/event-changes.md describes them.
//
// Usage:
//
//	events [-root dir] lint
//	events [-root dir] -base dir [-base-name name] breaking
//
// Both commands read paths relative to <root>, one per line, on stdin, and
// keep the JSON files in contracts/events. lint checks each schema. breaking
// compares each schema with the same file in the -base directory, which holds
// the files of the base commit at their paths, and reports each schema of the
// base that is gone from <root>; tools/scripts/events-breaking.sh fills that
// directory from Git.
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

	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/tools/internal/events"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/tools/internal/finding"
)

const usage = "usage: events [-root dir] [-base dir] [-base-name name] <command>\n" +
	"\n" +
	"commands:\n" +
	"  lint       check the event schemas listed on stdin\n" +
	"  breaking   compare the event schemas listed on stdin with the same files in -base,\n" +
	"             and report the schemas of -base that are gone\n"

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
	flags := flag.NewFlagSet("events", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() { _, _ = io.WriteString(stderr, usage) }
	rootDir := flags.String("root", ".", "the root of the repository")
	baseDir := flags.String("base", "", "the directory that holds the files of the base commit")
	baseName := flags.String("base-name", "the base", "how findings name the base, such as its short commit id")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitOK
		}
		return exitError
	}
	command := strings.Join(flags.Args(), " ")
	switch {
	case command != "lint" && command != "breaking":
		_, _ = fmt.Fprintf(stderr, "events: unknown command %q\n%s", command, usage)
		return exitError
	case command == "breaking" && *baseDir == "":
		_, _ = fmt.Fprintf(stderr, "events: breaking needs -base\n%s", usage)
		return exitError
	}

	findings, err := execute(command, *rootDir, *baseDir, *baseName, stdin)
	if err == nil {
		err = finding.Write(stdout, findings)
	}
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "events: %s: %v\n", command, err)
		return exitError
	}
	if len(findings) > 0 {
		return exitFindings
	}
	return exitOK
}

// execute runs a valid command.
func execute(command, rootDir, baseDir, baseName string, stdin io.Reader) ([]finding.Finding, error) {
	root, err := os.OpenRoot(rootDir)
	if err != nil {
		return nil, err
	}
	// Closing a directory that was only read through cannot lose data.
	defer func() { _ = root.Close() }()
	paths, err := events.ReadPaths(stdin)
	if err != nil {
		return nil, err
	}
	if command == "lint" {
		return events.Lint(root.FS(), paths)
	}
	baseRoot, err := os.OpenRoot(baseDir)
	if err != nil {
		return nil, err
	}
	defer func() { _ = baseRoot.Close() }()
	base, err := events.ReadBase(baseRoot.FS(), baseName)
	if err != nil {
		return nil, err
	}
	return events.Breaking(root.FS(), base, paths)
}
