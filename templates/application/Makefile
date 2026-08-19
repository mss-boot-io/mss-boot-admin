.PHONY: backend-build backend-test frontend-install frontend-lint frontend-test frontend-build verify

backend-build:
	mkdir -p bin
	GOWORK=off go build -o bin/__MSS_APP_NAME__ ./cmd/server

backend-test:
	GOWORK=off go test ./...

frontend-install:
	corepack pnpm@10.34.5 --dir web install

frontend-lint:
	corepack pnpm@10.34.5 --dir web lint

frontend-test:
	corepack pnpm@10.34.5 --dir web test

frontend-build:
	corepack pnpm@10.34.5 --dir web build

verify: backend-test frontend-lint frontend-test frontend-build
