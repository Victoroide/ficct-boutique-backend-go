# Tiny multi-stage build for the fake Expo Push API. Only the cmd/fake-expo-push
# binary is needed at runtime; the rest of the Go module is brought in just to
# satisfy go.mod.
ARG GO_VERSION=1.23-alpine

FROM golang:${GO_VERSION} AS builder
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/fake-expo-push ./cmd/fake-expo-push

FROM alpine:3.20 AS runtime
RUN apk add --no-cache ca-certificates curl
COPY --from=builder /out/fake-expo-push /usr/local/bin/fake-expo-push
EXPOSE 8080
HEALTHCHECK --interval=15s --timeout=3s --start-period=5s --retries=5 \
  CMD curl -fsS http://localhost:8080/healthz || exit 1
ENV PORT=8080
CMD ["fake-expo-push"]
