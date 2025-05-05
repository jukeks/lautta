.PHONY: proto test test-cover build
default: format proto build test

proto:
	cd proto && \
		buf lint . && \
		rm -fr ./gen && \
		buf generate .

test:
	go test ./...

build:
	go build -v -o ./bin/server ./cmd/server

format:
	go fmt ./...

test-cover:
	go test -coverprofile=coverage.out ./... -coverpkg=./...

cover: test-cover
	go tool cover -html=coverage.out
