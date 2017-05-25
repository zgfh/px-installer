kind: ConfigMap
apiVersion: v1
metadata:
  name: portworx-config
  namespace: kube-system
data:
  cluster.name: portworx-storage
  cluster.scheduler: kubernetes
  cluster.store: etcd://10.3.64.162:2379
  node.interface.management: eth0
  node.interface.data: eth0
  worker.devices: |
    -s /dev/xvdf
---
apiVersion: extensions/v1beta1
kind: DaemonSet
metadata:
  name: portworx
  labels:
    app: portworx
  namespace: kube-system
spec:
  template:
    metadata:
      labels:
        app: portworx
      namespace: kube-system
    spec:
      tolerations:
      - key: node-role.kubernetes.io/master
        operator: Equal
        effect: NoSchedule
      affinity:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
            - matchExpressions:
              - key: node-role.kubernetes.io/master
                operator: Exists
      hostNetwork: true
      hostIPC: true
      containers:
      - name: portworx
        image: portworx/px-dev
        imagePullPolicy: Always
        securityContext:
          privileged: true
        args:
        - "-daemon"
        - "-k $(PX_KV_STORE)"
        - "-c $(PWX_CLUSTER_ID)"
        - "-m $(PWX_IFACE_MGMT)"
        - "-d $(PWX_IFACE_DATA)"
        - "-x $(PX_SCHEDULER)"
        - "-z"
        env:
        - name: PX_KV_STORE
          valueFrom:
            configMapKeyRef:
              name: portworx-config
              key: cluster.store
        - name: PWX_CLUSTER_ID
          valueFrom:
            configMapKeyRef:
              name: portworx-config
              key: cluster.name
        - name: PX_SCHEDULER
          valueFrom:
            configMapKeyRef:
              name: portworx-config
              key: cluster.scheduler
        - name: PWX_IFACE_DATA
          valueFrom:
            configMapKeyRef:
              name: portworx-config
              key: node.interface.data
        - name: PWX_IFACE_MGMT
          valueFrom:
            configMapKeyRef:
              name: portworx-config
              key: node.interface.management
        volumeMounts:
        - name: docker-plugins
          mountPath: /run/docker/plugins
        - name: var-lib-osd
          mountPath: /var/lib/osd:shared
        - name: dev
          mountPath: /dev
        - name: etc-pwx
          mountPath: /etc/pwx
        - name: opt-pwx-bin
          mountPath: /export_bin
        - name: docker-sock
          mountPath: /var/run/docker.sock
        - name: var-cores
          mountPath: /var/cores
        - name: var-lib-kubelet
          mountPath: /var/lib/kubelet:shared
        - name: lib-modules
          mountPath: /lib/modules
#        - name: etc-kubernetes-ca-crt
#          mountPath: /etc/kubernetes/ca.crt
#        - name: etc-pwx-kubernetes-yaml
#          mountPath: /etc/pwx/kubernetes.yaml
      volumes:
#      - name: etc-pwx-kubernetes-yaml
#        hostPath:
#          path: /etc/kubernetes/kubeconfig
#      - name: etc-kubernetes-ca-crt
#        hostPath:
#          path: /etc/kubernetes/ca.crt
      - name: docker-plugins
        hostPath:
          path: /run/docker/plugins
      - name: var-lib-osd
        hostPath:
          path: /var/lib/osd
      - name: dev
        hostPath:
          path: /dev
      - name: etc-pwx
        hostPath:
          path: /etc/pwx
      - name: opt-pwx-bin
        hostPath:
          path: /opt/bin
      - name: docker-sock
        hostPath:
          path: /var/run/docker.sock
      - name: var-cores
        hostPath:
          path: /var/cores
      - name: var-lib-kubelet
        hostPath:
          path: /var/lib/kubelet
      - name: lib-modules
        hostPath:
          path: /lib/modules
---
apiVersion: extensions/v1beta1
kind: DaemonSet
metadata:
  name: portworx-peer
  labels:
    app: portworx-peer
  namespace: kube-system
spec:
  template:
    metadata:
      labels:
        app: portworx-peer
      namespace: kube-system
    spec:
      hostNetwork: true
      hostIPC: true
      containers:
      - name: portworx-consumer
        image: portworx/px-dev
        imagePullPolicy: Always
        securityContext:
          privileged: true
        env:
        - name: PWX_CLUSTER_ID
          value: portworx-storage
        args:
        - "-daemon"
        - "-k $(PX_KV_STORE)"
        - "-c $(PWX_CLUSTER_ID)"
        - "-m $(PWX_IFACE_MGMT)"
        - "-d $(PWX_IFACE_DATA)"
        - "-x $(PX_SCHEDULER)"
        - "-z"
        - "$(PX_STORAGE_DEVICE)"
        env:
        - name: PX_KV_STORE
          valueFrom:
            configMapKeyRef:
              name: portworx-config
              key: cluster.store
        - name: PWX_CLUSTER_ID
          valueFrom:
            configMapKeyRef:
              name: portworx-config
              key: cluster.name
        - name: PX_SCHEDULER
          valueFrom:
            configMapKeyRef:
              name: portworx-config
              key: cluster.scheduler
        - name: PWX_IFACE_DATA
          valueFrom:
            configMapKeyRef:
              name: portworx-config
              key: node.interface.data
        - name: PWX_IFACE_MGMT
          valueFrom:
            configMapKeyRef:
              name: portworx-config
              key: node.interface.management
        - name: PX_STORAGE_DEVICE
          valueFrom:
            configMapKeyRef:
              name: portworx-config
              key: worker.devices
        volumeMounts:
        - name: docker-plugins
          mountPath: /run/docker/plugins
        - name: var-lib-osd
          mountPath: /var/lib/osd:shared
        - name: dev
          mountPath: /dev
        - name: etc-pwx
          mountPath: /etc/pwx
        - name: opt-pwx-bin
          mountPath: /export_bin
        - name: docker-sock
          mountPath: /var/run/docker.sock
        - name: var-cores
          mountPath: /var/cores
        - name: var-lib-kubelet
          mountPath: /var/lib/kubelet:shared
        - name: lib-modules
          mountPath: /lib/modules
#        - name: etc-pwx-cafile
#          mountPath: /etc/pwx/my_cafile
#        - name: root-kube-config
#          mountPath: /root/.kube/config
      volumes:
#      - name: root-kube-config
#        hostPath:
#          path: /etc/kubernetes/kubeconfig
#      - name: etc-pwx-cafile
#        hostPath:
#          path: /etc/kubernetes/ca.crt
      - name: docker-plugins
        hostPath:
          path: /run/docker/plugins
      - name: var-lib-osd
        hostPath:
          path: /var/lib/osd
      - name: dev
        hostPath:
          path: /dev
      - name: etc-pwx
        hostPath:
          path: /etc/pwx
      - name: opt-pwx-bin
        hostPath:
          path: /opt/bin
      - name: docker-sock
        hostPath:
          path: /var/run/docker.sock
      - name: var-cores
        hostPath:
          path: /var/cores
      - name: var-lib-kubelet
        hostPath:
          path: /var/lib/kubelet
      - name: lib-modules
        hostPath:
          path: /lib/modules