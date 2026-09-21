FROM node:22-alpine AS builder
WORKDIR /app
COPY admin/package*.json ./
RUN npm ci
COPY admin/ ./
RUN npm run build

# The unprivileged image runs nginx as uid 101 with no root master process and
# listens on 8080, so the container needs neither root nor a capability to bind
# its port.
FROM nginxinc/nginx-unprivileged:1.29-alpine
USER root
COPY --from=builder /app/dist /usr/share/nginx/html
COPY admin/nginx.conf /etc/nginx/templates/default.conf.template
COPY admin/security-headers.conf /etc/nginx/snippets/security-headers.conf
USER 101
EXPOSE 8080
ENV API_URL=http://api:8080 \
    NGINX_ENTRYPOINT_LOCAL_RESOLVERS=1 \
    NGINX_ENVSUBST_FILTER="^(API_URL|NGINX_LOCAL_RESOLVERS)$"
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget -qO- http://127.0.0.1:8080/ >/dev/null || exit 1
