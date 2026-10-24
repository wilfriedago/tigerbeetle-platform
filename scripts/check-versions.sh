#!/usr/bin/env bash
# Fails when the TigerBeetle Go client version in apps/api/go.mod does not match the server image
# pinned in deploy/compose/compose.yaml. A client newer than the cluster is rejected by TigerBeetle
# at connection time, so the two must move together (Renovate groups them into one PR).
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

server="$(grep -Eo 'ghcr\.io/tigerbeetle/tigerbeetle:[0-9][^[:space:]"]*' "$root/deploy/compose/compose.yaml" | sort -u)"
if [[ "$(wc -l <<<"$server")" -ne 1 ]]; then
  echo "expected exactly one tigerbeetle image tag in compose.yaml, found:" >&2
  echo "$server" >&2
  exit 1
fi
server_version="${server##*:}"

client_version="$(cd "$root/apps/api" && go list -m -f '{{.Version}}' github.com/tigerbeetle/tigerbeetle-go)"
client_version="${client_version#v}"

# Known exceptions: client-only rebuilds published without a server release. Each entry is
# "<server>=<client>" and must be removed once a server release at or above the client exists.
# 0.17.10 only rebuilds the client's static libraries so they link on macOS 27; it speaks the
# 0.17.9 protocol and was verified against a 0.17.9 replica.
allowed_exceptions=("0.17.9=0.17.10")

echo "server image: $server_version"
echo "go client:    $client_version"

if [[ "$server_version" == "$client_version" ]]; then
  echo "versions match"
  exit 0
fi
for pair in "${allowed_exceptions[@]}"; do
  if [[ "$pair" == "${server_version}=${client_version}" ]]; then
    echo "versions differ but the pair is an allowed exception (see scripts/check-versions.sh)"
    exit 0
  fi
done

echo "TigerBeetle client ($client_version) does not match the server image ($server_version)." >&2
echo "Bump both together, or add an explicit exception in scripts/check-versions.sh." >&2
exit 1
