PROJECT:=mss-boot-admin

.PHONY: build

build:
	CGO_ENABLED=0 go build -o admin main.go
test:
	go test -cover -coverprofile=coverage.out -v ./...
deps:
	go mod download

.PHONY: lint
lint:
	golangci-lint run -v ./...
fix-lint:
	goimports -w .