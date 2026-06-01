FROM node:22-alpine AS builder
WORKDIR /app
COPY plugins.json ./
COPY web/package*.json ./web/
# --ignore-scripts skips postinstall (needs scripts/ dir not yet copied)
RUN cd web && npm ci --ignore-scripts
COPY web/ ./web/
# prebuild hook (sync-public-assets.mjs) runs here with all files present
RUN cd web && npm run build

FROM nginx:alpine
COPY --from=builder /app/web/dist /usr/share/nginx/html
COPY web/nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
