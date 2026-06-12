FROM golang:1.25.4 AS builder

WORKDIR /src

# Avoid inheriting host proxy settings (buildkit auto-injects HTTP(S)_PROXY build args).
ARG HTTP_PROXY
ARG HTTPS_PROXY
ARG NO_PROXY
ARG http_proxy
ARG https_proxy
ARG no_proxy
ENV HTTP_PROXY=
ENV HTTPS_PROXY=
ENV NO_PROXY=
ENV http_proxy=
ENV https_proxy=
ENV no_proxy=
ENV GOPROXY=https://proxy.golang.org,direct

COPY go.mod go.sum ./
# go.mod uses local replace for gotron-sdk; it must exist before `go mod download`.
COPY lib/gotron-sdk-0.24.1 ./lib/gotron-sdk-0.24.1
RUN go mod download

COPY . .

ENV CGO_ENABLED=0
ENV GOOS=linux
ENV GOARCH=amd64

RUN go build -trimpath -ldflags="-s -w" -o /out/bootstrapper ./cmd/bootstrapper

# Distroless contains CA certs for in-cluster API TLS.
FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /

COPY --from=builder /out/bootstrapper /bootstrapper

USER nonroot:nonroot

ENTRYPOINT ["/bootstrapper"]
