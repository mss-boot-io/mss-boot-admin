PROJECT:=mss-boot-admin

.PHONY: build

build:
	CGO_ENABLED=0 go build -o admin main.go
test:
	go test -v ./... -cover
deps:
	go mod tidy