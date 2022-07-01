.PHONY: all
all: dev

dev: fmt test build

fmt:
	go fmt ./...

VERSION = $(shell git describe --tags --abbrev=0)
HEAD_COMMIT_ID = $(shell git rev-parse HEAD)
LDFLAGS += -X "github.com/LENSHOOD/go-raft/cmd.Version=[$(VERSION) - $(HEAD_COMMIT_ID)]"

build:
	env GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags '$(LDFLAGS)'

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

### CHAOS TESTING
PROM_INFO = -X "github.com/LENSHOOD/go-raft/state_machine.url=$(PROM_PUSH_GATEWAY_URL)"
build-chaos:
ifeq ($(PROM_PUSH_GATEWAY_URL),)
	$(error Please set env PROM_PUSH_GATEWAY_URL as your prometheus push gateway ip:port)
else
	env GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -tags=chaos -trimpath -ldflags '$(LDFLAGS) $(PROM_INFO)'
endif

test-chaos:
	go test ./... -tags=chaos -ldflags '$(PROM_INFO)'

build-image-chaos:
	docker build . -t $(IMAGE_NAME_VERSION)-test-chaos