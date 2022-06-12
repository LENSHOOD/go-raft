.PHONY: all
all: dev

dev: fmt test build

fmt:
	go fmt ./...

VERSION = $(shell git describe --tags --abbrev=0)
HEAD_COMMIT_ID = $(shell git rev-parse HEAD)
LDFLAGS += -X "github.com/LENSHOOD/go-raft/cmd.Version=[$(VERSION) - $(HEAD_COMMIT_ID)]"

build:
	env GOOS=linux GOARCH=amd64 go build -trimpath -ldflags '$(LDFLAGS)'

IMAGE_NAME_VERSION = lenshood/go-raft:$(VERSION)
build-image:
	docker build . -t $(IMAGE_NAME_VERSION)

echo-repo:
	echo $(REPO)

REPO_IMAGE_NAME_VERSION = $(REPO)/$(IMAGE_NAME_VERSION)
push-image:
ifeq ($(REPO),)
	$(error Please set env REPO as your pushed repo name)
else
	docker tag $(IMAGE_NAME_VERSION) $(REPO_IMAGE_NAME_VERSION)
	docker push $(REPO_IMAGE_NAME_VERSION)
endif

test:
	go test ./...

clean:
	go clean -i -r -cache -testcache