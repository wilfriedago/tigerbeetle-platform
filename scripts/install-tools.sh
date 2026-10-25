#!/usr/bin/env bash
# Installs the pinned developer tools that are not managed by go.mod or package.json.
# Used locally (just tools-install, or the script itself the first time, since it installs just)
# and in CI. Binaries land in $(go env GOPATH)/bin, or $HOME/go/bin when Go is absent.
#
# With no argument every tool is installed. Name tools to install only those, for example
# `scripts/install-tools.sh just` in CI jobs that only need to run recipes.
#
# Versions are pinned here and bumped by Renovate (see renovate.json customManagers).
set -euo pipefail

# renovate: datasource=github-releases depName=golangci/golangci-lint
GOLANGCI_LINT_VERSION="v2.15.0"
# renovate: datasource=github-releases depName=sqlc-dev/sqlc
SQLC_VERSION="v1.31.1"
# renovate: datasource=go depName=golang.org/x/vuln
GOVULNCHECK_VERSION="v1.9.0"
# renovate: datasource=github-releases depName=casey/just
JUST_VERSION="1.59.0"

for tool in "$@"; do
  case "$tool" in
    golangci-lint | sqlc | just | govulncheck) ;;
    *)
      echo "unknown tool: $tool (expected golangci-lint, sqlc, just or govulncheck)" >&2
      exit 1
      ;;
  esac
done
requested=" $* "

# wanted succeeds when the tool was named, or when no tool was named at all.
wanted() {
  [[ "$requested" == "  " || "$requested" == *" $1 "* ]]
}

if command -v go >/dev/null 2>&1; then
  bin_dir="$(go env GOPATH)/bin"
else
  bin_dir="$HOME/go/bin"
fi
mkdir -p "$bin_dir"
# In GitHub Actions, later steps of the job find the tools on PATH.
if [[ -n "${GITHUB_PATH:-}" ]]; then
  echo "$bin_dir" >>"$GITHUB_PATH"
fi

os="$(uname -s | tr '[:upper:]' '[:lower:]')"
arch="$(uname -m)"
case "$arch" in
  x86_64) arch="amd64" ;;
  aarch64 | arm64) arch="arm64" ;;
  *)
    echo "unsupported architecture: $arch" >&2
    exit 1
    ;;
esac

sha256_of() {
  # sha256sum comes with coreutils; older macOS only ships shasum.
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | cut -d' ' -f1
  else
    shasum -a 256 "$1" | cut -d' ' -f1
  fi
}

# One scratch directory for every download, removed on exit.
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

installed_version() {
  # Prints the version of an installed binary, or nothing when it is missing.
  if command -v "$1" >/dev/null 2>&1; then "$1" "${@:2}" 2>/dev/null | head -n 1; fi
}

if wanted golangci-lint; then
  echo "==> golangci-lint ${GOLANGCI_LINT_VERSION}"
  if [[ "$(installed_version golangci-lint version --short)" == "${GOLANGCI_LINT_VERSION#v}" ]]; then
    echo "    already installed"
  else
    curl -sSfL https://golangci-lint.run/install.sh | sh -s -- -b "$bin_dir" "$GOLANGCI_LINT_VERSION"
  fi
fi

if wanted sqlc; then
  echo "==> sqlc ${SQLC_VERSION}"
  if [[ "$(installed_version sqlc version)" == "$SQLC_VERSION" ]]; then
    echo "    already installed"
  else
    # sqlc publishes no checksum file, so the archive digests are pinned here. Update them together
    # with SQLC_VERSION (sha256sum of each sqlc_<ver>_<os>_<arch>.tar.gz release asset).
    case "${os}_${arch}" in
      darwin_amd64) sqlc_sha256="c5af76772e3785d21663a62697056b383f07629979b1bd25b93872e73dbd519b" ;;
      darwin_arm64) sqlc_sha256="21602158c99eb1f2bae197a66abfb1941d1e9e50b23125bb193349c6b1acc71e" ;;
      linux_amd64) sqlc_sha256="497ae4fcdfa64c5b0c311ffe4c2bd991e43991e82e5367792ed78bc2dca27354" ;;
      linux_arm64) sqlc_sha256="b7cae247740d0c51a1e657479e5b2d21e6fef428f596682a01bc55bf4ab8a23d" ;;
      *)
        echo "no pinned sqlc checksum for ${os}_${arch}" >&2
        exit 1
        ;;
    esac
    archive="sqlc_${SQLC_VERSION#v}_${os}_${arch}.tar.gz"
    curl -sSfL "https://github.com/sqlc-dev/sqlc/releases/download/${SQLC_VERSION}/${archive}" -o "${tmp}/${archive}"
    actual_sha256="$(sha256_of "${tmp}/${archive}")"
    if [[ "$actual_sha256" != "$sqlc_sha256" ]]; then
      echo "sqlc checksum mismatch for ${archive}: got ${actual_sha256}, want ${sqlc_sha256}" >&2
      exit 1
    fi
    tar -xzf "${tmp}/${archive}" -C "$tmp" sqlc
    install -m 0755 "${tmp}/sqlc" "${bin_dir}/sqlc"
  fi
fi

if wanted just; then
  echo "==> just ${JUST_VERSION}"
  if [[ "$(installed_version just --version)" == "just ${JUST_VERSION}" ]]; then
    echo "    already installed"
  else
    # The release publishes SHA256SUMS; the digests are pinned here so a changed asset fails the
    # install. Update them together with JUST_VERSION. Linux uses the static musl builds.
    case "${os}_${arch}" in
      darwin_amd64) just_target="x86_64-apple-darwin" just_sha256="036ead9df9bc31dbae9934ae3b4236ed124d0bafcffd33439460ae084d5c9cb7" ;;
      darwin_arm64) just_target="aarch64-apple-darwin" just_sha256="6574cea774ed4cae12e24b6aa11857de33e80dab4667cf628fd5840df96a2a9d" ;;
      linux_amd64) just_target="x86_64-unknown-linux-musl" just_sha256="409044617b6f06517c1035e55ca38c8c856ff8b650e41bc2aa1ef82d5903a23e" ;;
      linux_arm64) just_target="aarch64-unknown-linux-musl" just_sha256="23871b1702bfd550fb5f901778fd4290ecf2ee034a45a889fbe96d89b188086d" ;;
      *)
        echo "no pinned just checksum for ${os}_${arch}" >&2
        exit 1
        ;;
    esac
    archive="just-${JUST_VERSION}-${just_target}.tar.gz"
    curl -sSfL "https://github.com/casey/just/releases/download/${JUST_VERSION}/${archive}" -o "${tmp}/${archive}"
    actual_sha256="$(sha256_of "${tmp}/${archive}")"
    if [[ "$actual_sha256" != "$just_sha256" ]]; then
      echo "just checksum mismatch for ${archive}: got ${actual_sha256}, want ${just_sha256}" >&2
      exit 1
    fi
    tar -xzf "${tmp}/${archive}" -C "$tmp" just
    install -m 0755 "${tmp}/just" "${bin_dir}/just"
  fi
fi

if wanted govulncheck; then
  echo "==> govulncheck ${GOVULNCHECK_VERSION}"
  GOFLAGS= go install "golang.org/x/vuln/cmd/govulncheck@${GOVULNCHECK_VERSION}"
fi

echo
echo "Tools installed in ${bin_dir}. Make sure it is on your PATH."
