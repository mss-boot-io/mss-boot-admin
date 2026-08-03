PROJECT:=mss-boot-admin

.PHONY: build test deps generate lint fix-lint \
	deps-framework deps-all test-framework test-all \
	web-install web-lint web-test web-build docs-install docs-build verify-all

build:
	CGO_ENABLED=0 go build -o admin main.go

test:
	go test -coverprofile=coverage.out ./...

deps:
	go list -deps ./... >/dev/null

deps-framework:
	cd mss-boot && GOWORK=off go mod download

deps-all: deps deps-framework

test-framework:
	cd mss-boot && go test ./...

test-all: test-framework test

generate:
	go generate ./...

lint:
	golangci-lint run -v ./...

fix-lint:
	goimports -w .

web-install:
	cd web/antd && corepack enable && pnpm install --frozen-lockfile

web-lint:
	cd web/antd && pnpm lint:js && pnpm tsc

web-test:
	cd web/antd && pnpm test -- --runInBand

web-build:
	cd web/antd && pnpm build:local

docs-install:
	cd docs && corepack enable && pnpm install --frozen-lockfile

docs-build:
	cd docs && pnpm build

verify-all: test-all web-lint web-test web-build docs-build
