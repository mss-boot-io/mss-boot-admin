FROM golang:1.26.5-alpine@sha256:0178a641fbb4858c5f1b48e34bdaabe0350a330a1b1149aabd498d0699ff5fb2 AS builder

ARG VERSION=devel
ARG COMMIT=unknown

WORKDIR /workspace

COPY . .

RUN CGO_ENABLED=0 go build \
    -mod=vendor \
    -trimpath \
    -ldflags="-s -w -X github.com/mss-boot-io/mss-boot-admin/admin/pkg.Version=${VERSION} -X github.com/mss-boot-io/mss-boot-admin/admin/pkg.Commit=${COMMIT}" \
    -o /out/mss-boot-admin \
    ./admin

FROM alpine:3.24.1@sha256:28bd5fe8b56d1bd048e5babf5b10710ebe0bae67db86916198a6eec434943f8b

ARG VERSION=devel
ARG COMMIT=unknown

LABEL org.opencontainers.image.source="https://github.com/mss-boot-io/mss-boot-admin" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${COMMIT}"

RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S admin \
    && adduser -S -G admin -h /app admin

WORKDIR /app

COPY --from=builder --chown=admin:admin /out/mss-boot-admin /app/mss-boot-admin
COPY --from=builder --chown=admin:admin /workspace/admin/config /app/config

USER admin

ENTRYPOINT ["/app/mss-boot-admin"]
CMD ["server"]
