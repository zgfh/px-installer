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

---

# Swarm/OCI prototype

Here's how one can make the PX-OCI run inside the Docker Swarm cluster.

### Challenge:

* the Docker Swarm's [service create](https://docs.docker.com/engine/reference/commandline/service_create) does not support `--privileged=true` (nor `--cap-add`, `--entrypoint`, `--net=host`) directives when creating service containers (see [moby:25303](https://github.com/moby/moby/issues/25303))
* this is required for "px-oci-monitor" to work properly, and install and control the external PX-OCI service via the [nsenter(1)](http://man7.org/linux/man-pages/man1/nsenter.1.html) command
    - workaround:
        1. use docker-service to set up a dummy "dockprox" container w/ mounted `docker.sock`,
        2. use the dummy "dockprox" container to start another (privileged) docker container

### Building "dockprox" container

* this build uses simple [Dockerfile](docker-proxy/Dockerfile), that starts with Ubuntu image, and adds the "/usr/bin/docker" command:

```bash
cd docker-proxy
make push
```

### Running "dockprox" global service

* we will run the "dockprox" container (e.g. `zoxpx/dockprox:latest`) as a global swarm service, and use it to kick-start the updated "oci-monitor" (e.g. `zoxpx/oci-monitor:latest`):

```bash
docker service create --name dockprox --mode=global --restart-condition none --detach=true \
    --mount type=bind,source=/var/run/docker.sock,target=/var/run/docker.sock \
    zoxpx/dockprox:latest run --net=host --privileged=true --name=oci-monitor --rm -i \
    -v /var/run/docker.sock:/var/run/docker.sock \
    -v /etc/pwx:/etc/pwx \
    -v /opt/pwx:/opt/pwx \
    -v /proc/1/ns:/host_proc/1/ns \
    -v /etc/systemd/system:/etc/systemd/system \
    zoxpx/oci-monitor:latest -k etcd://192.168.56.80:2379 -c kockica -d enp0s8 -m enp0s8 -s /dev/sdd -x swarm
```

### Results / Limitations:

* STARTUP **works OK**:
    - The workaround-procedure above will correctly:
        1. start the "dockprox" global service on all swarm-nodes,
        2. the "dockprox" will start the `oci-monitor` as a privileged container, which in turn
        3. will correctly spawn the external PX-OCI service.

* Service **removal DOES NOT WORK**:
    - Running the `docker service rm dockprox` will correctly remove the service, but
    - The spawned "oci-monitor" does not shut down (will still be running even after the parent "dockprox" service finished)
    - note that adding the "-it" to the "dockprox" invocation (which is supposed to make signals propagation work), actually *does not* work correctly as service startup fails with `ERROR: the input device is not a TTY`


**NEXT**:

If we'd to continue developing this solution, we should:

* replace the "dockprox" container with the original `oci-monitor`, also
* add the support into the `oci-monitor` to replace the "dockprox"'s docker-command, and start the privileged `oci-monitor` container
* make sure that the `oci-monitor` propagates the signals correctly, and passes the SIGSTOP to the privileged child container.
