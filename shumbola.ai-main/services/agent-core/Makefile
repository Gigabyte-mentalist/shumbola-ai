.PHONY: build test vet fmt run

build:
	go build ./...

test:
	go test ./... -v

vet:
	go vet ./...

fmt:
	gofmt -l .

run:
	go run ./cmd/server
