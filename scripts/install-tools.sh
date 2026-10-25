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
# renovate: datasource=github-releases depName=oasdiff/oasdiff
OASDIFF_VERSION="v1.34.1"
# renovate: datasource=github-releases depName=bufbuild/buf
BUF_VERSION="v1.75.0"
# protoc-gen-go must equal the google.golang.org/protobuf version in apps/api/go.mod.
# renovate: datasource=go depName=google.golang.org/protobuf
PROTOC_GEN_GO_VERSION="v1.36.12"
# renovate: datasource=go depName=google.golang.org/grpc/cmd/protoc-gen-go-grpc
PROTOC_GEN_GO_GRPC_VERSION="v1.6.2"

for tool in "$@"; do
  case "$tool" in
    golangci-lint | sqlc | just | oasdiff | govulncheck | buf | protoc-gen-go | protoc-gen-go-grpc) ;;
    *)
      echo "unknown tool: $tool (expected golangci-lint, sqlc, just, oasdiff, govulncheck, buf, protoc-gen-go or protoc-gen-go-grpc)" >&2
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

if wanted oasdiff; then
  echo "==> oasdiff ${OASDIFF_VERSION}"
  if [[ "$(installed_version oasdiff --version)" == *"${OASDIFF_VERSION#v}"* ]]; then
    echo "    already installed"
  else
    # The release publishes checksums.txt; the digests are pinned here. The macOS archive is one
    # universal binary for both architectures. Update them together with OASDIFF_VERSION.
    case "${os}_${arch}" in
      darwin_amd64 | darwin_arm64) oasdiff_target="darwin_all" oasdiff_sha256="64948ba30c5555d3d15b95a7f3eda22fa83c15cf37fd4dad1720c0deb0a07cb5" ;;
      linux_amd64) oasdiff_target="linux_amd64" oasdiff_sha256="75775f65b0adebd26a8e013af51a799342dfdc6262d89efa9e3b7bdf89657d90" ;;
      linux_arm64) oasdiff_target="linux_arm64" oasdiff_sha256="e8f99bbf102a4b7f235104da72d0b9d67702e78622d1d4d37169e698ee90a249" ;;
      *)
        echo "no pinned oasdiff checksum for ${os}_${arch}" >&2
        exit 1
        ;;
    esac
    archive="oasdiff_${OASDIFF_VERSION#v}_${oasdiff_target}.tar.gz"
    curl -sSfL "https://github.com/oasdiff/oasdiff/releases/download/${OASDIFF_VERSION}/${archive}" -o "${tmp}/${archive}"
    actual_sha256="$(sha256_of "${tmp}/${archive}")"
    if [[ "$actual_sha256" != "$oasdiff_sha256" ]]; then
      echo "oasdiff checksum mismatch for ${archive}: got ${actual_sha256}, want ${oasdiff_sha256}" >&2
      exit 1
    fi
    tar -xzf "${tmp}/${archive}" -C "$tmp" oasdiff
    install -m 0755 "${tmp}/oasdiff" "${bin_dir}/oasdiff"
  fi
fi

if wanted buf; then
  echo "==> buf ${BUF_VERSION}"
  if [[ "$(installed_version buf --version)" == "${BUF_VERSION#v}" ]]; then
    echo "    already installed"
  else
    # The release publishes sha256.txt; the digests are pinned here. Update them together with
    # BUF_VERSION. Asset names use the uname spelling of the platform.
    case "${os}_${arch}" in
      darwin_amd64) buf_target="Darwin-x86_64" buf_sha256="d04292081e76c19c613d2411b2d0b3619d6d49d54ce6cbcf9e12f72edce0532b" ;;
      darwin_arm64) buf_target="Darwin-arm64" buf_sha256="5478531c4645c5b80866e87d76d73f0e8bf5a35644b12c92cbdcc408e7c8b9d3" ;;
      linux_amd64) buf_target="Linux-x86_64" buf_sha256="7168a06e29bd5eb8d1530848f242264f7a86b9b8af3b2e811570fbea0c929a30" ;;
      linux_arm64) buf_target="Linux-aarch64" buf_sha256="0fe2cf43cff973e27a09219e611d98101cb22b0fee398f10f2be2256dd7f892e" ;;
      *)
        echo "no pinned buf checksum for ${os}_${arch}" >&2
        exit 1
        ;;
    esac
    archive="buf-${buf_target}.tar.gz"
    curl -sSfL "https://github.com/bufbuild/buf/releases/download/${BUF_VERSION}/${archive}" -o "${tmp}/${archive}"
    actual_sha256="$(sha256_of "${tmp}/${archive}")"
    if [[ "$actual_sha256" != "$buf_sha256" ]]; then
      echo "buf checksum mismatch for ${archive}: got ${actual_sha256}, want ${buf_sha256}" >&2
      exit 1
    fi
    tar -xzf "${tmp}/${archive}" -C "$tmp" buf/bin/buf
    install -m 0755 "${tmp}/buf/bin/buf" "${bin_dir}/buf"
  fi
