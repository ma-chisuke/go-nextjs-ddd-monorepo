// Package httpserver runs the HTTP API of one bounded context: the server and
// its shutdown, the middleware every API shares, and the health check.
package httpserver

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
)

// Routes registers the operations of an API on mux. The generated code of
// each context provides it (HandlerFromMux), so that no operation exists
// outside the API document (TPL-0007).
type Routes func(mux *http.ServeMux)

// NewHandler returns the handler of one API: the generated routes behind
// request validation, with the access log and panic recovery around them, and
// the health check next to them.
func NewHandler(doc *openapi3.T, logger *slog.Logger, routes Routes) (http.Handler, error) {
	validate, err := ValidateRequests(doc)
	if err != nil {
		return nil, err
	}
	api := http.NewServeMux()
	routes(api)

	root := http.NewServeMux()
	// The health check is not an operation of the API: it belongs to the
	// platform that runs the service. It answers without touching the
	// database, so that it never keeps a paused database awake (TPL-0016).
	// Docker Compose and ECS call it every few seconds, so it stays out of
	// the access log.
	root.HandleFunc("GET "+HealthPath, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	root.Handle("/", AccessLog(logger)(Recover(logger)(validate(api))))
	return root, nil
}

// Server timeouts. A slow or stalled client must not hold a connection open.
const (
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 15 * time.Second
	writeTimeout      = 30 * time.Second
	idleTimeout       = 60 * time.Second
	shutdownTimeout   = 20 * time.Second
)

// Run serves handler on addr until ctx is canceled, then lets requests in
// flight finish before it returns.
func Run(ctx context.Context, addr string, handler http.Handler, logger *slog.Logger) error {
	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
		BaseContext:       func(net.Listener) context.Context { return context.WithoutCancel(ctx) },
	}
	failed := make(chan error, 1)
	go func() {
		logger.InfoContext(ctx, "listening", slog.String("addr", addr))
		failed <- srv.ListenAndServe()
	}()
	select {
	case err := <-failed:
		return err
	case <-ctx.Done():
	}
	logger.InfoContext(ctx, "shutting down")
	stop, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(stop); err != nil {
		return err
	}
	if err := <-failed; !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
