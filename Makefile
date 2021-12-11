.PHONY: all
all: dev

dev: fmt test

fmt:
	go fmt ./...

build:
	go build

test:
	go test ./...

clean:
	go clean -i -r