fi

if wanted protoc-gen-go; then
  echo "==> protoc-gen-go ${PROTOC_GEN_GO_VERSION}"
  if [[ "$(installed_version protoc-gen-go --version)" == "protoc-gen-go ${PROTOC_GEN_GO_VERSION}" ]]; then
    echo "    already installed"
  else
    # The release assets have no checksum file; the digests are pinned here. Update them together
    # with PROTOC_GEN_GO_VERSION (sha256sum of each protoc-gen-go.<ver>.<os>.<arch>.tar.gz).
    case "${os}_${arch}" in
      darwin_amd64) pgg_sha256="1df5af4a047a54fb15b1f9c4a174562a692f55cab13e94b8fbf274e766800314" ;;
      darwin_arm64) pgg_sha256="951d1d2558bbbc8c6680dcb6d302263fa35302cb6e1fccad297a50d21d827f2b" ;;
      linux_amd64) pgg_sha256="14abe70d1557026d3dce6676b7a44d3ada91d79902c71cb075fbb8d315943320" ;;
      linux_arm64) pgg_sha256="bf385e22e18661b044f7b96ec28384f3f76aa6da10e956b450ffb949306768de" ;;
      *)
        echo "no pinned protoc-gen-go checksum for ${os}_${arch}" >&2
        exit 1
        ;;
    esac
    archive="protoc-gen-go.${PROTOC_GEN_GO_VERSION}.${os}.${arch}.tar.gz"
    curl -sSfL "https://github.com/protocolbuffers/protobuf-go/releases/download/${PROTOC_GEN_GO_VERSION}/${archive}" -o "${tmp}/${archive}"
    actual_sha256="$(sha256_of "${tmp}/${archive}")"
    if [[ "$actual_sha256" != "$pgg_sha256" ]]; then
      echo "protoc-gen-go checksum mismatch for ${archive}: got ${actual_sha256}, want ${pgg_sha256}" >&2
      exit 1
    fi
    tar -xzf "${tmp}/${archive}" -C "$tmp" protoc-gen-go
    install -m 0755 "${tmp}/protoc-gen-go" "${bin_dir}/protoc-gen-go"
  fi
fi

if wanted protoc-gen-go-grpc; then
  echo "==> protoc-gen-go-grpc ${PROTOC_GEN_GO_GRPC_VERSION}"
  if [[ "$(installed_version protoc-gen-go-grpc --version)" == "protoc-gen-go-grpc ${PROTOC_GEN_GO_GRPC_VERSION#v}" ]]; then
    echo "    already installed"
  else
    # Released under its own tag line, cmd/protoc-gen-go-grpc/<ver>, with no checksum file; the
    # digests are pinned here. Update them together with PROTOC_GEN_GO_GRPC_VERSION.
    case "${os}_${arch}" in
      darwin_amd64) pggg_sha256="e891ebaf91f5ac959234cf9b323fecb14947f8b6f989989c50023da55bd55af3" ;;
      darwin_arm64) pggg_sha256="eefe7ad5e1d578c5d1124a9084333cb938274b7a2eb54c006e7769530119cba3" ;;
      linux_amd64) pggg_sha256="3d36f376624c73eb62c7a46e34c2b21a1e97684fb1812e882ba775ba05b7bfc9" ;;
      linux_arm64) pggg_sha256="343307d42a93c0219692302296e5fa38b89a9563732511b18a93479be4b0f43d" ;;
      *)
        echo "no pinned protoc-gen-go-grpc checksum for ${os}_${arch}" >&2
        exit 1
        ;;
    esac
    archive="protoc-gen-go-grpc.${PROTOC_GEN_GO_GRPC_VERSION}.${os}.${arch}.tar.gz"
    curl -sSfL "https://github.com/grpc/grpc-go/releases/download/cmd%2Fprotoc-gen-go-grpc%2F${PROTOC_GEN_GO_GRPC_VERSION}/${archive}" -o "${tmp}/${archive}"
    actual_sha256="$(sha256_of "${tmp}/${archive}")"
    if [[ "$actual_sha256" != "$pggg_sha256" ]]; then
      echo "protoc-gen-go-grpc checksum mismatch for ${archive}: got ${actual_sha256}, want ${pggg_sha256}" >&2
      exit 1
    fi
    tar -xzf "${tmp}/${archive}" -C "$tmp" ./protoc-gen-go-grpc
    install -m 0755 "${tmp}/protoc-gen-go-grpc" "${bin_dir}/protoc-gen-go-grpc"
  fi
fi

if wanted govulncheck; then
  echo "==> govulncheck ${GOVULNCHECK_VERSION}"
  GOFLAGS= go install "golang.org/x/vuln/cmd/govulncheck@${GOVULNCHECK_VERSION}"
fi

echo
echo "Tools installed in ${bin_dir}. Make sure it is on your PATH."
