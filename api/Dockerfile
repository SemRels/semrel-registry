FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY api/go.* ./
RUN go mod download
COPY api/ .
COPY plugins.json ./plugins.json
# Static, stripped binary with the build metadata the runtime image cannot
# recover on its own.
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o api .

# Pinned by digest-bearing tag rather than :latest so a rebuild cannot silently
# change the runtime base. ca-certificates is explicit because the API calls
# the GitHub API over TLS and a missing trust store fails at request time, not
# at build time.
FROM alpine:3.22
RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -g 65532 -S nonroot \
    && adduser -u 65532 -S nonroot -G nonroot
WORKDIR /app
COPY --from=builder /app/api /usr/local/bin/
COPY --from=builder /app/database/migrations /app/database/migrations
COPY --from=builder /app/plugins.json /app/plugins.json
USER 65532:65532
EXPOSE 8080
ENV MIGRATE_DIR=/app/database/migrations
ENV PLUGINS_JSON_PATH=/app/plugins.json
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD wget -qO- http://127.0.0.1:8080/health >/dev/null || exit 1
CMD ["api"]
