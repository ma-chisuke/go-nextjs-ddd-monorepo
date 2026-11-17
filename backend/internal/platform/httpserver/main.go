package httpserver

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/config"
)

// HealthCheckCommand is the argument that makes Main check the health of the
// service instead of running it.
const HealthCheckCommand = "healthcheck"

// RunFunc runs a service until ctx is canceled. getenv reads its
// configuration, and the service writes its log to stdout.
type RunFunc func(ctx context.Context, getenv func(string) string, stdout io.Writer) error

// Mode is another process of a bounded context, which the same binary runs
// when its name is the argument: a worker next to the API, such as the relay
// of the outbox (TPL-0014).
type Mode struct {
	// Name is the argument that selects the mode, such as "relay".
	Name string
	// Run runs the process of the mode until ctx is canceled.
	Run RunFunc
}

// Main is the main function of the command of a bounded context.
//
// Without arguments, it runs the service until SIGINT or SIGTERM. With the
// name of a mode as its argument, it runs that mode instead. With the
// argument "healthcheck", it asks the process that runs on this host for its
// health (CheckHealth), and exits with 0 when the process is healthy and 1
// when it is not. A container image without a shell or curl, such as
// distroless, runs this as its health check, in Docker Compose and in ECS.
func Main(name string, run RunFunc, modes ...Mode) {
	os.Exit(command(context.Background(), name, os.Args[1:], os.Getenv, os.Stdout, os.Stderr, run, modes...))
}

// command is Main without the process: it returns the exit code.
func command(ctx context.Context, name string, args []string, getenv func(string) string, stdout, stderr io.Writer, run RunFunc, modes ...Mode) int {
	var err error
	switch {
	case len(args) == 0:
		err = runUntilSignal(ctx, run, getenv, stdout)
	case len(args) == 1 && args[0] == HealthCheckCommand:
		addr := config.NewReader(getenv).Default(AddrVariable, DefaultAddr)
		ctx, cancel := context.WithTimeout(ctx, healthCheckTimeout)
		err = CheckHealth(ctx, addr)
		cancel()
	case len(args) == 1 && modeNamed(modes, args[0]) != nil:
		err = runUntilSignal(ctx, modeNamed(modes, args[0]), getenv, stdout)
	default:
		names := make([]string, 0, len(modes)+1)
		for _, mode := range modes {
			names = append(names, mode.Name)
		}
		names = append(names, HealthCheckCommand)
		_, _ = fmt.Fprintf(stderr, "usage: %s [%s]\n", name, strings.Join(names, "|"))
		return 2
	}
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "%s: %v\n", name, err)
		return 1
	}
	return 0
}

// runUntilSignal runs a process until it returns, or until SIGINT or SIGTERM
// cancels its context.
func runUntilSignal(ctx context.Context, run RunFunc, getenv func(string) string, stdout io.Writer) error {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()
	return run(ctx, getenv, stdout)
}

// modeNamed returns the function of the mode with the name, or nil.
func modeNamed(modes []Mode, name string) RunFunc {
	for _, mode := range modes {
		if mode.Name == name {
			return mode.Run
		}
	}
	return nil
}
