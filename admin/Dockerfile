FROM node:22-alpine AS builder
WORKDIR /app
COPY admin/package*.json ./
RUN npm ci
COPY admin/ ./
RUN npm run build

FROM nginx:alpine
COPY --from=builder /app/dist /usr/share/nginx/html
COPY admin/nginx.conf /etc/nginx/templates/default.conf.template
EXPOSE 80
ENV API_URL=http://api:8080 \
    NGINX_ENTRYPOINT_LOCAL_RESOLVERS=1 \
    NGINX_ENVSUBST_FILTER="^(API_URL|NGINX_LOCAL_RESOLVERS)$"
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget -qO- http://127.0.0.1/ >/dev/null || exit 1
