// Package config reads the configuration of a service from environment
// variables (the Twelve-Factor App, factor III).
package config

import (
	"fmt"
	"strings"
)

// Reader reads variables and remembers the required ones that are missing,
// so that a service reports all of them at once when it starts.
type Reader struct {
	getenv  func(string) string
	missing []string
}

// NewReader returns a Reader over getenv, which is os.Getenv outside tests.
func NewReader(getenv func(string) string) *Reader {
	return &Reader{getenv: getenv}
}

// Required returns the value of the variable name, and notes it as missing
// when it is empty.
func (r *Reader) Required(name string) string {
	v := r.getenv(name)
	if v == "" {
		r.missing = append(r.missing, name)
	}
	return v
}

// Default returns the value of the variable name, or fallback when it is empty.
func (r *Reader) Default(name, fallback string) string {
	if v := r.getenv(name); v != "" {
		return v
	}
	return fallback
}

// Err returns an error that names every required variable that was missing.
func (r *Reader) Err() error {
	if len(r.missing) == 0 {
		return nil
	}
	return fmt.Errorf("config: set the environment variables %s", strings.Join(r.missing, ", "))
}
