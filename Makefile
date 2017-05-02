all: clean
	go build -o portworx-mon installer.go
	go build -o portworx-mon-websvc websvc.go
	@echo "Building container: docker build --tag $(DOCKER_HUB_INSTALLER_REPO)/$(DOCKER_HUB_INSTALLER_IMAGE):$(DOCKER_HUB_TAG) ."
	sudo docker build --tag $(DOCKER_HUB_INSTALLER_REPO)/$(DOCKER_HUB_INSTALLER_IMAGE):$(DOCKER_HUB_TAG) .

	@echo "Building container: docker build --tag $(DOCKER_HUB_INSTALLER_REPO)/$(DOCKER_HUB_INSTALLER_WEBSVC_IMAGE):$(DOCKER_HUB_TAG) ."
	sudo docker build --tag $(DOCKER_HUB_INSTALLER_REPO)/$(DOCKER_HUB_INSTALLER_WEBSVC_IMAGE):$(DOCKER_HUB_TAG) -f websvc.Dockerfile .

deploy: all
	docker push $(DOCKER_HUB_INSTALLER_REPO)/$(DOCKER_HUB_INSTALLER_IMAGE):$(DOCKER_HUB_TAG)
	docker push $(DOCKER_HUB_INSTALLER_REPO)/$(DOCKER_HUB_INSTALLER_WEBSVC_IMAGE):$(DOCKER_HUB_TAG)

clean:
	-docker rmi -f $(DOCKER_HUB_INSTALLER_REPO)/$(DOCKER_HUB_INSTALLER_IMAGE):$(DOCKER_HUB_TAG)
	-docker rmi -f $(DOCKER_HUB_INSTALLER_REPO)/$(DOCKER_HUB_INSTALLER_WEBSVC_IMAGE):$(DOCKER_HUB_TAG)
