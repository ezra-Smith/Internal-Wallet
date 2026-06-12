# syntax=docker/dockerfile:1.7
#
# Unified multi-target Dockerfile for wallet-biz services.
# Each final target name matches the service image name:
#   api-gateway, admin-rpc, business-rpc, signer-rpc, chainrpc, chainsync,
#   swap-rpc, accounting-rpc, market, consolidation
#
# Build examples:
#   docker build -f infra/docker/wallet-biz.Dockerfile --target admin-rpc -t internal-wallet/admin-rpc .
#

ARG GO_VERSION=1.25.4
ARG RUNTIME_IMAGE=gcr.io/distroless/static-debian12:nonroot

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
FROM ${RUNTIME_IMAGE} AS runtime-base

COPY --from=builder-base /usr/share/zoneinfo /usr/share/zoneinfo
ENV TZ=Asia/Tokyo

VOLUME ["/app/etc"]

USER nonroot:nonroot

ENTRYPOINT ["/app/service"]

# ============================================================
# Final targets (one per service image)
# ============================================================
FROM runtime-base AS api-gateway
COPY --from=build-api-gateway /out/api-gateway /app/service
EXPOSE 8080
CMD ["-f", "/app/etc/gateway.yaml"]

FROM runtime-base AS admin-rpc
COPY --from=build-admin-rpc /out/admin-rpc /app/service
EXPOSE 9014
CMD ["-f", "/app/etc/admin.yaml"]

FROM runtime-base AS business-rpc
COPY --from=build-business-rpc /out/business-rpc /app/service
EXPOSE 8081 9091
CMD ["-f", "/app/etc/business.yaml"]

FROM runtime-base AS signer-rpc
COPY --from=build-signer-rpc /out/signer-rpc /app/service
EXPOSE 8084
CMD ["-f", "/app/etc/signer.yaml"]

FROM runtime-base AS chainrpc
COPY --from=build-chainrpc /out/chainrpc /app/service
EXPOSE 8082
CMD ["-f", "/app/etc/chainrpc.yaml"]

FROM runtime-base AS chainsync
COPY --from=build-chainsync /out/chainsync /app/service
EXPOSE 8083
CMD ["-f", "/app/etc/chainsync.yaml"]

FROM runtime-base AS swap-rpc
COPY --from=build-swap-rpc /out/swap-rpc /app/service
EXPOSE 9017
CMD ["-f", "/app/etc/swap.yaml"]

FROM runtime-base AS accounting-rpc
COPY --from=build-accounting-rpc /out/accounting-rpc /app/service
EXPOSE 9015
CMD ["-f", "/app/etc/accounting.yaml"]

FROM runtime-base AS market
COPY --from=build-market /out/market /app/service
EXPOSE 8086
CMD ["-f", "/app/etc/market.yaml"]

FROM runtime-base AS consolidation
COPY --from=build-consolidation /out/consolidation /app/service
EXPOSE 9016 9097
CMD ["-f", "/app/etc/consolidation.yaml"]

