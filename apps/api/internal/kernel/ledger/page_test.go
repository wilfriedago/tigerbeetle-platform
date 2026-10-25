package ledger

import (
	"errors"
	"testing"
)

func TestPageSize(t *testing.T) {
	t.Parallel()
	for requested, want := range map[int]int{-1: 25, 0: 25, 1: 1, 100: 100, 101: 100} {
		if got := PageSize(requested); got != want {
			t.Errorf("PageSize(%d) = %d, want %d", requested, got, want)
		}
	}
}

func TestCursors(t *testing.T) {
	t.Parallel()
	key, err := DecodeCursor(EncodeCursor("users:1"))
	if err != nil || key != "users:1" {
		t.Fatalf("round trip = %q, %v", key, err)
	}
	if key, err := DecodeCursor(""); err != nil || key != "" {
		t.Fatalf("empty cursor = %q, %v", key, err)
	}
	if _, err := DecodeCursor("not a cursor!"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("bad cursor error = %v", err)
	}
	page, next := TrimPage([]string{"a", "b", "c"}, 2, func(s string) string { return s })
	if len(page) != 2 || next != EncodeCursor("b") {
		t.Fatalf("TrimPage = %v, %q", page, next)
	}
	if page, next := TrimPage([]string{"a"}, 2, func(s string) string { return s }); len(page) != 1 || next != "" {
		t.Fatalf("last page = %v, %q", page, next)
	}
}
