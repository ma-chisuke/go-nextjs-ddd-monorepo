// Command arch checks that the Go backend keeps the boundaries of the bounded
// contexts and of their layers (TPL-0003, TPL-0004, TPL-0005, TPL-0007,
// TPL-0009). The rules are in package tools/internal/arch.
//
// Usage:
//
//	arch [-root dir]
//
// It reads the module in <root>/backend with `go list` and go/parser, the SQL
// files of each context, and the glossaries in <root>/docs/contexts/.
// Findings go to stdout. The exit status is 0 without findings, 1 with
// findings, and 2 on a usage or I/O error.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/tools/internal/arch"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/tools/internal/finding"
)

const usage = "usage: arch [-root dir]\n" +
	"\n" +
	"Checks the boundaries of the contexts and the layers of <root>/backend.\n"

// Exit statuses.
const (
	exitOK       = 0
	exitFindings = 1
	exitError    = 2
)

func main() {
	os.Exit(run(context.Background(), os.Args[1:], os.Stdout, os.Stderr))
}

// run checks the repository and returns the exit status.
func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("arch", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() { _, _ = io.WriteString(stderr, usage) }
	rootDir := flags.String("root", ".", "the root of the repository")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitOK
		}
		return exitError
	}
	if flags.NArg() > 0 {
		_, _ = fmt.Fprintf(stderr, "arch: unexpected argument %q\n%s", flags.Arg(0), usage)
		return exitError
	}
	findings, err := arch.Check(ctx, *rootDir)
	if err == nil {
		err = finding.Write(stdout, findings)
	}
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "arch: %v\n", err)
		return exitError
	}
	if len(findings) > 0 {
		return exitFindings
	}
	return exitOK
}
