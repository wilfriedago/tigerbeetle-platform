#!/usr/bin/env bash
# Smoke test for the api image: it prints its version, runs as a non-root user, starts without
# its dependencies (Keycloak included), answers the liveness probe and reports the missing dependencies on /readyz.
# Usage: scripts/smoke-test.sh <image>
set -euo pipefail

image="${1:?usage: smoke-test.sh <image>}"
name="api-smoke-$$"
port="${SMOKE_PORT:-18080}"

cleanup() { docker rm -f "$name" >/dev/null 2>&1 || true; }
trap cleanup EXIT

echo "==> prints its version"
docker run --rm "$image" -version | grep "tigerbeetle-platform-api"

echo "==> runs as non-root"
user="$(docker inspect --format '{{.Config.User}}' "$image")"
echo "    user: $user"
[[ "$user" == "nonroot:nonroot" || "$user" == "nonroot" || "$user" == "65532:65532" || "$user" == "65532" ]] \
  || { echo "image does not run as nonroot" >&2; exit 1; }

echo "==> starts without its dependencies"
# The TigerBeetle client needs io_uring, which Docker 25+ blocks in its default seccomp profile.
docker run -d --name "$name" -p "${port}:8080" --security-opt seccomp=unconfined \
  -e POSTGRES_URL="postgres://nobody:nobody@127.0.0.1:1/nothing" \
  -e TIGERBEETLE_ADDRESSES="127.0.0.1:1" \
  -e AUTH_ISSUER_URL="http://127.0.0.1:1/realms/tbp" \
  -e AUTH_AUDIENCE="tbp-api" \
  "$image" >/dev/null

for _ in $(seq 1 30); do
  if body="$(curl -fsS "http://127.0.0.1:${port}/healthz" 2>/dev/null)"; then
    echo "    /healthz: $body"
    echo "==> /readyz reports the missing dependencies with 503"
    code="$(curl -s -o /dev/null -w '%{http_code}' "http://127.0.0.1:${port}/readyz")"
    [[ "$code" == "503" ]] || { echo "expected 503 from /readyz, got $code" >&2; docker logs "$name" >&2; exit 1; }
    echo "    503 as expected"
    echo "smoke test passed"
    exit 0
  fi
  sleep 1
done

echo "api did not answer /healthz in time" >&2
docker logs "$name" >&2
exit 1
