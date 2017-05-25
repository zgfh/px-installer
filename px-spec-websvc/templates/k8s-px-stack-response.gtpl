apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: etcd-discovery
  namespace: kube-system
spec:
  strategy:
    type: Recreate
  replicas: 1
  selector:
    matchLabels:
      name: etcd-discovery
  template:
    metadata:
      labels:
        name: etcd-discovery
    spec:
      containers:
      - name: discovery
        image: openshift/etcd-20-centos7
        args:
        - etcd-discovery.sh
        ports:
        - containerPort: 2379
          protocol: TCP
        resources: {}
        terminationMessagePath: "/dev/termination-log"
        imagePullPolicy: IfNotPresent
        securityContext:
          capabilities: {}
          privileged: false
      restartPolicy: Always
      dnsPolicy: ClusterFirst
      serviceAccount: ''
status: {}
---
kind: Service
apiVersion: v1
metadata:
  name: etcd-discovery
  namespace: kube-system
  labels:
    name: etcd-discovery
spec:
  ports:
  - protocol: TCP
    port: 2379
    targetPort: 2379
    nodePort: 0
  selector:
    name: etcd-discovery
  sessionAffinity: None
  type: ClusterIP
status:
  loadBalancer: {}
---
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: etcd
  namespace: kube-system
spec:
  strategy:
    type: Recreate
  replicas: 3
  selector:
    matchLabels:
      name: etcd
  template:
    metadata:
      labels:
        name: etcd
    spec:
      containers:
      - name: member
        image: openshift/etcd-20-centos7
        ports:
        - containerPort: 2379
          protocol: TCP
        - containerPort: 2380
          protocol: TCP
        env:
          # ETCD_NUM_MEMBERS is the maximum number of members to launch (have to match with # of replicas)
        - name: ETCD_NUM_MEMBERS
          value: "3"
        - name: ETCD_INITIAL_CLUSTER_STATE
          value: "new"
          # ETCD_INITIAL_CLUSTER_TOKEN is a token etcd uses to generate unique cluster ID and member ID. Conforms to [a-z0-9]{40}
        - name: ETCD_INITIAL_CLUSTER_TOKEN
          value: INSERT_ETCD_INITIAL_CLUSTER_TOKEN
          # ETCD_DISCOVERY_TOKEN is a unique token used by the discovery service. Conforms to etcd-cluster-[a-z0-9]{5}
        - name: ETCD_DISCOVERY_TOKEN
          value: INSERT_ETCD_DISCOVERY_TOKEN
          # ETCD_DISCOVERY_URL connects etcd instances together by storing a list of peer addresses,
          # metadata and the initial size of the cluster under a unique address
        - name: ETCD_DISCOVERY_URL
          value: "http://etcd-discovery:2379"
        - name: ETCDCTL_PEERS
          value: "http://etcd:2379"
        resources: {}
        terminationMessagePath: "/dev/termination-log"
        imagePullPolicy: IfNotPresent
        securityContext:
          capabilities: {}
          privileged: false
      restartPolicy: Always
      dnsPolicy: ClusterFirst
      serviceAccount: ''
status: {}
---
kind: Service
apiVersion: v1
metadata:
  name: etcd
  namespace: kube-system
  labels:
    name: etcd
spec:
  ports:
  - name: client
    protocol: TCP
    port: 2379
    targetPort: 2379
    nodePort: 0
  - name: server
    protocol: TCP
    port: 2380
    targetPort: 2380
    nodePort: 0
  selector:
    name: etcd
  sessionAffinity: None
  type: ClusterIP
status:
  loadBalancer: {}
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: px-account
  namespace: kube-system
---
kind: ClusterRole
apiVersion: rbac.authorization.k8s.io/v1alpha1
metadata:
   name: node-get-put-list-role
rules:
- apiGroups: [""]
  resources: ["nodes"]
  verbs: ["get", "update", "list"]
---
kind: ClusterRoleBinding
apiVersion: rbac.authorization.k8s.io/v1alpha1
metadata:
  name: node-role-binding
