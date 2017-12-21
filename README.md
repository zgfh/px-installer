This repository is composed of following modules

### ~~px-init~~  (DEPRECATED)
* Used in kubernetes to run as an init container. This uses nsenter to enable shared mounts on the host systems

### ~~px-mon~~ (DEPRECATED)
* This is used in Docker Swarm only
* This is a privileged docker container that runs the actual Portworx container on the target nodes.
* The use case is for this to run as a daemon/agent on the target nodes so that it can ensure Portworx is running and also to make certain host system changes necessary to run Portworx (e.g enable shared mounts)

### px-oci-mon
* This is a "monitor" container for the OCI runC Portworx container (see [docs/runc](http://docs.portworx.com/runc)).
* This container is normally started directly as Kubernetes pod, and it'll install and start the external PX- OCI/runC service using identical environment/mount parameters.
* The installs and startup/restarts of the px-oci-mon pod are done intelligently, so it will not reinstall / restart PX-OCI service unless required.

### px-spec-websvc
* The goal for this web service is to take custom parameters from user's web request and produce a custom YAML output that users can supply to kubectl/docker commands to deploy Portworx
* Can be ran interactively without Docker, via `cd px-spec-websvc/templates; go run ../px-spec-websvc.go`

# Build

Common build targets:

```bash
# Build OCI-Monitor container
make px-oci-mon/px-oci-mon px-oci-mon-container

# Build Web service container
make px-spec-websvc/px-spec-websvc px-oci-mon-container

# .. or, Build all
make all px-container

# .. or, Build all and deploy (incl. re-tag as latest if needed)
make all px-container deploy deploy_latest
```

>**NOTE**:<br/>Take a look at [Makefile](Makefile) for the `DOCKER_HUB_*` variables that influence the build.

# Usage

### Kubernetes

Read [Kubernetes install](https://docs.portworx.com/scheduler/kubernetes/install.html) to see how this is used.

### Swarm

Read [Deploy Portworx on Docker Swarm or UCP](https://docs.portworx.com/scheduler/docker/install-px-docker-service.html) to see how this is used.

