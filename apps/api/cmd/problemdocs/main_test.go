package main

import (
	"bytes"
	"strings"
	"testing"

	bookkeepingdto "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/interfaces/dto"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/problem"
)

// TestBuild checks that the catalogue is byte-stable and has a heading for every reason a problem
// type URI can end with, so no type points at a missing entry.
func TestBuild(t *testing.T) {
	t.Parallel()
	first, err := build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	second, err := build()
	if err != nil || !bytes.Equal(first, second) {
		t.Fatal("the catalogue is not byte-stable")
	}
	doc := string(first)
	reasons := problem.KnownReasons()
	for _, d := range bookkeepingdto.ProblemTypes() {
		reasons = append(reasons, d.Reason)
	}
	for _, reason := range reasons {
		if !strings.Contains(doc, "## "+reason+"\n") {
			t.Errorf("no heading for %q", reason)
		}
		if !problem.ValidErrorReason(problem.ErrorReasonFor(reason)) {
			t.Errorf("reason %q breaks AIP-193", reason)
		}
	}
}
