all:
	go build -o portworx-mon installer.go
	go build -o portworx-websvc websvc.go
	@echo "Building container: docker build --tag $(DOCKER_HUB_REPO)/$(DOCKER_HUB_INSTALLER_IMAGE):$(DOCKER_HUB_TAG) ."
	sudo docker build --tag $(DOCKER_HUB_REPO)/$(DOCKER_HUB_INSTALLER_IMAGE):$(DOCKER_HUB_TAG) .

deploy:
	docker push $(DOCKER_HUB_REPO)/$(DOCKER_HUB_INSTALLER_IMAGE):$(DOCKER_HUB_TAG)
