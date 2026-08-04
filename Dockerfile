FROM golang:1.26.5-alpine AS builder

WORKDIR /workspace

COPY . .

RUN CGO_ENABLED=0 go build \
    -mod=vendor \
    -trimpath \
    -ldflags='-s -w' \
    -o /out/mss-boot-admin \
    ./admin

FROM alpine:latest

LABEL org.opencontainers.image.source="https://github.com/mss-boot-io/mss-boot-admin"

RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S admin \
    && adduser -S -G admin -h /app admin

WORKDIR /app

COPY --from=builder --chown=admin:admin /out/mss-boot-admin /app/mss-boot-admin
COPY --from=builder --chown=admin:admin /workspace/admin/config /app/config

USER admin

ENTRYPOINT ["/app/mss-boot-admin"]
CMD ["server"]
