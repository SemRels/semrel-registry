FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o api main.go

FROM alpine:latest
COPY --from=builder /app/api /usr/local/bin/
COPY --from=builder /app/database/migrations /app/database/migrations
EXPOSE 8080
ENV MIGRATE_DIR=/app/database/migrations
CMD ["api"]
