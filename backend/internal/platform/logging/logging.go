// Package logging builds the logger of a service: structured records, one
// JSON object per line, on standard output (the Twelve-Factor App, factor XI).
package logging

import (
	"fmt"
	"io"
	"log/slog"
)

// New returns a JSON logger that writes to w. level is debug, info, warn, or
// error; service is added to every record.
func New(w io.Writer, level, service string) (*slog.Logger, error) {
	var l slog.Level
	if err := l.UnmarshalText([]byte(level)); err != nil {
		return nil, fmt.Errorf("logging: unknown level %q (use debug, info, warn, or error)", level)
	}
	handler := slog.NewJSONHandler(w, &slog.HandlerOptions{Level: l})
	return slog.New(handler).With(slog.String("service", service)), nil
}
