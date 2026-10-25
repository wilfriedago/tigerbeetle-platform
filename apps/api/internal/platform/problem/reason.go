package problem

import (
	"regexp"
	"slices"
	"strings"
)

// ErrorReason returns the machine reason of the problem in the form Google's AIP-193 gives
// google.rpc.ErrorInfo.reason: the slug of its type in UPPER_SNAKE_CASE, so
// "/problems/exceeds-credits" gives EXCEEDS_CREDITS, and the kind for an about:blank problem. Every
// transport that needs such a reason calls this one function, so it never drifts from the type.
func (p Problem) ErrorReason() string {
	if p.Type() == TypeBlank {
		return string(p.Kind)
	}
	return ErrorReasonOf(p.Type())
}

// ErrorReasonOf returns the AIP-193 reason of a problem type URI: "/problems/write-pending" gives
// WRITE_PENDING.
func ErrorReasonOf(problemType string) string {
	slug := strings.TrimPrefix(problemType, typePrefix)
	return strings.ToUpper(strings.ReplaceAll(slug, "-", "_"))
}

// aipReason is the pattern AIP-193 sets for ErrorInfo.reason; the reason is also at most 63
// characters long.
var aipReason = regexp.MustCompile(`^[A-Z][A-Z0-9_]+[A-Z0-9]$`)

// ValidErrorReason reports whether reason follows AIP-193.
func ValidErrorReason(reason string) bool {
	return len(reason) <= 63 && aipReason.MatchString(reason)
}

// KnownReasons lists the reasons this package names: the titled ones and the generic reason of
// each sentinel. Reasons derived from TigerBeetle results are listed by the module that derives
// them.
func KnownReasons() []string {
	out := make([]string, 0, len(titles)+len(sentinels))
	for reason := range titles {
		out = append(out, reason)
	}
	for _, s := range sentinels {
		out = append(out, s.reason)
	}
	slices.Sort(out)
	return slices.Compact(out)
}
