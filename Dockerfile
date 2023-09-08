FROM alpine

LABEL authors="lwnmengjing"

COPY ./admin /app/admin

ENTRYPOINT ["/app/admin"]