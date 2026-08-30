PROJECT := mss-boot-admin
ADMIN_DIR := admin
FRAMEWORK_DIR := mss-boot
BIN_DIR ?= bin
COVERAGE_DIR ?= .coverage
COVERAGE_POLICY := .mss/coverage.json

.PHONY: build build-admin build-agent test test-agent test-admin test-admin-race \
	coverage-admin vet-admin tidy-admin-check tidy-admin-prepublication-check verify-admin verify-admin-preview compatibility-admin compatibility-standalone-mss compatibility-foundation-next \
	deps deps-agent deps-admin deps-admin-workspace deps-framework deps-all deps-release-preview \
	test-framework test-framework-race coverage-framework vet-framework \
	tidy-framework-check verify-framework test-all generate lint fix-lint clean \
	web-install web-lint web-test web-build \
	web-v6-install web-v6-lint web-v6-test web-v6-build web-v6-qualify \
	docs-install docs-build verify-all verify-release-evidence

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

tidy-admin-prepublication-check:
	bash tools/ci/verify-admin-module-metadata.sh

compatibility-admin:
	go test -count=1 ./$(ADMIN_DIR)/compatibility
	bash tools/compatibility/test-admin-external-consumer.sh

compatibility-standalone-mss:
	bash tools/compatibility/test-standalone-mss-consumer.sh

compatibility-foundation-next:
	bash tools/compatibility/test-standalone-mss-consumer.sh --upgrade --next-foundation

verify-admin: test-admin-race coverage-admin vet-admin tidy-admin-check compatibility-admin build-admin

verify-admin-preview: test-admin-race coverage-admin vet-admin tidy-admin-prepublication-check compatibility-admin build-admin

deps:
	$(MAKE) deps-all

deps-agent:
	GOWORK=off go mod download

deps-admin:
	cd $(ADMIN_DIR) && GOWORK=off go mod download

deps-admin-workspace:
	@temporary_gowork="$$(mktemp "$(CURDIR)/.release-preview-go.XXXXXX.work")"; \
	trap 'rm -f -- "$$temporary_gowork" "$$temporary_gowork.sum"' EXIT HUP INT TERM; \
	cp go.work "$$temporary_gowork"; \
	cd $(ADMIN_DIR) && GOWORK="$$temporary_gowork" go mod download

deps-framework:
	cd $(FRAMEWORK_DIR) && GOWORK=off go mod download

deps-all: deps-agent deps-admin deps-framework

deps-release-preview: deps-agent deps-admin-workspace deps-framework

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
	cd web/antd-v6 && corepack pnpm@10.34.5 deps:check
	cd web/antd-v6 && corepack pnpm@10.34.5 dedupe --check --config.strict-peer-dependencies=false --config.ignore-scripts=true
	cd web/antd-v6 && corepack pnpm@10.34.5 audit:release
	$(MAKE) web-v6-lint
	$(MAKE) web-v6-test
	$(MAKE) web-v6-build
	cd web/antd-v6 && corepack pnpm@10.34.5 delivery:smoke
	bash tools/verification/run-frontend-e2e.sh

docs-install:
	cd docs && corepack pnpm@9.15.9 install --frozen-lockfile

docs-build:
	cd docs && corepack pnpm@9.15.9 build

verify-all:
	GOWORK=off go run ./cmd/mss verify --all

verify-release-evidence:
	@test -n "$(COMMIT)" || { echo 'COMMIT=<full-sha> is required' >&2; exit 2; }
	GOWORK=off go run ./cmd/mss verify --all --release-evidence --expect-commit "$(COMMIT)"

clean:
	rm -rf $(BIN_DIR) $(COVERAGE_DIR)
