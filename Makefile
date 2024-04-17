PROJECT:=mss-boot-admin

.PHONY: build

build:
	CGO_ENABLED=0 go build -o admin main.go
test:
	go test -coverprofile=coverage.out ./...
deps:
	go mod download

.PHONY: lint
lint:
	golangci-lint run -v ./...
fix-lint:
	goimports -w .