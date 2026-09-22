FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/server .

FROM alpine:3.20
RUN apk add --no-cache curl && adduser -D -u 10001 appuser
COPY --from=build /out/server /usr/local/bin/server
USER appuser
EXPOSE 8080
# Poll every 2s while starting (so `depends_on: service_healthy` does not wait),
# then only every 5 minutes: a health probe every few seconds is log noise.
HEALTHCHECK --interval=5m --timeout=3s --start-period=30s --start-interval=2s --retries=3 \
  CMD curl -fsS "http://localhost:${BACKEND_PORT:-8080}/healthz" || exit 1
ENTRYPOINT ["server"]
