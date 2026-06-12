# syntax=docker/dockerfile:1.7
#
# Alpine-runtime multi-target Dockerfile for deploy/main.
#
# Why not reuse infra/docker/wallet-biz.Dockerfile?
# - That one uses distroless runtime (no /bin/sh).
# - deploy/main currently generates YAML configs from env vars in `sh -lc '...'`.
#
# Targets (final stage names):
#   api-gateway, admin-rpc, business-rpc, signer-rpc, chainrpc, chainsync,
#   swap-rpc, accounting-rpc, market, consolidation
#

ARG GO_VERSION=1.25.4
ARG ALPINE_VERSION=3.20

# ============================================================
# Builder base (shared)
# ============================================================
FROM golang:${GO_VERSION}-alpine AS builder-base

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /workspace

# Cache dependencies (go mod)
COPY go.mod go.sum ./
COPY lib/gotron-sdk-0.24.1/go.mod lib/gotron-sdk-0.24.1/go.sum ./lib/gotron-sdk-0.24.1/
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go mod download

# Copy source code
COPY . .

# ============================================================
# Build stages (one per service)
# ============================================================
FROM builder-base AS build-api-gateway
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w -X main.Version=$(git describe --tags --always 2>/dev/null || echo 'dev')" \
    -o /out/api-gateway ./api-gateway/gateway.go

FROM builder-base AS build-admin-rpc
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" \
    -o /out/admin-rpc ./services/admin/rpc/admin.go

FROM builder-base AS build-business-rpc
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" \
    -o /out/business-rpc ./services/business/rpc/business.go

FROM builder-base AS build-signer-rpc
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" \
    -o /out/signer-rpc ./services/signer/rpc/signer.go

FROM builder-base AS build-chainrpc
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" \
    -o /out/chainrpc ./services/chainrpc/rpc/chainrpc.go

FROM builder-base AS build-chainsync
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" \
    -o /out/chainsync ./services/chainsync/rpc/chainsync.go

FROM builder-base AS build-swap-rpc
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" \
    -o /out/swap-rpc ./services/swap/rpc/swap.go

FROM builder-base AS build-accounting-rpc
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" \
    -o /out/accounting-rpc ./services/accounting/rpc/accounting.go

FROM builder-base AS build-market
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" \
    -o /out/market ./services/market/rpc/market.go

FROM builder-base AS build-consolidation
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" \
    -o /out/consolidation ./services/consolidation/rpc/consolidation.go

# ============================================================
# Runtime base (shared)
# ============================================================
FROM alpine:${ALPINE_VERSION} AS runtime-base

RUN apk add --no-cache \
      bash \
      ca-certificates \
      tzdata \
      curl \
      wget \
    && addgroup -g 1000 app \
    && adduser -D -u 1000 -G app app \
    && mkdir -p /app \
    && chown -R app:app /app

WORKDIR /app
USER app

# ============================================================
# Final targets (one per service image)
# ============================================================
FROM runtime-base AS api-gateway
COPY --from=build-api-gateway /out/api-gateway /app/service
COPY api-gateway/routes.yaml /app/api-gateway/routes.yaml
COPY docs/swagger /app/docs/swagger
EXPOSE 8080
CMD ["/app/service"]

FROM runtime-base AS admin-rpc
COPY --from=build-admin-rpc /out/admin-rpc /app/service
EXPOSE 9013 9094
CMD ["/app/service"]

FROM runtime-base AS business-rpc
COPY --from=build-business-rpc /out/business-rpc /app/service
EXPOSE 8081 9091
CMD ["/app/service"]

FROM runtime-base AS signer-rpc
COPY --from=build-signer-rpc /out/signer-rpc /app/service
EXPOSE 9012 9092
CMD ["/app/service"]

FROM runtime-base AS chainrpc
COPY --from=build-chainrpc /out/chainrpc /app/service
EXPOSE 9005 9093
CMD ["/app/service"]

FROM runtime-base AS chainsync
COPY --from=build-chainsync /out/chainsync /app/service
EXPOSE 9001 9095
CMD ["/app/service"]

FROM runtime-base AS swap-rpc
COPY --from=build-swap-rpc /out/swap-rpc /app/service
EXPOSE 9014
CMD ["/app/service"]

FROM runtime-base AS accounting-rpc
COPY --from=build-accounting-rpc /out/accounting-rpc /app/service
EXPOSE 9015 9096
CMD ["/app/service"]

FROM runtime-base AS market
COPY --from=build-market /out/market /app/service
EXPOSE 18080
CMD ["/app/service"]

FROM runtime-base AS consolidation
COPY --from=build-consolidation /out/consolidation /app/service
EXPOSE 9016 9097
CMD ["/app/service"]

