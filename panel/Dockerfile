# Build context: repository root (see docker-compose.yml)
FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY pkg ./pkg
COPY panel ./panel
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/service ./panel/cmd

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata wget && adduser -D -u 10001 app
USER app
COPY --from=build /out/service /usr/local/bin/service
EXPOSE 8080
HEALTHCHECK --interval=10s --timeout=3s --start-period=10s --retries=5   CMD wget -qO- http://127.0.0.1:8080/health || exit 1
ENTRYPOINT ["/usr/local/bin/service"]
