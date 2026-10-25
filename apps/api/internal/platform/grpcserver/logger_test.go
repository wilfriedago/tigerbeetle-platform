package grpcserver

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestSlogLogger(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	l := slogLogger{logger: slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))}
	l.Infof("channel %d ready", 3)
	l.Warning("slow", " peer")
	l.Errorln("closed")
	out := buf.String()
	for _, want := range []string{`level=DEBUG msg="channel 3 ready"`, `level=WARN msg="slow peer"`, `level=ERROR msg=closed`} {
		if !strings.Contains(out, want) {
			t.Errorf("log %q lacks %q", out, want)
		}
	}
	if l.V(1) || !l.V(0) {
		t.Error("verbose levels above 0 must be off")
	}
}
