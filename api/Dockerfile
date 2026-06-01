FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY api/go.* ./
RUN go mod download
COPY api/ .
COPY plugins.json ./plugins.json
RUN CGO_ENABLED=0 go build -o api main.go

FROM alpine:latest
COPY --from=builder /app/api /usr/local/bin/
COPY --from=builder /app/database/migrations /app/database/migrations
COPY --from=builder /app/plugins.json /app/plugins.json
EXPOSE 8080
ENV MIGRATE_DIR=/app/database/migrations
CMD ["api"]
