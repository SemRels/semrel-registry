FROM node:22-alpine AS builder
WORKDIR /app
COPY admin/package*.json ./
RUN npm ci
COPY admin/ ./
RUN npm run build

FROM nginx:alpine
COPY --from=builder /app/dist /usr/share/nginx/html
# nginx.conf uses ${API_URL} — substituted at container start via envsubst.
COPY admin/nginx.conf /etc/nginx/templates/default.conf.template
EXPOSE 80
ENV API_URL=http://api:8080
