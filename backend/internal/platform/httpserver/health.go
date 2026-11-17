package httpserver

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

// HealthPath is the path of the health check that NewHandler serves next to
// the API. It is not an operation of the API.
const HealthPath = "/healthz"

// The address that a service listens on: the environment variable that holds
// it, and the address when the variable is empty.
const (
	AddrVariable = "HTTP_ADDR"
	DefaultAddr  = ":8080"
)

// healthCheckTimeout bounds one health check, below the timeouts that Docker
// Compose and ECS give the command.
const healthCheckTimeout = 2 * time.Second

// CheckHealth asks the service that listens on addr, on this host, whether it
// is healthy: GET HealthPath must answer with a 2xx status. addr is the
// address that the service listens on, such as ":8080"; an address without a
// host, or with an unspecified one, is reached over the loopback interface.
func CheckHealth(ctx context.Context, addr string) error {
	target, err := healthURL(addr)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return err
	}
	// No proxy: the service runs on this host.
	client := &http.Client{Transport: &http.Transport{Proxy: nil}, Timeout: healthCheckTimeout}
	res, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("the health check failed: %w", err)
	}
	defer func() { _ = res.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 1<<10))
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return fmt.Errorf("the health check failed: GET %s answered %s", target, res.Status)
	}
	return nil
}

// healthURL returns the URL of the health check of a service that listens on
// addr.
func healthURL(addr string) (string, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "", fmt.Errorf("the address %q is not host:port: %w", addr, err)
	}
	if ip := net.ParseIP(host); host == "" || (ip != nil && ip.IsUnspecified()) {
		host = "127.0.0.1"
		if ip != nil && ip.To4() == nil {
			host = "::1"
		}
	}
	return "http://" + net.JoinHostPort(host, port) + HealthPath, nil
}
