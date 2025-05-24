.PHONY: proto test test-cover build
default: build test

proto:
	cd proto && \
		buf lint . && \
		rm -fr ./gen && \
		buf generate .

test:
	go test ./...

build:
	go build -v -o ./bin/node ./cmd/node

run1:
	./bin/node -config "1=localhost:40050,2=localhost:40051,3=localhost:40052"

run2:
	./bin/node -config "2=localhost:40051,1=localhost:40050,3=localhost:40052"

run3:
	./bin/node -config "3=localhost:40052,1=localhost:40050,2=localhost:40051"

format:
	go fmt ./...

test-cover:
	go test -coverprofile=coverage.out ./... -coverpkg=./...

cover: test-cover
	go tool cover -html=coverage.out
