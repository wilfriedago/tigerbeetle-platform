package web

import "testing"

func TestReturnToStaysOnTheConsole(t *testing.T) {
	tests := map[string]string{
		"/acme/ledgers?tab=1":   "/acme/ledgers?tab=1",
		"":                      "/",
		"//evil.example/x":      "/",
		"https://evil.example/": "/",
		"/\\evil.example":       "/",
		"acme":                  "/",
	}
	for in, want := range tests {
		if got := returnTo(in); got != want {
			t.Errorf("returnTo(%q) = %q, want %q", in, got, want)
		}
	}
}
