# ── Build stage ────────────────────────────────────────────────
FROM golang:1.23-alpine AS builder

RUN apk add --no-cache gcc musl-dev

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY *.go ./
RUN CGO_ENABLED=1 GOOS=linux go build -ldflags="-s -w" -o frkn-backend .

# ── Runtime stage ───────────────────────────────────────────────
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata
RUN adduser -D -s /bin/sh frkn

WORKDIR /app
COPY --from=builder /app/frkn-backend .

# Static files and data dirs
RUN mkdir -p /app/static /app/data && chown -R frkn:frkn /app

USER frkn

ENV PORT=8080
ENV DB_PATH=/app/data/data.db
ENV STATIC_DIR=/app/static

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s \
  CMD wget -qO- http://localhost:8080/api/stats || exit 1

CMD ["./frkn-backend"]
