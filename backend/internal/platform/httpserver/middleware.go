package httpserver

import (
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/problem"
)

// statusRecorder remembers the status code that a handler wrote.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

// WriteHeader remembers the status code and passes it on.
func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

// Write passes the body on. A handler that writes a body without a status
// code answers 200.
func (r *statusRecorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	return r.ResponseWriter.Write(b)
}

// AccessLog writes one log record per request.
func AccessLog(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w}
			next.ServeHTTP(rec, r)
			logger.InfoContext(r.Context(), "request",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", rec.status),
				slog.Duration("duration", time.Since(start)),
			)
		})
	}
}

// Recover turns a panic in a handler into an internal error problem and logs
// the panic with its stack. The response says nothing about the cause.
func Recover(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			defer func() {
				if v := recover(); v != nil {
					logger.ErrorContext(ctx, "panic in a handler",
						slog.Any("panic", v),
						slog.String("stack", string(debug.Stack())),
					)
					problem.Write(w, problem.New(problem.InternalError, ""))
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// RequestError is the handler that the generated strict server calls when it
// cannot read a request: a parameter or a body that does not decode.
func RequestError(w http.ResponseWriter, _ *http.Request, err error) {
	problem.Write(w, problem.New(problem.MalformedRequest, err.Error()))
}

// ResponseError returns the handler that the generated strict server calls
// when an operation returns an error. A problem is written as it is. Any
// other error is logged and becomes an internal error.
func ResponseError(logger *slog.Logger) func(http.ResponseWriter, *http.Request, error) {
	return func(w http.ResponseWriter, r *http.Request, err error) {
		p, ok := problem.From(err)
		if !ok {
			logger.ErrorContext(r.Context(), "operation failed", slog.Any("error", err))
		}
		problem.Write(w, p)
	}
}
