package problem

import (
	"regexp"
	"testing"
)

// kebab is the form of a reason: lowercase words joined by dashes, which is also the fragment
// GitHub gives the reason's heading in docs/problems.md.
var kebab = regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)*$`)

// TestCatalog checks what RFC 9457 section 4 asks of each definition and that the generic reason
// of every sentinel is defined with the status the sentinel maps to.
func TestCatalog(t *testing.T) {
	t.Parallel()
	seen := map[string]bool{}
	for _, d := range Definitions() {
		if seen[d.Reason] {
			t.Errorf("reason %q is defined twice", d.Reason)
		}
		seen[d.Reason] = true
		if !kebab.MatchString(d.Reason) {
			t.Errorf("reason %q is not kebab-case", d.Reason)
		}
		if d.Title == "" || d.Description == "" || d.Status < 400 || d.Status > 599 {
			t.Errorf("definition %+v lacks a title, a description or an error status", d)
		}
		if TitleFor(d.Reason) != d.Title {
			t.Errorf("TitleFor(%q) = %q, want %q", d.Reason, TitleFor(d.Reason), d.Title)
		}
	}
	for _, s := range sentinels {
		d, ok := definition(s.reason)
		if !ok {
			t.Errorf("the reason %q of a sentinel is not catalogued", s.reason)
			continue
		}
		if d.Status != s.status {
			t.Errorf("reason %q is catalogued with status %d, its sentinel maps to %d", s.reason, d.Status, s.status)
		}
	}
}
