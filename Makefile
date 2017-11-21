# Makefile for PX-INSTALLER project
#

# VARIABLES
#
ifndef DOCKER_HUB_REPO
    DOCKER_HUB_REPO := $(shell id -un)px
    $(warning DOCKER_HUB_REPO not defined, using '$(DOCKER_HUB_REPO)' instead)
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

ifdef PXTAG
    LDFLAGS += -X main.PXTAG=$(PXTAG)
    DOCKER_HUB_TAG := $(PXTAG)
    $(warning Using PXTAG '$(PXTAG)' to set up dependencies, and DOCKER_HUB_TAG)
endif

GO		:= go
GOENV		:= GOOS=linux GOARCH=amd64
SUDO		:= sudo
OCIMON_IMG	:= $(DOCKER_HUB_REPO)/$(DOCKER_HUB_OCIMON_IMAGE):$(DOCKER_HUB_TAG)
WEBSVC_IMG	:= $(DOCKER_HUB_REPO)/$(DOCKER_HUB_WEBSVC_IMAGE):$(DOCKER_HUB_TAG)

BUILD_TYPE=static
ifeq ($(BUILD_TYPE),static)
    LDFLAGS += -extldflags -static
    BUILD_OPTIONS += -v -a -ldflags "$(LDFLAGS)"
    GOENV += CGO_ENABLED=0
else ifeq ($(BUILD_TYPE),debug)
    BUILD_OPTIONS += -i -v -gcflags "-N -l" -ldflags "$(LDFLAGS)"
else
    BUILD_OPTIONS += -i -v -ldflags "$(LDFLAGS)"
endif

ifeq ($(shell id -u),0)
    SUDO :=
endif

TARGETS += px-spec-websvc/px-spec-websvc px-oci-mon/px-oci-mon


# BUILD RULES
#

.PHONY: all deploy rmi clean distclean vendor-sync px-container

all: $(TARGETS)


px-oci-mon/px-oci-mon: px-oci-mon/main.go
	@echo "Building $@ binary..."
	@cd px-oci-mon && env $(GOENV) $(GO) build $(BUILD_OPTIONS)

px-spec-websvc/px-spec-websvc: px-spec-websvc/px-spec-websvc.go
	@echo "Building $@ binary..."
	@cd px-spec-websvc && env $(GOENV) $(GO) build $(BUILD_OPTIONS)


px-oci-mon-container:
	@echo "Building $@ ..."
	@cd px-oci-mon && $(SUDO) docker build -t $(OCIMON_IMG) .

px-spec-websvc-container:
	@echo "Building $@ ..."
	@cd px-spec-websvc && $(SUDO) docker build -t $(WEBSVC_IMG) .

px-container: px-oci-mon-container px-spec-websvc-container

vendor-sync:
	govendor sync

deploy:
	@echo "Deploying all containers..."
ifneq ($(DOCKER_HUB_PASSWD),)
	$(warning Found DOCKER_HUB_PASSWD env - using authenticated docker push)
	$(SUDO) docker login --username=$(DOCKER_HUB_USER) --password=$(DOCKER_HUB_PASSWD)
endif
	$(SUDO) docker push $(OCIMON_IMG)
	$(SUDO) docker push $(WEBSVC_IMG)

deploy_latest:
	@echo "Re-Deploying current containers as TAG:latest..."
ifneq ($(DOCKER_HUB_TAG),latest)
ifneq ($(DOCKER_HUB_PASSWD),)
	$(warning Found DOCKER_HUB_PASSWD env - using authenticated docker push)
	$(SUDO) docker login --username=$(DOCKER_HUB_USER) --password=$(DOCKER_HUB_PASSWD)
endif
	$(SUDO) docker tag $(OCIMON_IMG) $(DOCKER_HUB_REPO)/$(DOCKER_HUB_OCIMON_IMAGE):latest
	$(SUDO) docker push $(DOCKER_HUB_REPO)/$(DOCKER_HUB_OCIMON_IMAGE):latest
	$(SUDO) docker tag $(WEBSVC_IMG) $(DOCKER_HUB_REPO)/$(DOCKER_HUB_WEBSVC_IMAGE):latest
	$(SUDO) docker push $(DOCKER_HUB_REPO)/$(DOCKER_HUB_WEBSVC_IMAGE):latest
endif

rmi:
	-$(SUDO) docker rmi -f $(WEBSVC_IMG) $(OCIMON_IMG) \
	    $(DOCKER_HUB_REPO)/$(DOCKER_HUB_OCIMON_IMAGE):latest \
	    $(DOCKER_HUB_REPO)/$(DOCKER_HUB_WEBSVC_IMAGE):latest

clean:
	rm -f $(TARGETS)

distclean: rmi clean

