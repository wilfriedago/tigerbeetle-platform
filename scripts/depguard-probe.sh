#!/usr/bin/env bash
# Proves that every depguard rule of apps/api/.golangci.yml fires (plan section 14.2). Each fixture
# in apps/api/testdata/depguard breaks one rule, named on its "// probe:" line; this script lints
# each fixture package with depguard alone and fails unless the report names exactly that rule,
# once. It also fails when a rule has no fixture. A lint rule that matches nothing looks exactly
# like a rule that is obeyed; this is how the difference shows.
set -euo pipefail

cd "$(dirname "$0")/../apps/api"
fixtures=testdata/depguard
status=0

rules=$(awk '/^      rules:/{in_rules=1; next} in_rules && /^    [a-z]/{exit} in_rules && /^        [a-z-]+:$/{sub(/:$/, "", $1); print $1}' .golangci.yml)
probed=$(grep -rh '^// probe: ' "$fixtures" | sed 's|^// probe: ||' | sort -u)

for rule in $rules; do
	if ! grep -qx "$rule" <<<"$probed"; then
		echo "rule $rule has no fixture in apps/api/$fixtures" >&2
		status=1
	fi
done

while IFS= read -r file; do
	dir=$(dirname "$file")
	want=$(sed -n 's|^// probe: ||p' "$file")
	report=$(golangci-lint run --enable-only depguard "./$dir/" 2>&1 || true)
	found=$(grep -c '(depguard)$' <<<"$report" || true)
	if [[ "$found" != 1 ]] || ! grep -q "from list '$want'" <<<"$report"; then
		echo "$dir: want one finding of rule $want, got:" >&2
		echo "$report" >&2
		status=1
	else
		echo "ok   $want  $dir"
	fi
done < <(find "$fixtures" -name '*.go' | sort)

exit "$status"
