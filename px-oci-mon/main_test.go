package main

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestIsRestartRequired(t *testing.T) {
	var data = []struct {
		log         string
		expectation bool
	}{
		{`time="2017-09-30T05:48:06Z" level=info msg="> Updated mounts: add{/var/lib/kubelet/pods/e5b67f9c-a5a2-11e7-922e-5254007a695a/etc-hosts:/etc/hosts /var/lib/kubelet/pods/e5b67f9c-a5a2-11e7-922e-5254007a695a/containers/portworx/064db72c:/tmp/px-termination-log /var/lib/kubelet/pods/e5b67f9c-a5a2-11e7-922e-5254007a695a/volumes/kubernetes.io~secret/px-account-token-kx5pz:/var/run/secrets/kubernetes.io/serviceaccount:ro} rm{/var/lib/kubelet/pods/209b4313-a596-11e7-9508-5254007a695a/etc-hosts:/etc/hosts /var/lib/kubelet/pods/209b4313-a596-11e7-9508-5254007a695a/containers/portworx/a0f279f8:/tmp/px-termination-log /var/lib/kubelet/pods/209b4313-a596-11e7-9508-5254007a695a/volumes/kubernetes.io~secret/px-account-token-kx5pz:/var/run/secrets/kubernetes.io/serviceaccount:ro}"
`, false},
		{`time="2017-09-30T05:48:06Z" level=info msg="Rootfs found at /opt/pwx/oci/rootfs"
time="2017-09-30T05:48:06Z" level=info msg="SPEC UPDATED [0892499e680147b53335759aa487886e  /opt/pwx/oci/config.json]"
time="2017-09-30T05:48:06Z" level=info msg="> Updated mounts: add{/var/lib/kubelet/pods/e5b67f9c-a5a2-11e7-922e-5254007a695a/etc-hosts:/etc/hosts /var/lib/kubelet/pods/e5b67f9c-a5a2-11e7-922e-5254007a695a/containers/portworx/064db72c:/tmp/px-termination-log /var/lib/kubelet/pods/e5b67f9c-a5a2-11e7-922e-5254007a695a/volumes/kubernetes.io~secret/px-account-token-kx5pz:/var/run/secrets/kubernetes.io/serviceaccount:ro} rm{/var/lib/kubelet/pods/209b4313-a596-11e7-9508-5254007a695a/etc-hosts:/etc/hosts /var/lib/kubelet/pods/209b4313-a596-11e7-9508-5254007a695a/containers/portworx/a0f279f8:/tmp/px-termination-log /var/lib/kubelet/pods/209b4313-a596-11e7-9508-5254007a695a/volumes/kubernetes.io~secret/px-account-token-kx5pz:/var/run/secrets/kubernetes.io/serviceaccount:ro}"
time="2017-09-30T05:48:06Z" level=info msg="PX-RunC arguments: -a -c mycluster22 -d eth1 -f -k etcd://192.168.56.1:2379 -m eth1 -x kubernetes"
time="2017-09-30T05:48:06Z" level=info msg="PX-RunC mounts: /dev/:/dev/ /var/lib/kubelet/pods/e5b67f9c-a5a2-11e7-922e-5254007a695a/etc-hosts:/etc/hosts /etc/pwx/:/etc/pwx/ /etc/resolv.conf:/etc/resolv.conf:ro /opt/pwx/bin/:/export_bin/ /lib/modules/:/lib/modules/ proc:/proc/:nosuid,noexec,nodev /run/docker/:/run/docker/ sysfs:/sys/:nosuid,noexec,nodev cgroup:/sys/fs/cgroup/:nosuid,noexec,nodev /var/lib/kubelet/pods/e5b67f9c-a5a2-11e7-922e-5254007a695a/containers/portworx/064db72c:/tmp/px-termination-log /usr/src/:/usr/src/ /var/cores/:/var/cores/ /var/run/:/var/host_run/ /var/lib/kubelet:/var/lib/kubelet:shared /var/lib/osd:/var/lib/osd:shared /var/lib/osd/:/var/lib/osd/:shared /var/lib/kubelet/pods/e5b67f9c-a5a2-11e7-922e-5254007a695a/volumes/kubernetes.io~secret/px-account-token-kx5pz:/var/run/secrets/kubernetes.io/serviceaccount:ro"
time="2017-09-30T05:48:06Z" level=info msg="PX-RunC env: BTRFS_SOURCE=/home/px_btrfs GOMAXPROCS=64 GOTRACEBACK=crash KUBERNETES_PORT=tcp://10.96.0.1:443 KUBERNETES_PORT_443_TCP=tcp://10.96.0.1:443 KUBERNETES_PORT_443_TCP_ADDR=10.96.0.1 KUBERNETES_PORT_443_TCP_PORT=443 KUBERNETES_PORT_443_TCP_PROTO=tcp KUBERNETES_SERVICE_HOST=10.96.0.1 KUBERNETES_SERVICE_PORT=443 KUBERNETES_SERVICE_PORT_HTTPS=443 KUBE_DNS_PORT=udp://10.96.0.10:53 KUBE_DNS_PORT_53_TCP=tcp://10.96.0.10:53 KUBE_DNS_PORT_53_TCP_ADDR=10.96.0.10 KUBE_DNS_PORT_53_TCP_PORT=53 KUBE_DNS_PORT_53_TCP_PROTO=tcp KUBE_DNS_PORT_53_UDP=udp://10.96.0.10:53 KUBE_DNS_PORT_53_UDP_ADDR=10.96.0.10 KUBE_DNS_PORT_53_UDP_PORT=53 KUBE_DNS_PORT_53_UDP_PROTO=udp KUBE_DNS_SERVICE_HOST=10.96.0.10 KUBE_DNS_SERVICE_PORT=53 KUBE_DNS_SERVICE_PORT_DNS=53 KUBE_DNS_SERVICE_PORT_DNS_TCP=53 PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin PORTWORX_SERVICE_PORT=tcp://10.110.109.163:9001 PORTWORX_SERVICE_PORT_9001_TCP=tcp://10.110.109.163:9001 PORTWORX_SERVICE_PORT_9001_TCP_ADDR=10.110.109.163 PORTWORX_SERVICE_PORT_9001_TCP_PORT=9001 PORTWORX_SERVICE_PORT_9001_TCP_PROTO=tcp PORTWORX_SERVICE_SERVICE_HOST=10.110.109.163 PORTWORX_SERVICE_SERVICE_PORT=9001 PXMOD_SOURCE=/home/px-fuse PXMOD_VERSION=5 PX_IMAGE=zoxpx/px-dev PX_RUNC=true TERM=xterm ZOX=was here..."
time="2017-09-30T05:48:06Z" level=info msg="Successfully written /etc/systemd/system/portworx.service"
time="2017-09-30T05:48:06Z" level=info msg="Stopping Portworx service (if any)"
time="2017-09-30T05:48:06Z" level=info msg="> run: /bin/sh -c systemctl stop portworx"
Warning: portworx.service changed on disk. Run 'systemctl daemon-reload' to reload units.
time="2017-09-30T05:48:06Z" level=info msg="Starting Portworx service"
time="2017-09-30T05:48:06Z" level=info msg="> run: /bin/sh -c systemctl daemon-reload && systemctl enable portworx && systemctl start portworx"
time="2017-09-30T05:48:07Z" level=info msg="Install done - going to sleep"`, false},
		{`INFO[0000] Rootfs found at /opt/pwx/oci/rootfs
INFO[0000] SPEC CREATED [cc824f500363f0cf4e60c570cf9e8931  /opt/pwx/oci/config.json]
INFO[0000] PX-RunC arguments: -c zox-dbg-mk126 -d enp0s8 -k etcd://70.0.0.65:2379 -m enp0s8 -s /dev/sdc -x kubernetes
INFO[0000] PX-RunC mounts: /dev/:/dev/ /etc/hosts:/etc/hosts:ro /etc/pwx/:/etc/pwx/ /etc/resolv.conf:/etc/resolv.conf:ro /opt/pwx/bin/:/export_bin/ /lib/modules/:/lib/modules/ proc:/proc/:nosuid,noexec,nodev /run/docker/:/run/docker/ sysfs:/sys/:nosuid,noexec,nodev cgroup:/sys/fs/cgroup/:nosuid,noexec,nodev /usr/src/:/usr/src/ /var/cores/:/var/cores/ /var/run/:/var/host_run/ /var/lib/kubelet:/var/lib/kubelet:shared /var/lib/osd/:/var/lib/osd/:shared
INFO[0000] PX-RunC env: PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin TERM=xterm GOTRACEBACK=crash GOMAXPROCS=64 PXMOD_SOURCE=/home/px-fuse PXMOD_VERSION=5 BTRFS_SOURCE=/home/px_btrfs PX_RUNC=true
INFO[0000] Successfully written /etc/systemd/system/portworx.service`, true},
		{`INFO[0000] Rootfs found at /opt/pwx/oci/rootfs
INFO[0000] SPEC UNCHANGED [cc824f500363f0cf4e60c570cf9e8931 /opt/pwx/oci/config.json]
INFO[0000] PX-RunC arguments: -c zox-dbg-mk126 -d enp0s8 -k etcd://70.0.0.65:2379 -m enp0s8 -s /dev/sdc -x kubernetes
INFO[0000] PX-RunC mounts: /dev/:/dev/ /etc/hosts:/etc/hosts:ro /etc/pwx/:/etc/pwx/ /etc/resolv.conf:/etc/resolv.conf:ro /opt/pwx/bin/:/export_bin/ /lib/modules/:/lib/modules/ proc:/proc/:nosuid,noexec,nodev /run/docker/:/run/docker/ sysfs:/sys/:nosuid,noexec,nodev cgroup:/sys/fs/cgroup/:nosuid,noexec,nodev /usr/src/:/usr/src/ /var/cores/:/var/cores/ /var/run/:/var/host_run/ /var/lib/kubelet:/var/lib/kubelet:shared /var/lib/osd/:/var/lib/osd/:shared
INFO[0000] PX-RunC env: PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin TERM=xterm GOTRACEBACK=crash GOMAXPROCS=64 PXMOD_SOURCE=/home/px-fuse PXMOD_VERSION=5 BTRFS_SOURCE=/home/px_btrfs PX_RUNC=true
INFO[0000] Successfully written /etc/systemd/system/portworx.service`, false},
	}

	origFn := getKubernetesRootDirFn
	getKubernetesRootDirFn = func() (string, error) {
		return "/var/lib/kubelet", nil
	}
	defer func() {
		getKubernetesRootDirFn = origFn
	}()

	for _, v := range data {
		assert.Equal(t, v.expectation, isRestartRequired(v.log),
			"Was expecting isRestartRequired()=%v for `%s`", v.expectation, v.log)
	}
}
