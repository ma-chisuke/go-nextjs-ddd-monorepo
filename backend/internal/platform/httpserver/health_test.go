package httpserver

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

// listenAddr returns the address of a running server, as HTTP_ADDR would
// name it.
func listenAddr(t *testing.T, handler http.Handler) string {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv.Listener.Addr().String()
}

// apiHandler is the handler of an API without operations, as NewHandler
// builds it for every context.
func apiHandler(t *testing.T, logger *slog.Logger) http.Handler {
	t.Helper()
	doc := &openapi3.T{
		OpenAPI: "3.1.0",
		Info:    &openapi3.Info{Title: "Empty", Version: "0.1.0"},
		Paths:   openapi3.NewPaths(),
	}
	handler, err := NewHandler(doc, logger, func(*http.ServeMux) {})
	if err != nil {
		t.Fatal(err)
	}
	return handler
}

func TestCheckHealthAsksTheHandlerOfAnAPI(t *testing.T) {
	var log bytes.Buffer
	addr := listenAddr(t, apiHandler(t, slog.New(slog.NewJSONHandler(&log, nil))))
	if err := CheckHealth(t.Context(), addr); err != nil {
		t.Fatalf("CheckHealth(%q) = %v, want nil", addr, err)
	}
	if log.Len() > 0 {
		t.Errorf("the health check was logged: %s", log.String())
	}
}

func TestCheckHealthFails(t *testing.T) {
	closed, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	unreachable := closed.Addr().String()
	if err := closed.Close(); err != nil {
		t.Fatal(err)
	}
	unhealthy := listenAddr(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	tests := []struct {
		name, addr, want string
	}{
		{"nothing listens", unreachable, "connection refused"},
		{"the service is unhealthy", unhealthy, "503 Service Unavailable"},
		{"the address has no port", "localhost", "is not host:port"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckHealth(t.Context(), tt.addr)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("CheckHealth(%q) = %v, want an error that says %q", tt.addr, err, tt.want)
			}
		})
	}
}

func TestHealthURL(t *testing.T) {
	tests := []struct{ addr, want string }{
		{":8080", "http://127.0.0.1:8080/healthz"},
		{"0.0.0.0:8080", "http://127.0.0.1:8080/healthz"},
		{"[::]:8080", "http://[::1]:8080/healthz"},
		{"localhost:9000", "http://localhost:9000/healthz"},
		{"10.0.0.5:8080", "http://10.0.0.5:8080/healthz"},
	}
	for _, tt := range tests {
		got, err := healthURL(tt.addr)
		if err != nil || got != tt.want {
			t.Errorf("healthURL(%q) = %q, %v, want %q", tt.addr, got, err, tt.want)
		}
	}
}

func TestCommand(t *testing.T) {
	healthy := listenAddr(t, apiHandler(t, slog.New(slog.DiscardHandler)))
	failing := errors.New("the database is gone")
	tests := []struct {
		name     string
		args     []string
		addr     string
		run      error
		wantCode int
		wantErr  string
		wantRun  bool
		wantMode bool
	}{
		{name: "runs the service", wantCode: 0, wantRun: true},
		{name: "reports the error of the service", run: failing, wantCode: 1, wantErr: "svc: the database is gone", wantRun: true},
		{name: "checks the health of a healthy service", args: []string{"healthcheck"}, addr: healthy, wantCode: 0},
		{name: "checks the health of a service that is down", args: []string{"healthcheck"}, addr: "127.0.0.1:1", wantCode: 1, wantErr: "svc: the health check failed"},
		{name: "runs a mode", args: []string{"relay"}, wantCode: 0, wantMode: true},
		{name: "reports the error of a mode", args: []string{"relay"}, run: failing, wantCode: 1, wantErr: "svc: the database is gone", wantMode: true},
		{name: "refuses another argument", args: []string{"serve"}, wantCode: 2, wantErr: "usage: svc [relay|healthcheck]"},
		{name: "refuses a second argument", args: []string{"relay", "now"}, wantCode: 2, wantErr: "usage: svc [relay|healthcheck]"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			getenv := func(name string) string {
				if name == AddrVariable {
					return tt.addr
				}
				return ""
			}
			ran := false
			run := func(context.Context, func(string) string, io.Writer) error {
				ran = true
				return tt.run
			}
			ranMode := false
			relay := Mode{Name: "relay", Run: func(context.Context, func(string) string, io.Writer) error {
				ranMode = true
				return tt.run
			}}
			var stderr bytes.Buffer
			code := command(t.Context(), "svc", tt.args, getenv, io.Discard, &stderr, run, relay)
			if code != tt.wantCode {
				t.Errorf("exit code = %d, want %d (stderr: %s)", code, tt.wantCode, stderr.String())
			}
			if !strings.Contains(stderr.String(), tt.wantErr) {
				t.Errorf("stderr = %q, want it to hold %q", stderr.String(), tt.wantErr)
			}
			if ran != tt.wantRun {
				t.Errorf("ran the service: %v, want %v", ran, tt.wantRun)
			}
			if ranMode != tt.wantMode {
				t.Errorf("ran the mode: %v, want %v", ranMode, tt.wantMode)
			}
		})
	}
}
