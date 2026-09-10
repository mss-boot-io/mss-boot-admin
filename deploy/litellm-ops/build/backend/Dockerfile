FROM alpine:3.24.1@sha256:28bd5fe8b56d1bd048e5babf5b10710ebe0bae67db86916198a6eec434943f8b
RUN addgroup -S -g 10001 admin \
    && adduser -S -D -H -u 10001 -G admin admin \
    && mkdir -p /app/config /app/logs /data \
    && chown -R 10001:10001 /app /data
WORKDIR /app
COPY --chown=10001:10001 mss-boot-admin /app/mss-boot-admin
USER 10001:10001
ENTRYPOINT ["/app/mss-boot-admin"]
CMD ["server"]
