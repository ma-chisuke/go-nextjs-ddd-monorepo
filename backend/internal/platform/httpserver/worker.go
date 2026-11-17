package httpserver

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
)

// RunWorker runs a process that serves no API, such as the relay of an outbox
// or the consumer of a queue (TPL-0014). It runs work until ctx is canceled,
// and serves the health check on addr meanwhile, so that Docker Compose and
// ECS watch a worker with the command that watches an API (Main).
//
// The health check says that the process runs. A worker that cannot reach its
// database or its broker stays healthy and tries again: restarting it would
// not bring them back.
//
// RunWorker returns when work returns or when the listener fails, and stops
// the other one first.
func RunWorker(ctx context.Context, addr string, logger *slog.Logger, work func(context.Context) error) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	health := http.NewServeMux()
	handleHealth(health)
	served := make(chan error, 1)
	go func() {
		defer cancel()
		served <- Run(ctx, addr, health, logger)
	}()

	workErr := work(ctx)
	cancel()
	return errors.Join(workErr, <-served)
}
