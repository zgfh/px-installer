MONITOR_IMG=$(DOCKER_HUB_MONITOR_REPO)/$(DOCKER_HUB_MONITOR_IMAGE):$(DOCKER_HUB_TAG)
MONITOR_WEBSVC_IMG=$(DOCKER_HUB_MONITOR_REPO)/$(DOCKER_HUB_MONITOR_WEBSVC_IMAGE):$(DOCKER_HUB_TAG)

all: clean
	go build -o px-init px-init.go
	go build -o px-spec-websvc px-spec-websvc.go
	@echo "Building container: docker build --tag $(MONITOR_IMG) ."
	sudo docker build --tag $(MONITOR_IMG) .

	@echo "Building container: docker build --tag $(MONITOR_WEBSVC_IMG) ."
	sudo docker build --tag $(MONITOR_WEBSVC_IMG) -f websvc.Dockerfile .

deploy: all
	docker push $(MONITOR_IMG)
	docker push $(MONITOR_WEBSVC_IMG)

clean:
	-docker rmi -f $(MONITOR_IMG)
	-docker rmi -f $(MONITOR_WEBSVC_IMG)
