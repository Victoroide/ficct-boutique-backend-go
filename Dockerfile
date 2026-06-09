ARG GO_VERSION=1.23-alpine

FROM golang:${GO_VERSION} AS builder

WORKDIR /src

RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum* ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/migrate ./cmd/migrate
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/seed ./cmd/seed


FROM alpine:3.20 AS runtime

RUN apk add --no-cache ca-certificates curl tzdata \
    && addgroup -S ficct -g 1000 \
    && adduser -S ficct -G ficct -u 1000

WORKDIR /app

COPY --from=builder /out/server /usr/local/bin/server
COPY --from=builder /out/migrate /usr/local/bin/migrate
COPY --from=builder /out/seed /usr/local/bin/seed
COPY --chown=ficct:ficct migrations /app/migrations
COPY --chown=ficct:ficct docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh

USER ficct
EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 \
  CMD curl -fsS http://localhost:8080/health || exit 1

ENTRYPOINT ["sh", "/usr/local/bin/docker-entrypoint.sh"]
CMD ["server"]
