.PHONY: all
all: dev

dev: fmt test

fmt:
	go fmt ./...

build:
	env GOOS=linux GOARCH=amd64 go build -trimpath

build-image: clean build
	docker build . -t lenshood/go-raft:v0.1.0

test:
	go test ./...

clean:
	go clean -i -r -cache -testcache