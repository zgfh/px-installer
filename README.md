This repository is composed of following modules

### px-init
* Used in kubernetes to run as an init container. This uses nsenter to enable shared mounts on the host systems

### px-mon
* This is used in Docker Swarm only
* This is a privileged docker container that runs the actual Portworx container on the target nodes.
* The use case is for this to run as a daemon/agent on the target nodes so that it can ensure Portworx is running and also to make certain host system changes necessary to run Portworx (e.g enable shared mounts)

### px-spec-websvc
* The goal for this web service is to take custom parameters from user's web request and spit out a custom YAML output that users can supply to kubectl/docker commands to deploy portworx

# Build

Make sure your environment has following environment variables set
```
export DOCKER_HUB_INSTALL_REPO=portworx # or your private docker repo (if you change this, also update image in the .gtpl template)
export DOCKER_HUB_MONITOR_IMAGE=monitor
export DOCKER_HUB_WEBSVC_IMAGE=monitor-websvc
export DOCKER_HUB_OCIMON_IMAGE=oci-monitor
export PX_INSTALLER_DOCKER_HUB_TAG=1.0.0
```

Note: If you set `DOCKER_HUB_INSTALL_REPO` to `portworx`, it will update the official image which external customers are using.

Compile binaries: `make`
* This compiles all modules, creates a container for each of them

Compile and push the docker images: `make deploy`
* This compiles all modules, creates a container for each of them and pushes them to the configured docker respository.

# Usage

### Kubernetes

Read [Kubernetes install](https://docs.portworx.com/scheduler/kubernetes/install.html) to see how this is used.

### Swarm

Read [Deploy Portworx on Docker Swarm or UCP](https://docs.portworx.com/scheduler/docker/install-px-docker-service.html) to see how this is used.
