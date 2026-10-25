package ledger

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	"time"
)

// Limit is the balance limit flag of a TigerBeetle account. TigerBeetle rejects an account with
// both limit flags, so a rule sets at most one (ADR-0006).
type Limit string

// Balance limits. With the posting source as the debit account (ADR-0007),
// debits_must_not_exceed_credits keeps credits minus debits, the user balance, from going below
// zero, counting pending debits (holds).
const (
	LimitNone                       Limit = "none"
	LimitDebitsMustNotExceedCredits Limit = "debits_must_not_exceed_credits" //nolint:gosec // G101: a TigerBeetle flag name, not a credential
	LimitCreditsMustNotExceedDebits Limit = "credits_must_not_exceed_debits"
)

// ParseLimit validates a limit name.
func ParseLimit(s string) (Limit, error) {
	switch l := Limit(s); l {
	case LimitNone, LimitDebitsMustNotExceedCredits, LimitCreditsMustNotExceedDebits:
		return l, nil
	default:
		return "", fmt.Errorf("%w: limit %q must be none, debits_must_not_exceed_credits or credits_must_not_exceed_debits", ErrInvalid, s)
	}
}

// Pattern selects the addresses a rule applies to (ADR-0006). It is one of:
//   - "*", the catch-all, which matches every address;
//   - an address such as "world", which matches exactly that address;
//   - an address followed by a colon such as "users:", which matches every address below it,
//     on whole segments, so "users:1:" matches "users:1:wallet" but not "users:12:wallet".
type Pattern string

// CatchAll is the pattern that matches every address.
const CatchAll Pattern = "*"

// ParsePattern validates and returns a pattern.
func ParsePattern(s string) (Pattern, error) {
	if s == string(CatchAll) {
		return CatchAll, nil
	}
	if _, err := ParseAddress(strings.TrimSuffix(s, ":")); err != nil {
		return "", fmt.Errorf("%w: pattern %q must be *, an address, or an address followed by a colon", ErrInvalid, s)
	}
	return Pattern(s), nil
}

// IsPrefix reports whether the pattern covers the addresses below it rather than one address.
func (p Pattern) IsPrefix() bool {
	return strings.HasSuffix(string(p), ":")
}

// Segments is the specificity of the pattern: the number of address segments it spells out. The
// catch-all counts zero.
func (p Pattern) Segments() int {
	if p == CatchAll {
		return 0
	}
	return len(Address(strings.TrimSuffix(string(p), ":")).Segments())
}

// Matches reports whether the pattern applies to the address.
func (p Pattern) Matches(a Address) bool {
	switch {
	case p == CatchAll:
		return true
	case p.IsPrefix():
		return strings.HasPrefix(string(a), string(p))
	default:
		return string(a) == string(p)
	}
}

// Rule decides the flags and code of the TigerBeetle accounts created for matching addresses. The
// values are frozen on each account at creation; changing a rule only affects accounts created
// afterwards (ADR-0006).
type Rule struct {
	Pattern Pattern
	Limit   Limit
	// History asks TigerBeetle to keep point-in-time balances (get_account_balances). It cannot be
	// added to an existing account.
	History bool
	// Code is the TigerBeetle account code, a non-zero u16.
	Code uint16
	// BuiltIn marks the default "*" and "world" rules, which exist without being stored.
	BuiltIn   bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Validate checks the fields a client controls.
func (r Rule) Validate() error {
	if _, err := ParsePattern(string(r.Pattern)); err != nil {
		return err
	}
	if _, err := ParseLimit(string(r.Limit)); err != nil {
		return err
	}
	if r.Code == 0 {
		return fmt.Errorf("%w: code must be between 1 and 65535; TigerBeetle rejects code 0", ErrInvalid)
	}
	return nil
}

// DefaultRules apply when a ledger stores nothing else: every account cannot go negative, and world,
// where money enters and leaves the ledger, has no limit (ADR-0006).
func DefaultRules() []Rule {
	return []Rule{
		{Pattern: CatchAll, Limit: LimitDebitsMustNotExceedCredits, Code: 1, BuiltIn: true},
		{Pattern: Pattern(World), Limit: LimitNone, Code: 2, BuiltIn: true},
	}
}

// EffectiveRules merges stored rules over the defaults (a stored "*" or "world" replaces the
// built-in one) and sorts them from most to least specific, then by pattern.
func EffectiveRules(stored []Rule) []Rule {
	byPattern := make(map[Pattern]Rule, len(stored)+2)
	for _, r := range DefaultRules() {
		byPattern[r.Pattern] = r
	}
	for _, r := range stored {
		byPattern[r.Pattern] = r
	}
	rules := make([]Rule, 0, len(byPattern))
	for _, r := range byPattern {
		rules = append(rules, r)
	}
	slices.SortFunc(rules, func(a, b Rule) int {
		if c := cmp.Compare(b.Pattern.Segments(), a.Pattern.Segments()); c != 0 {
			return c
		}
		return cmp.Compare(a.Pattern, b.Pattern)
	})
	return rules
}

// IsBuiltInPattern reports whether a pattern has a built-in default.
func IsBuiltInPattern(p Pattern) bool {
	return p == CatchAll || p == Pattern(World)
}

// RuleCandidate is one rule considered for an address, for the "which rule wins" view.
type RuleCandidate struct {
	Rule    Rule
	Matches bool
}

// MatchRule returns the most specific rule matching the address among the stored rules and the
// defaults, and every effective rule with whether it matches. Two distinct patterns that both match one address always have different
// segment counts, so there are no ties: exact patterns spell out every segment of the address,
// prefix patterns fewer. The catch-all guarantees a match.
func MatchRule(rules []Rule, a Address) (Rule, []RuleCandidate) {
	effective := EffectiveRules(rules)
	candidates := make([]RuleCandidate, 0, len(effective))
	var (
		winner Rule
		found  bool
	)
	for _, r := range effective {
		matches := r.Pattern.Matches(a)
		candidates = append(candidates, RuleCandidate{Rule: r, Matches: matches})
		if matches && !found {
			winner, found = r, true
		}
	}
	return winner, candidates
}
