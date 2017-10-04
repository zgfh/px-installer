PXINIT_IMG=$(DOCKER_HUB_INSTALL_REPO)/$(DOCKER_HUB_PXINIT_IMAGE):$(PX_INSTALLER_DOCKER_HUB_TAG)
MONITOR_IMG=$(DOCKER_HUB_INSTALL_REPO)/$(DOCKER_HUB_MONITOR_IMAGE):$(PX_INSTALLER_DOCKER_HUB_TAG)
WEBSVC_IMG=$(DOCKER_HUB_INSTALL_REPO)/$(DOCKER_HUB_WEBSVC_IMAGE):$(PX_INSTALLER_DOCKER_HUB_TAG)

all: clean pxinit pxmon pxwebsvc

pxinit:
	go build -o px-init/px-init px-init/px-init.go
	@echo "Building container: docker build --tag $(PXINIT_IMG) -f px-init/Dockerfile ."
	sudo docker build --tag $(PXINIT_IMG) -f px-init/Dockerfile px-init

pxmon:
	go build -o px-mon/px-mon px-mon/px-mon.go
	@echo "Building container: docker build --tag $(MONITOR_IMG) -f px-mon/Dockerfile ."
	sudo docker build --tag $(MONITOR_IMG) -f px-mon/Dockerfile px-mon

pxwebsvc:
	go build -o px-spec-websvc/px-spec-websvc px-spec-websvc/px-spec-websvc.go
	@echo "Building container: docker build --tag $(WEBSVC_IMG) -f px-spec-websvc/Dockerfile ."
	sudo docker build --tag $(WEBSVC_IMG) -f px-spec-websvc/Dockerfile px-spec-websvc

deploy: all
	docker push $(PXINIT_IMG)
	docker push $(MONITOR_IMG)
	docker push $(WEBSVC_IMG)

clean:
	-rm -rf px-init/px-init
	-rm -rf px-mon/px-mon
	-rm -rf px-spec-websvc/px-spec-websvc
	-docker rmi -f $(PXINIT_IMG)
	-docker rmi -f $(MONITOR_IMG)
	-docker rmi -f $(WEBSVC_IMG)
