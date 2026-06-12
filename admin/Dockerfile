FROM oven/bun:1.3.0-alpine AS builder

WORKDIR /app

COPY web/admin/package.json web/admin/bun.lock web/admin/.npmrc web/admin/orval.config.cjs ./
# Docker build may inherit host proxy envs (often socks5://) which Bun doesn't support.
# Force Bun to avoid unsupported proxy envs.
RUN unset http_proxy https_proxy all_proxy HTTP_PROXY HTTPS_PROXY ALL_PROXY NO_PROXY no_proxy \
	&& bun install --frozen-lockfile --ignore-scripts

COPY web/admin/ ./
COPY docs/swagger/admin.swagger.json ./admin.swagger.json

RUN test -s ./admin.swagger.json
ENV ORVAL_ADMIN_SWAGGER_TARGET=./admin.swagger.json

RUN bun run postinstall
RUN bun run build

FROM nginx:1.27-alpine

RUN rm -rf /usr/share/nginx/html/*

COPY --from=builder /app/dist/ /usr/share/nginx/html/
COPY web/admin/nginx.conf /etc/nginx/conf.d/default.conf

# Create nginx cache/run dirs with proper permissions for non-root runtime.
RUN mkdir -p /var/cache/nginx/client_temp \
             /var/cache/nginx/proxy_temp \
             /var/cache/nginx/fastcgi_temp \
             /var/cache/nginx/uwsgi_temp \
             /var/cache/nginx/scgi_temp && \
    chown -R nginx:nginx /var/cache/nginx && \
    chown -R nginx:nginx /usr/share/nginx/html && \
    touch /var/run/nginx.pid && \
    chown nginx:nginx /var/run/nginx.pid

USER nginx

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/ || exit 1

CMD ["nginx", "-g", "daemon off;"]
