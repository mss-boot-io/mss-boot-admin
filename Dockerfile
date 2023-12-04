FROM alpine

LABEL authors="lwnmengjing"

WORKDIR /app

COPY ./admin /app/admin

ENTRYPOINT ["/app/admin", "server"]