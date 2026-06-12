FROM alpine:3.20

RUN apk add --no-cache \
    bash \
    ca-certificates \
    coreutils \
    mariadb-client \
    python3 \
    tzdata

WORKDIR /app

COPY infra/docker/db-migrator/entrypoint.sh /app/entrypoint.sh
COPY deploy/docker/init-db /app/migrations

RUN chmod +x /app/entrypoint.sh \
    && addgroup -g 1000 app \
    && adduser -D -u 1000 -G app app \
    && chown -R app:app /app

USER app

ENV MIGRATIONS_DIR=/app/migrations

ENTRYPOINT ["/app/entrypoint.sh"]
