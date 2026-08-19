FROM golang:1.26 AS backend
WORKDIR /src
COPY . .
RUN GOWORK=off go build -trimpath -o /out/admin ./cmd/server

FROM node:24-bookworm-slim AS frontend
WORKDIR /src/web
RUN corepack enable
COPY web/package.json ./
RUN corepack pnpm@10.34.5 install --no-frozen-lockfile
COPY web/ ./
RUN corepack pnpm@10.34.5 build

FROM debian:bookworm-slim
RUN useradd --create-home --uid 10001 admin
WORKDIR /app
COPY --from=backend /out/admin /app/admin
COPY --from=frontend /src/web/dist /app/public
USER admin
ENTRYPOINT ["/app/admin"]
CMD ["server"]
