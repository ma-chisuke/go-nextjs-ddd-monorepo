package httpserver

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/config"
)

// HealthCheckCommand is the argument that makes Main check the health of the
// service instead of running it.
const HealthCheckCommand = "healthcheck"

// RunFunc runs a service until ctx is canceled. getenv reads its
// configuration, and the service writes its log to stdout.
type RunFunc func(ctx context.Context, getenv func(string) string, stdout io.Writer) error

// Main is the main function of the command of a bounded context.
//
// Without arguments, it runs the service until SIGINT or SIGTERM. With the
// argument "healthcheck", it asks the service that runs on this host for its
// health instead (CheckHealth), and exits with 0 when the service is healthy
// and 1 when it is not. A container image without a shell or curl, such as
// distroless, runs this as its health check, in Docker Compose and in ECS.
func Main(name string, run RunFunc) {
	os.Exit(command(context.Background(), name, os.Args[1:], os.Getenv, os.Stdout, os.Stderr, run))
}

// command is Main without the process: it returns the exit code.
func command(ctx context.Context, name string, args []string, getenv func(string) string, stdout, stderr io.Writer, run RunFunc) int {
	var err error
	switch {
	case len(args) == 0:
		ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
		err = run(ctx, getenv, stdout)
		stop()
	case len(args) == 1 && args[0] == HealthCheckCommand:
		addr := config.NewReader(getenv).Default(AddrVariable, DefaultAddr)
		ctx, cancel := context.WithTimeout(ctx, healthCheckTimeout)
		err = CheckHealth(ctx, addr)
		cancel()
	default:
		_, _ = fmt.Fprintf(stderr, "usage: %s [%s]\n", name, HealthCheckCommand)
		return 2
	}
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "%s: %v\n", name, err)
		return 1
	}
	return 0
}
