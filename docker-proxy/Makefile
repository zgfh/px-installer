# VARIABLES
#
ifndef DOCKER_HUB_REPO
    DOCKER_HUB_REPO := $(shell id -un)px
    $(warning DOCKER_HUB_REPO not defined, using '$(DOCKER_HUB_REPO)' instead)
endif
ifndef DOCKER_HUB_DOCKPROX_IMAGE
    DOCKER_HUB_DOCKPROX_IMAGE := dockprox
    $(warning DOCKER_HUB_DOCKPROX_IMAGE not defined, using '$(DOCKER_HUB_DOCKPROX_IMAGE)' instead)
endif
ifndef DOCKER_HUB_TAG
    DOCKER_HUB_TAG := $(shell git rev-parse HEAD | cut -c-7)
    #DOCKER_HUB_TAG := latest
    $(warning DOCKER_HUB_TAG not defined, using '$(DOCKER_HUB_TAG)' instead)
endif

DOCKER_IMG := $(DOCKER_HUB_REPO)/$(DOCKER_HUB_DOCKPROX_IMAGE):$(DOCKER_HUB_TAG)

.PHONY: all build push clean

docker: /usr/bin/docker
	cp -a $< $@

build: Dockerfile docker
	docker build -t $(DOCKER_IMG) .

push: build
	docker push $(DOCKER_IMG)

clean:
	-docker rmi -f $(DOCKER_IMG)
	rm -fr /usr/bin/docker

