package logging

import (
	"context"
	"log/slog"
)

// contextKey is unexported so no other package can read or overwrite the logger by accident.
type contextKey struct{}

// With returns a copy of ctx that carries logger. Transports store a logger that already carries
// the request ID, so every line written while handling a call includes it.
func With(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, contextKey{}, logger)
}

// From returns the logger carried by ctx, or slog.Default outside a request.
func From(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(contextKey{}).(*slog.Logger); ok {
		return l
	}
	return slog.Default()
}
