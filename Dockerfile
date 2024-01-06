FROM golang:alpine AS builder

WORKDIR /go/src/github.com/mss-boot-io/mss-boot-admin

COPY . .

RUN make build

FROM alpine

LABEL authors="lwnmengjing"

WORKDIR /app

COPY --from=builder /go/src/github.com/mss-boot-io/mss-boot-admin/admin /app/admin

ENTRYPOINT ["/app/admin", "server"]