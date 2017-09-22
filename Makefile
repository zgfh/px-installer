# Makefile for PX-INSTALLER project
#

# VARIABLES
#
ifndef DOCKER_HUB_REPO
    DOCKER_HUB_REPO := portworx
    $(warning DOCKER_HUB_REPO not defined, using '$(DOCKER_HUB_REPO)' instead)
endif
ifndef DOCKER_HUB_MONITOR_IMAGE
    DOCKER_HUB_MONITOR_IMAGE := monitor
    $(warning DOCKER_HUB_MONITOR_IMAGE not defined, using '$(DOCKER_HUB_MONITOR_IMAGE)' instead)
endif
ifndef DOCKER_HUB_WEBSVC_IMAGE
    DOCKER_HUB_WEBSVC_IMAGE := monitor-websvc
    $(warning DOCKER_HUB_WEBSVC_IMAGE not defined, using '$(DOCKER_HUB_WEBSVC_IMAGE)' instead)
endif
ifndef DOCKER_HUB_OCIMON_IMAGE
    DOCKER_HUB_OCIMON_IMAGE := oci-monitor
    $(warning DOCKER_HUB_OCIMON_IMAGE not defined, using '$(DOCKER_HUB_OCIMON_IMAGE)' instead)
endif
ifndef DOCKER_HUB_TAG
    #DOCKER_HUB_TAG := $(shell git rev-parse HEAD | cut -c-7)
    DOCKER_HUB_TAG := latest
    $(warning DOCKER_HUB_TAG not defined, using '$(DOCKER_HUB_TAG)' instead)
endif

GO		:= go
GOENV		:= GOOS=linux GOARCH=amd64
SUDO		:= sudo
MONITOR_IMG	:= $(DOCKER_HUB_REPO)/$(DOCKER_HUB_MONITOR_IMAGE):$(DOCKER_HUB_TAG)
WEBSVC_IMG	:= $(DOCKER_HUB_REPO)/$(DOCKER_HUB_WEBSVC_IMAGE):$(DOCKER_HUB_TAG)
OCIMON_IMG	:= $(DOCKER_HUB_REPO)/$(DOCKER_HUB_OCIMON_IMAGE):$(DOCKER_HUB_TAG)

BUILD_TYPE=static
ifeq ($(BUILD_TYPE),static)
    BUILD_OPTIONS += -v -a --ldflags "-extldflags -static"
    GOENV += CGO_ENABLED=0
else ifeq ($(BUILD_TYPE),debug)
    BUILD_OPTIONS += -i -v -gcflags "-N -l"
else
    BUILD_OPTIONS += -i -v
endif

ifeq ($(shell id -u),0)
    SUDO :=
endif

TARGETS += px-mon/px-mon px-spec-websvc/px-spec-websvc px-oci-mon/px-oci-mon


# BUILD RULES
#

.PHONY: all deploy clean distclean vendor-pull px-container

all: $(TARGETS)


px-mon/px-mon: px-mon/px-mon.go vendor/github.com/fsouza/go-dockerclient
	@echo "Building $@ binary..."
	@cd px-mon && env $(GOENV) $(GO) build $(BUILD_OPTIONS)

px-oci-mon/px-oci-mon: px-oci-mon/main.go vendor/github.com/docker/docker/api
	@echo "Building $@ binary..."
	@cd px-oci-mon && env $(GOENV) $(GO) build $(BUILD_OPTIONS)

px-spec-websvc/px-spec-websvc: px-spec-websvc/px-spec-websvc.go vendor/github.com/gorilla/schema
	@echo "Building $@ binary..."
	@cd px-spec-websvc && env $(GOENV) $(GO) build $(BUILD_OPTIONS)


px-mon-container:
	@echo "Building $@ ..."
	@cd px-mon && $(SUDO) docker build -t $(MONITOR_IMG) .

px-oci-mon-container:
	@echo "Building $@ ..."
	@cd px-oci-mon && $(SUDO) docker build -t $(OCIMON_IMG) .

px-spec-websvc-container:
	@echo "Building $@ ..."
	@cd px-spec-websvc && $(SUDO) docker build -t $(WEBSVC_IMG) .

px-container: px-mon-container px-oci-mon-container px-spec-websvc-container


$(GOPATH)/bin/govendor:
	$(GO) get -v github.com/kardianos/govendor

vendor-pull: $(GOPATH)/bin/govendor
	$(GOENV) $(GOPATH)/bin/govendor sync

vendor/github.com/fsouza/go-dockerclient: vendor-pull
vendor/github.com/docker/docker/api: vendor-pull
vendor/github.com/gorilla/schema: vendor-pull

deploy:
	@echo "Deploying all containers..."
ifneq ($(DOCKER_HUB_PASSWD),)
	$(warning Found DOCKER_HUB_PASSWD env - using authenticated docker push)
	$(SUDO) docker login --username=$(DOCKER_HUB_USER) --password=$(DOCKER_HUB_PASSWD)
endif
	$(SUDO) docker push $(MONITOR_IMG)
	$(SUDO) docker push $(OCIMON_IMG)
	$(SUDO) docker push $(WEBSVC_IMG)
	-$(SUDO) docker logout

deploy_latest:
	@echo "Re-Deploying current containers as TAG:latest..."
ifneq ($(DOCKER_HUB_TAG),latest)
ifneq ($(DOCKER_HUB_PASSWD),)
	$(warning Found DOCKER_HUB_PASSWD env - using authenticated docker push)
	$(SUDO) docker login --username=$(DOCKER_HUB_USER) --password=$(DOCKER_HUB_PASSWD)
endif
	$(SUDO) docker tag $(MONITOR_IMG) $(DOCKER_HUB_REPO)/$(DOCKER_HUB_MONITOR_IMAGE):latest
	$(SUDO) docker push $(DOCKER_HUB_REPO)/$(DOCKER_HUB_MONITOR_IMAGE):latest
	$(SUDO) docker tag $(OCIMON_IMG) $(DOCKER_HUB_REPO)/$(DOCKER_HUB_OCIMON_IMAGE):latest
	$(SUDO) docker push $(DOCKER_HUB_REPO)/$(DOCKER_HUB_OCIMON_IMAGE):latest
	$(SUDO) docker tag $(WEBSVC_IMG) $(DOCKER_HUB_REPO)/$(DOCKER_HUB_WEBSVC_IMAGE):latest
	$(SUDO) docker push $(DOCKER_HUB_REPO)/$(DOCKER_HUB_WEBSVC_IMAGE):latest
	-$(SUDO) docker logout
endif

clean:
	rm -f $(TARGETS)
	-$(SUDO) docker rmi -f $(MONITOR_IMG) $(WEBSVC_IMG) $(OCIMON_IMG) \
	    $(DOCKER_HUB_REPO)/$(DOCKER_HUB_MONITOR_IMAGE):latest \
	    $(DOCKER_HUB_REPO)/$(DOCKER_HUB_OCIMON_IMAGE):latest \
	    $(DOCKER_HUB_REPO)/$(DOCKER_HUB_WEBSVC_IMAGE):latest

distclean: clean
	@rm -fr vendor/github.com vendor/golang.org

