FROM node:22-alpine AS builder
WORKDIR /app
# Copy plugins.json for the sync-public-assets script
COPY plugins.json ./
COPY web/package*.json ./web/
RUN cd web && npm ci
COPY web/ ./web/
RUN cd web && npm run build

FROM nginx:alpine
COPY --from=builder /app/web/dist /usr/share/nginx/html
COPY web/nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
