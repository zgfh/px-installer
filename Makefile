# Makefile for PX-INSTALLER project
#

# VARIABLES
#
ifndef DOCKER_HUB_INSTALL_REPO
    DOCKER_HUB_INSTALL_REPO := portworx
    $(warning DOCKER_HUB_INSTALL_REPO not defined, using '$(DOCKER_HUB_INSTALL_REPO)' instead)
endif
ifndef DOCKER_HUB_MONITOR_IMAGE
    DOCKER_HUB_MONITOR_IMAGE := monitor
    $(warning DOCKER_HUB_MONITOR_IMAGE not defined, using '$(DOCKER_HUB_MONITOR_IMAGE)' instead)
endif
ifndef DOCKER_HUB_WEBSVC_IMAGE
    DOCKER_HUB_WEBSVC_IMAGE := monitor-websvc
    $(warning DOCKER_HUB_WEBSVC_IMAGE not defined, using '$(DOCKER_HUB_WEBSVC_IMAGE)' instead)
endif
ifndef DOCKER_HUB_RUNC_IMAGE
    DOCKER_HUB_RUNC_IMAGE := px-runcds
    $(warning DOCKER_HUB_RUNC_IMAGE not defined, using '$(DOCKER_HUB_RUNC_IMAGE)' instead)
endif
ifndef PX_INSTALLER_DOCKER_HUB_TAG
    PX_INSTALLER_DOCKER_HUB_TAG := $(shell git rev-parse HEAD | cut -c-7)
    $(warning PX_INSTALLER_DOCKER_HUB_TAG not defined, using '$(PX_INSTALLER_DOCKER_HUB_TAG)' instead)
endif

GO		:= go
GOENV		:= GOOS=linux GOARCH=amd64
SUDO		:= sudo
MONITOR_IMG	:= $(DOCKER_HUB_INSTALL_REPO)/$(DOCKER_HUB_MONITOR_IMAGE):$(PX_INSTALLER_DOCKER_HUB_TAG)
WEBSVC_IMG	:= $(DOCKER_HUB_INSTALL_REPO)/$(DOCKER_HUB_WEBSVC_IMAGE):$(PX_INSTALLER_DOCKER_HUB_TAG)
RUNC_IMG	:= $(DOCKER_HUB_INSTALL_REPO)/$(DOCKER_HUB_RUNC_IMAGE):$(PX_INSTALLER_DOCKER_HUB_TAG)

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

TARGETS += px-mon px-spec-websvc px-runcds
$(info  $(TARGETS))


# BUILD RULES
#

.PHONY: all deploy clean distclean vendor-pull px-container $(TARGETS)

all: $(TARGETS)

px-mon: px-mon/px-mon.go vendor/github.com/fsouza/go-dockerclient
	@echo "Building $@..."
	@cd px-mon && env $(GOENV) $(GO) build $(BUILD_OPTIONS)

px-spec-websvc: px-spec-websvc/px-spec-websvc.go vendor/github.com/gorilla/schema
	@echo "Building $@..."
	@cd px-spec-websvc && env $(GOENV) $(GO) build $(BUILD_OPTIONS)

px-runcds: px-runcds/px-runcds.go vendor/github.com/docker/docker/api
	@echo "Building $@ binary..."
	@cd px-runcds && env $(GOENV) $(GO) build $(BUILD_OPTIONS)

px-container:
	@cd px-mon && $(SUDO) docker build -t $(MONITOR_IMG) .
	@cd px-spec-websvc && $(SUDO) docker build -t $(WEBSVC_IMG) .
	@cd px-runcds && $(SUDO) docker build -t $(RUNC_IMG) .

$(GOPATH)/bin/govendor:
	$(GO) get -v github.com/kardianos/govendor

vendor-pull: $(GOPATH)/bin/govendor
	$(GOENV) $(GOPATH)/bin/govendor sync

vendor/github.com/fsouza/go-dockerclient: vendor-pull
vendor/github.com/docker/docker/api: vendor-pull
vendor/github.com/gorilla/schema: vendor-pull

deploy:
ifneq ($(DOCKER_HUB_PASSWD),)
	$(warning Found DOCKER_HUB_PASSWD env - using authenticated docker push)
	$(SUDO) docker login --username=$(DOCKER_HUB_USER) --password=$(DOCKER_HUB_PASSWD)
endif
	$(SUDO) docker push $(MONITOR_IMG)
	$(SUDO) docker push $(WEBSVC_IMG)
	$(SUDO) docker push $(RUNC_IMG)
	-$(SUDO) docker logout

clean:
	@rm -rf px-mon/px-mon px-spec-websvc/px-spec-websvc px-runcds/px-runcds
	-$(SUDO) docker rmi -f $(MONITOR_IMG) $(WEBSVC_IMG) $(RUNC_IMG)

distclean: clean
	@rm -fr vendor/github.com vendor/golang.org

