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
RUN apk add --no-cache file gcc git linux-headers musl-dev
WORKDIR /src

FROM base AS version
ARG GIT_FETCH_HISTORY=0
RUN --mount=target=.,rw <<'EOF'
  set -eu
  if [ "$GIT_FETCH_HISTORY" = 1 ]; then
    if [ "$(git rev-parse --is-shallow-repository)" = true ]; then
      git fetch --unshallow --tags origin
    else
      git fetch --tags origin
    fi
  fi
  version=$(git describe --match 'v[0-9]*' --dirty='.m' --always --tags)
  echo "$version" | tee /tmp/.version
EOF

FROM base AS build-base
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
  go mod download

FROM build-base AS mod-tidy
RUN --mount=type=bind,target=.,rw \
    --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
  go mod tidy && \
  mkdir /out && cp go.mod go.sum /out/

FROM scratch AS mod-update
COPY --from=mod-tidy /out/ /

FROM mod-tidy AS mod-validate
RUN --mount=type=bind,target=/context <<'EOF'
  set -eu
  status=0
  for path in go.mod go.sum; do
    diff -u "/context/$path" "/out/$path" || status=1
  done
  if [ "$status" -ne 0 ]; then
    echo >&2 'ERROR: Module files differ. Run "docker buildx bake mod-update".'
    exit 1
  fi
EOF

FROM build-base AS update-sources
RUN --mount=type=bind,target=.,rw \
    --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build <<EOT
  set -eu
  go run ./hack/update
  mkdir -p /out/internal
  cp go.mod go.sum /out/
  cp -r internal/gopls internal/xtools /out/internal/
EOT

FROM scratch AS update-modules
COPY --from=update-sources /out/go.mod /out/go.sum /

FROM scratch AS update-gopls
COPY --from=update-sources /out/internal/gopls /

FROM scratch AS update-xtools
COPY --from=update-sources /out/internal/xtools /

FROM update-sources AS update-validate
RUN --mount=type=bind,target=/context <<'EOF'
  set -eu
  status=0
  for path in go.mod go.sum internal/gopls internal/xtools; do
    diff -ru "/context/$path" "/out/$path" || status=1
  done
  if [ "$status" -ne 0 ]; then
    echo >&2 'ERROR: Generated sources differ. Run "docker buildx bake --allow=buildx.local.delete update".'
    exit 1
  fi
EOF

FROM build-base AS test
RUN --mount=type=bind,target=. \
    --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
  mkdir /tmp/coverage && \
  GOCOVERDIR=/tmp/coverage go test -v -count=1 -race ./... && \
  go tool covdata textfmt -i=/tmp/coverage -o=/tmp/coverage.txt && \
  go tool cover -func=/tmp/coverage.txt

FROM scratch AS test-coverage
COPY --from=test /tmp/coverage.txt /

FROM build-base AS lint
RUN --mount=type=bind,target=. \
    --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache \
    --mount=from=golangci-lint,source=/usr/bin/golangci-lint,target=/usr/bin/golangci-lint \
  golangci-lint run ./cmd/... ./hack/...

FROM build-base AS build
ARG TARGETPLATFORM
ENV CGO_ENABLED=0
RUN --mount=type=bind,target=. \
    --mount=type=bind,from=version,source=/tmp/.version,target=/tmp/.version \
    --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
  xx-go build -trimpath -ldflags "-s -w -X main.version=$(cat /tmp/.version)" -o /out/ ./cmd/gopls-analyzers && \
  xx-verify /out/*

FROM scratch AS binary
ARG BUILDKIT_SBOM_SCAN_STAGE=true
COPY --from=build /out/ /

FROM --platform=$BUILDPLATFORM alpine:${ALPINE_VERSION} AS build-artifact
ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT
RUN --mount=type=bind,from=binary,target=/build <<'EOF'
  set -eu
  mkdir /out
  filename="gopls-analyzers_${TARGETOS}_${TARGETARCH}${TARGETVARIANT}"
  extension=""
  if [ "$TARGETOS" = "windows" ]; then
    extension=".exe"
  fi
  cp "/build/gopls-analyzers${extension}" "/out/${filename}${extension}"
EOF

FROM scratch AS artifact
COPY --from=build-artifact /out/ /

FROM scratch AS image
COPY --from=build /out/gopls-analyzers /usr/local/bin/gopls-analyzers
ENTRYPOINT ["/usr/local/bin/gopls-analyzers"]
