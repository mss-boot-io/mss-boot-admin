# syntax=docker/dockerfile:1.7

FROM --platform=$BUILDPLATFORM golang:1.26.6-alpine@sha256:3889b425f035be855a72fb4755265311293b6d414521f0a519d819df32222d83 AS backend
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src
COPY go.mod go.sum ./
RUN GOWORK=off go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH GOWORK=off \
    go build -trimpath -ldflags="-s -w" -o /out/admin ./cmd/server

FROM --platform=$BUILDPLATFORM node:24.19.0-bookworm-slim@sha256:3638d9a6fe4030bd716be989438248074489337ba3275657f93595428be4fc03 AS frontend
WORKDIR /src/web
RUN corepack enable
COPY web/package.json web/pnpm-lock.yaml web/.npmrc ./
RUN corepack pnpm@10.34.5 install --frozen-lockfile
COPY web/ ./
RUN corepack pnpm@10.34.5 build

FROM alpine:3.24.1@sha256:28bd5fe8b56d1bd048e5babf5b10710ebe0bae67db86916198a6eec434943f8b
RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S admin \
    && adduser -S -G admin -h /app admin
WORKDIR /app
COPY --from=backend --chown=admin:admin /out/admin /app/admin
COPY --from=frontend --chown=admin:admin /src/web/dist /app/public
USER admin
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
  CMD wget --quiet --spider http://127.0.0.1:8080/healthz || exit 1
ENTRYPOINT ["/app/admin"]
CMD ["server"]
