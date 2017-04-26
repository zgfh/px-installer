all:
	go build .
	@echo "Building container: docker build --tag $(DOCKER_HUB_REPO)/$(DOCKER_HUB_INSTALLER_IMAGE):$(DOCKER_HUB_TAG) ."
	sudo docker build --tag $(DOCKER_HUB_REPO)/$(DOCKER_HUB_INSTALLER_IMAGE):$(DOCKER_HUB_TAG) .

deploy:
	docker push $(DOCKER_HUB_REPO)/$(DOCKER_HUB_INSTALLER_IMAGE):$(DOCKER_HUB_TAG)
