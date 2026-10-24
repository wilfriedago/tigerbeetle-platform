# syntax=docker/dockerfile:1

# Build stage. The TigerBeetle Go client is a cgo binding around a static C library, so the
# build needs gcc and glibc. Debian 13 (trixie) here must match the runtime image below: a binary
# linked against glibc 2.41 does not start on an older glibc.
FROM golang:1.27.3-trixie AS build

ARG VERSION=dev
WORKDIR /src
ENV CGO_ENABLED=1 GOFLAGS=-mod=readonly GOTOOLCHAIN=local

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" -o /out/api ./cmd/api

# Runtime stage. Distroless glibc base (Debian 13, same glibc as the builder), no shell, no
# package manager, runs as the unprivileged nonroot user (uid 65532). Alpine would break the cgo
# build (musl) and scratch lacks libc, libm and libdl that the TigerBeetle client links.
FROM gcr.io/distroless/base-debian13:nonroot

COPY --from=build /out/api /app/api
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/app/api"]
