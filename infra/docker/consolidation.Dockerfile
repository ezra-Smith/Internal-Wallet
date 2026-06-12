# Consolidation RPC Dockerfile
# Background consolidation scheduler/executor service.
#
# Build:
#   docker build -f infra/docker/consolidation.Dockerfile -t internal-wallet/consolidation .
#
# ============================================================
# Build Stage
# ============================================================
FROM golang:1.25.4-alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /workspace

COPY go.mod go.sum ./
COPY lib/gotron-sdk-0.24.1/go.mod lib/gotron-sdk-0.24.1/go.sum ./lib/gotron-sdk-0.24.1/
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" \
    -o /app/consolidation ./services/consolidation/rpc/consolidation.go

# ============================================================
# Runtime Stage
# ============================================================
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
ENV TZ=Asia/Tokyo

COPY --from=builder /app/consolidation /app/service

VOLUME ["/app/etc"]

# gRPC + metrics
EXPOSE 9016 9097

USER nonroot:nonroot

ENTRYPOINT ["/app/service"]
CMD ["-f", "/app/etc/consolidation.yaml"]

