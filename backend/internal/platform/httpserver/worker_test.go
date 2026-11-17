package httpserver

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"testing"
	"time"
)

// freeAddr returns an address on the loopback interface that nothing listens
// on now.
func freeAddr(t *testing.T) string {
	t.Helper()
	l, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	return addr
}

func waitHealthy(t *testing.T, addr string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		err := CheckHealth(t.Context(), addr)
		if err == nil {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("the worker did not become healthy: %v", err)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestRunWorkerIsHealthyWhileTheWorkRuns(t *testing.T) {
	addr := freeAddr(t)
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() {
		done <- RunWorker(ctx, addr, slog.New(slog.DiscardHandler), func(ctx context.Context) error {
			<-ctx.Done()
			return nil
		})
	}()
	waitHealthy(t, addr)

	cancel()
	if err := <-done; err != nil {
		t.Errorf("RunWorker = %v, want nil after the cancellation", err)
	}
	if err := CheckHealth(t.Context(), addr); err == nil {
		t.Error("the health check still answers after the worker stopped")
	}
}

func TestRunWorkerReturnsTheErrorOfTheWork(t *testing.T) {
	failing := errors.New("the queue is gone")
	err := RunWorker(t.Context(), freeAddr(t), slog.New(slog.DiscardHandler), func(context.Context) error {
		return failing
	})
	if !errors.Is(err, failing) {
		t.Errorf("RunWorker = %v, want the error of the work", err)
	}
}

func TestRunWorkerStopsTheWorkWhenItCannotListen(t *testing.T) {
	taken, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = taken.Close() }()

	err = RunWorker(t.Context(), taken.Addr().String(), slog.New(slog.DiscardHandler), func(ctx context.Context) error {
		<-ctx.Done()
		return nil
	})
	if err == nil {
		t.Error("RunWorker = nil, want the error of the listener")
	}
}
