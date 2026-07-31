W:=${W}
V:=${V}
NAME=micro
IMAGE_NAME=micro/$(NAME)
GIT_COMMIT=$(shell git rev-parse --short HEAD)
GIT_TAG=$(shell git describe --abbrev=0 --tags --always --match "v*")
GIT_IMPORT=github.com/mss-boot-io/mss-boot
VERSION_IMPORT=$(GIT_IMPORT)/pkg/version
CGO_ENABLED=0
BUILD_DATE=$(shell date -u +'%Y-%m-%dT%H:%M:%SZ')
LDFLAGS=-X $(VERSION_IMPORT).buildDate=$(BUILD_DATE) -X $(VERSION_IMPORT).gitCommit=$(GIT_COMMIT) -X $(VERSION_IMPORT).gitVersion=$(GIT_TAG)
IMAGE_TAG=$(GIT_TAG)-$(GIT_COMMIT)
PROTO_FLAGS=--go_opt=paths=source_relative
PROTO_PATH=$(GOPATH)/src:.
SRC_DIR=$(GOPATH)/src

.PHONY: proto
proto:
	#protoc-go-inject-tag -I ./proto -I ${GOPATH}/src  --go_out=plugins=grpc: proto/${W}/${V}/*;
	find proto/ -name '*.proto' -exec protoc --proto_path=$(PROTO_PATH) $(PROTO_FLAGS) --go_out=plugins=grpc:. {} \;


.PHONY: lint
lint:
	golangci-lint run -v ./...

.PHONY: fix-lint
fix-lint:
	goimports -w .

.PHONY: test
test:
	go test ./...

.PHONY: coverage
coverage:
	go test ./... -coverprofile=coverage.out
	go tool cover -func=coverage.out

.PHONY: tidy
tidy:
	go mod tidy