subjects:
- apiVersion: v1
  kind: ServiceAccount
  name: px-account
  namespace: kube-system
roleRef:
  kind: ClusterRole
  name: node-get-put-list-role
  apiGroup: rbac.authorization.k8s.io

---

kind: Service
apiVersion: v1
metadata:
  name: portworx-service
  namespace: kube-system
spec:
  selector:
    name: portworx
  ports:
    - protocol: TCP
      port: 9001
      targetPort: 9001
  type: NodePort

---

apiVersion: extensions/v1beta1
kind: DaemonSet
metadata:
  name: portworx
  namespace: kube-system
spec:
  minReadySeconds: 0
  updateStrategy:
    type: RollingUpdate
    rollingUpdate:
      maxUnavailable: 1
  template:
    metadata:
      labels:
        name: portworx
    spec:
      hostNetwork: true
      hostPID: true
      containers:
        - name: portworx
          image: portworx/px-enterprise:latest
          imagePullPolicy: Always
          args:
             ["-k {{if .Kvdb}}{{.Kvdb}}{{else}}etcd://etcd:2379{{end}}",
              "{{if .Cluster}}-c {{.Cluster}}{{end}}",
              "{{if .DIface}}-d {{.DIface}}{{end}}",
              "{{if .MIface}}-m {{.MIface}}{{end}}",
              "{{if .Drives}}{{.Drives}}{{end}}",
              "{{if .EtcdPasswd}}-userpwd {{.EtcdPasswd}}{{end}}",
              "{{if .EtcdCa}}-ca {{.EtcdCa}}{{end}}",
              "{{if .EtcdCert}}-cert {{.EtcdCert}}{{end}}",
              "{{if .EtcdKey}}-key {{.EtcdKey}}{{end}}",
              "{{if .Acltoken}}-acltoken {{.Acltoken}}{{end}}",
              "{{if .Token}}-t {{.Token}}{{end}}",
              "{{if .Env}}{{.Env}}{{end}}",
              "-x", "kubernetes"]
          livenessProbe:
            initialDelaySeconds: 840 # allow image pull in slow networks
            httpGet:
              host: 127.0.0.1
              path: /status
              port: 9001
          readinessProbe:
            periodSeconds: 10
            httpGet:
              host: 127.0.0.1
              path: /status
              port: 9001
          securityContext:
            privileged: true
          volumeMounts:
            - name: dockersock
              mountPath: /var/run/docker.sock
            - name: libosd
              mountPath: /var/lib/osd:shared
            - name: dev
              mountPath: /dev
            - name: etcpwx
              mountPath: /etc/pwx/
            - name: optpwx
              mountPath: /export_bin:shared
            - name: cores
              mountPath: /var/cores
            - name: kubelet
              mountPath: /var/lib/kubelet:shared
            - name: src
              mountPath: /usr/src
            - name: dockerplugins
              mountPath: /run/docker/plugins
      initContainers:
        - name: px-init
          image: harshpx/monitor
          securityContext:
            privileged: true
          volumeMounts:
            - name: hostproc
              mountPath: /media/host/proc
      restartPolicy: Always
      tolerations:
      - key: node-role.kubernetes.io/master
        effect: NoSchedule
      serviceAccountName: px-account
      volumes:
        - name: libosd
          hostPath:
            path: /var/lib/osd
        - name: dev
          hostPath:
            path: /dev
        - name: etcpwx
          hostPath:
            path: /etc/pwx
        - name: optpwx
          hostPath:
            path: /opt/pwx/bin
        - name: cores
          hostPath:
            path: /var/cores
        - name: kubelet
          hostPath:
            path: /var/lib/kubelet
        - name: src
          hostPath:
            path: /usr/src
        - name: dockerplugins
          hostPath:
            path: /run/docker/plugins
        - name: dockersock
          hostPath:
            path: /var/run/docker.sock
        - name: hostproc
          hostPath:
            path: /proc