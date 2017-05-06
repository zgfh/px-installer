# portworx-mon

This repository is composed of 2 main modules
### Monitor
* This is a privileged docker container that runs the actual Portworx container on the target nodes.
* The use case is for this to run as a daemon/agent on the target nodes so that it can ensure Portworx is running and also to make certain host system changes necessary to run Portworx (e.g enable shared mounts)

### Monitor Web Service
* Currently, the web service is only relevant to Kubernetes
* The goal for this web service is to take custom parameters from user's web request and spit out a custom YAML output that users can supply to kubectl commands to deploy the monitor

# Build

Make sure your environment has following environment variables set
```
export DOCKER_HUB_MONITOR_REPO=portworx # or your private docker repo (if you change this, also update image in the .gtpl template)
export DOCKER_HUB_MONITOR_IMAGE=monitor
export DOCKER_HUB_MONITOR_WEBSVC_IMAGE=monitor-websvc
```

Compile and push the docker image using following command
```
make deploy
```

This compiles the px monitor and px web service source code, creates a container for each of them and pushes them to the configured docker respository.

# Usage

### Kubernetes

Portworx needs to run on the kubernetes master node (_This requirement will be removed soon_). To allow that, run the following command.
```
kubectl taint nodes --all node-role.kubernetes.io/master-
```

Now deploy Portworx.
```
kubectl create -f "http://35.185.236.78?cluster=mycluster&etcd=etcd://etcd.fake.net:4001"
```
Above command fetches the YAML spec from the web service and gives it to kubectl to create a Portworx monitor DaemonSet.
* This monitor ensures Portworx is running and maintains it's lifecycle
* The YAML spec declares details of the px-mon (image, parameters etc)
* Note how we give custom parameters which are specific to each setup.


Examples including optional parameters
```
# To specify data and management interfaces
 kubectl create -f "http://35.185.236.78?cluster=mycluster&etcd=etcd://etcd.fake.net:4001&diface=enp0s8&miface=enp0s8"

# To specify drive
 kubectl create -f "http://35.185.236.78?cluster=mycluster&etcd=etcd://etcd.fake.net:4001&drive=/dev/sdb"
```
