.PHONY: all
all: dev

dev: fmt test

fmt:
	go fmt ./...

VERSION = $(shell git describe --tags --abbrev=0)
HEAD_COMMIT_ID = $(shell git rev-parse HEAD)
LDFLAGS += -X "github.com/LENSHOOD/go-raft/cmd.Version=[$(VERSION) - $(HEAD_COMMIT_ID)]"

build:
	env GOOS=linux GOARCH=amd64 go build -trimpath -ldflags '$(LDFLAGS)'

build-image: clean build
	docker build . -t lenshood/go-raft:$(VERSION)

test:
	go test ./...

clean:
	go clean -i -r -cache -testcache