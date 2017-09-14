GO	:= go
GOENV	:= GOOS=linux GOARCH=amd64
EXE	:= px-runcds

#DOCKER_IMG := portworx/px-runcds:latest
ifndef DOCKER_IMG
	DOCKER_IMG := $(DOCKER_HUB_REPO)/$(DOCKER_HUB_IMAGE)-runcds:$(DOCKER_HUB_TAG)
endif


.PHONY: all build clean distclean static

all:	px-runcds

build:	Dockerfile static
	docker build -t $(DOCKER_IMG) .

# DEFAULT build
$(EXE):	px-runcds.go utils/installer.go utils/inspect.go vendor/github.com/docker/docker/api/common.go
	$(GOENV) $(GO) build -v -i -o $(EXE)

# STATIC build
static:	px-runcds.go utils/installer.go utils/inspect.go vendor/github.com/docker/docker/api/common.go
	CGO_ENABLED=0 $(GOENV) $(GO) build -v -a --ldflags '-extldflags "-static"' -o $(EXE)

vendor/github.com/docker/docker/api/common.go:
	$(GO) get -v github.com/kardianos/govendor
	$(GOENV) govendor sync

clean:
	@rm -fr $(EXE)

distclean:	clean
	@rm -fr vendor/github.com vendor/golang.org

