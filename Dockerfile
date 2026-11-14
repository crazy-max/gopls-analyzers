# syntax=docker/dockerfile:1

ARG GO_VERSION="1.27"
ARG ALPINE_VERSION="3.24"
ARG XX_VERSION="1.10.0"
ARG GOLANGCI_LINT_VERSION="v2.15.0"

FROM --platform=$BUILDPLATFORM tonistiigi/xx:${XX_VERSION} AS xx
FROM --platform=$BUILDPLATFORM golangci/golangci-lint:${GOLANGCI_LINT_VERSION}-alpine AS golangci-lint

FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-alpine${ALPINE_VERSION} AS base
ENV GOFLAGS="-buildvcs=false"
ENV GOTOOLCHAIN=local
COPY --from=xx / /
RUN apk add --no-cache file gcc linux-headers musl-dev
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .

FROM base AS test
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
  go test -v -race ./...

FROM base AS lint
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache \
    --mount=from=golangci-lint,source=/usr/bin/golangci-lint,target=/usr/bin/golangci-lint \
  golangci-lint run ./cmd/... ./hack/...

FROM base AS build
ARG TARGETPLATFORM
ENV CGO_ENABLED=0
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
  xx-go build -trimpath -o /out/ ./cmd/gopls-analyzers && \
  xx-verify /out/*

FROM scratch AS binary
COPY --from=build /out/ /

FROM scratch AS image
COPY --from=build /out/gopls-analyzers /usr/local/bin/gopls-analyzers
ENTRYPOINT ["/usr/local/bin/gopls-analyzers"]
