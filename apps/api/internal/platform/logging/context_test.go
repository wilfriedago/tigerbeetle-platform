package logging

import (
	"context"
	"io"
	"log/slog"
	"testing"
)

func TestWithFrom(t *testing.T) {
	t.Parallel()
	if From(context.Background()) != slog.Default() {
		t.Fatal("From(empty context) is not slog.Default")
	}
	l := slog.New(slog.NewTextHandler(io.Discard, nil))
	if From(With(context.Background(), l)) != l {
		t.Fatal("From(With(l)) did not return l")
	}
}
