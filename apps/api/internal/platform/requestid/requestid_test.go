package requestid

import (
	"context"
	"strings"
	"testing"
)

func TestValid(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		id   string
		want bool
	}{
		{name: "printable", id: "abc-123", want: true},
		{name: "longest allowed", id: strings.Repeat("x", 128), want: true},
		{name: "empty", id: "", want: false},
		{name: "too long", id: strings.Repeat("x", 129), want: false},
		{name: "space", id: "a b", want: false},
		{name: "control character", id: "a\tb", want: false},
		{name: "non-ASCII", id: "café", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := Valid(tt.id); got != tt.want {
				t.Fatalf("Valid(%q) = %v, want %v", tt.id, got, tt.want)
			}
		})
	}
}

func TestNew(t *testing.T) {
	t.Parallel()
	a, b := New(), New()
	if len(a) != 32 || !Valid(a) {
		t.Fatalf("New() = %q, want 32 valid characters", a)
	}
	if strings.Trim(a, "0123456789abcdef") != "" {
		t.Fatalf("New() = %q, want lowercase hexadecimal", a)
	}
	if a == b {
		t.Fatalf("New() returned %q twice", a)
	}
}

func TestWithFrom(t *testing.T) {
	t.Parallel()
	if got := From(context.Background()); got != "" {
		t.Fatalf("From(empty context) = %q, want \"\"", got)
	}
	if got := From(With(context.Background(), "req-1")); got != "req-1" {
		t.Fatalf("From(With(req-1)) = %q", got)
	}
}
