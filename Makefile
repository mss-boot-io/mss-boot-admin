PROJECT := mss-boot-admin
ADMIN_DIR := admin
FRAMEWORK_DIR := mss-boot
BIN_DIR ?= bin
COVERAGE_DIR ?= .coverage
COVERAGE_POLICY := .mss/coverage.json

.PHONY: build build-admin build-agent test test-agent test-admin test-admin-race \
	coverage-admin vet-admin tidy-admin-check verify-admin compatibility-admin \
	deps deps-agent deps-admin deps-framework deps-all \
	test-framework test-framework-race coverage-framework vet-framework \
	tidy-framework-check verify-framework test-all generate lint fix-lint clean \
	web-install web-lint web-test web-build \
	web-v6-install web-v6-lint web-v6-test web-v6-build web-v6-qualify \
	docs-install docs-build verify-all

build: build-admin

build-admin:
	mkdir -p $(BIN_DIR)
	cd $(ADMIN_DIR) && CGO_ENABLED=0 go build -trimpath -o ../$(BIN_DIR)/$(PROJECT) .

build-agent:
	mkdir -p $(BIN_DIR)
	GOWORK=off go build -trimpath -o $(BIN_DIR)/mss ./cmd/mss
	GOWORK=off go build -trimpath -o $(BIN_DIR)/mss-mcp ./cmd/mss-mcp

test: test-agent test-admin

test-agent:
	GOWORK=off go test -shuffle=on -count=1 ./...

test-admin:
	cd $(ADMIN_DIR) && go test -shuffle=on -count=1 ./...

test-admin-race:
	cd $(ADMIN_DIR) && go test -race -shuffle=on -count=1 ./...

coverage-admin:
	mkdir -p $(COVERAGE_DIR)
	cd $(ADMIN_DIR) && go test -shuffle=on -count=1 -covermode=atomic -coverpkg=./... -coverprofile=../$(COVERAGE_DIR)/admin.out ./...
	python3 scripts/check-go-coverage.py --profile $(COVERAGE_DIR)/admin.out --policy $(COVERAGE_POLICY) --component admin --summary

vet-admin:
	cd $(ADMIN_DIR) && go vet ./...

tidy-admin-check:
	cd $(ADMIN_DIR) && go mod tidy
	git diff --exit-code -- $(ADMIN_DIR)/go.mod $(ADMIN_DIR)/go.sum

compatibility-admin:
	go test -count=1 ./$(ADMIN_DIR)/compatibility
	bash tools/compatibility/test-admin-external-consumer.sh

verify-admin: test-admin-race coverage-admin vet-admin tidy-admin-check compatibility-admin build-admin

deps:
	$(MAKE) deps-all

deps-agent:
	GOWORK=off go mod download

deps-admin:
	cd $(ADMIN_DIR) && go mod download

deps-framework:
	cd $(FRAMEWORK_DIR) && GOWORK=off go mod download

deps-all: deps-agent deps-admin deps-framework

test-framework:
	cd $(FRAMEWORK_DIR) && GOWORK=off go test -shuffle=on -count=1 ./...

test-framework-race:
	cd $(FRAMEWORK_DIR) && GOWORK=off go test -race -shuffle=on -count=1 ./...

coverage-framework:
	mkdir -p $(COVERAGE_DIR)
	cd $(FRAMEWORK_DIR) && GOWORK=off go test -shuffle=on -count=1 -covermode=atomic -coverpkg=./... -coverprofile=../$(COVERAGE_DIR)/mss-boot.out ./...
	python3 scripts/check-go-coverage.py --profile $(COVERAGE_DIR)/mss-boot.out --policy $(COVERAGE_POLICY) --component mss-boot --summary

vet-framework:
	cd $(FRAMEWORK_DIR) && GOWORK=off go vet ./...

tidy-framework-check:
	cd $(FRAMEWORK_DIR) && GOWORK=off go mod tidy
	git diff --exit-code -- $(FRAMEWORK_DIR)/go.mod $(FRAMEWORK_DIR)/go.sum

verify-framework: test-framework-race coverage-framework vet-framework tidy-framework-check

test-all: test-agent test-admin test-framework

generate:
	GOWORK=off go generate ./...
	cd $(ADMIN_DIR) && go generate ./...

lint:
	GOWORK=off golangci-lint run -v ./...
	cd $(ADMIN_DIR) && golangci-lint run -v ./...
	cd $(FRAMEWORK_DIR) && GOWORK=off golangci-lint run -v ./...

fix-lint:
	goimports -w cmd internal $(ADMIN_DIR) $(FRAMEWORK_DIR)

web-install: web-v6-install

web-lint: web-v6-lint

web-test: web-v6-test

web-build: web-v6-build

web-v6-install:
	cd web/antd-v6 && corepack pnpm@10.34.5 install --frozen-lockfile

web-v6-lint:
	cd web/antd-v6 && corepack pnpm@10.34.5 lint

web-v6-test:
	cd web/antd-v6 && corepack pnpm@10.34.5 test:ci

web-v6-build:
	cd web/antd-v6 && corepack pnpm@10.34.5 build:release

web-v6-qualify:
	$(MAKE) web-v6-lint
	$(MAKE) web-v6-test
	$(MAKE) web-v6-build
	cd web/antd-v6 && corepack pnpm@10.34.5 delivery:smoke
	cd web/antd-v6 && corepack pnpm@10.34.5 test:e2e

docs-install:
	cd docs && corepack pnpm@9.15.9 install --frozen-lockfile

docs-build:
	cd docs && corepack pnpm@9.15.9 build

verify-all: verify-admin verify-framework test-agent web-v6-lint web-v6-test web-v6-build docs-build

clean:
	rm -rf $(BIN_DIR) $(COVERAGE_DIR)
