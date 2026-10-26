package problem

import (
	"regexp"
	"slices"
	"strings"
)

// ErrorReason returns the machine reason of the problem in the form Google's AIP-193 gives
// google.rpc.ErrorInfo.reason: the reason in UPPER_SNAKE_CASE, so "exceeds-credits" gives
// EXCEEDS_CREDITS, and the kind for an about:blank problem. Every transport that needs such a
// reason calls this one function, so it never drifts from the type.
func (p Problem) ErrorReason() string {
	if p.Reason == "" {
		return string(p.Kind)
	}
	return ErrorReasonFor(p.Reason)
}

// ErrorReasonFor returns the AIP-193 reason of a problem reason: "write-pending" gives
// WRITE_PENDING.
func ErrorReasonFor(reason string) string {
	return strings.ToUpper(strings.ReplaceAll(reason, "-", "_"))
}

// aipReason is the pattern AIP-193 sets for ErrorInfo.reason; the reason is also at most 63
// characters long.
var aipReason = regexp.MustCompile(`^[A-Z][A-Z0-9_]+[A-Z0-9]$`)

// ValidErrorReason reports whether reason follows AIP-193.
func ValidErrorReason(reason string) bool {
	return len(reason) <= 63 && aipReason.MatchString(reason)
}

// KnownReasons lists the reasons this package names: the catalogued ones, which include the
// generic reason of each sentinel. Reasons derived from TigerBeetle results are listed by the
// module that derives them.
func KnownReasons() []string {
	out := make([]string, 0, len(catalog)+len(sentinels))
	for _, d := range catalog {
		out = append(out, d.Reason)
	}
	for _, s := range sentinels {
		out = append(out, s.reason)
	}
	slices.Sort(out)
	return slices.Compact(out)
}
