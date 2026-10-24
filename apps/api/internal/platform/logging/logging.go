// Package logging builds the process logger: log/slog with the JSON handler, so every line is
// machine-readable and carries the attributes added by the HTTP request middleware.
package logging

import (
	"io"
	"log/slog"
)

// New returns a JSON logger writing to w at the given level.
func New(w io.Writer, level slog.Level) *slog.Logger {
	return slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level}))
}
