# SOURCE: {{.Origin}}
{{- if .StartStork}}
apiVersion: v1
kind: ConfigMap
metadata:
  name: stork-config
  namespace: kube-system
data:
  policy.cfg: |-
    {
      "kind": "Policy",
      "apiVersion": "v1",
      "predicates": [
{{- if lt .KubeVer "1.9.z"}}
        {"name": "NoVolumeNodeConflict"},
{{- end}}
        {"name": "MaxAzureDiskVolumeCount"},
        {"name": "NoVolumeZoneConflict"},
        {"name": "PodToleratesNodeTaints"},
        {"name": "CheckNodeMemoryPressure"},
        {"name": "MaxEBSVolumeCount"},
        {"name": "MaxGCEPDVolumeCount"},
        {"name": "MatchInterPodAffinity"},
        {"name": "NoDiskConflict"},
        {"name": "GeneralPredicates"},
        {"name": "CheckNodeDiskPressure"}
      ],
      "priorities": [
        {"name": "NodeAffinityPriority", "weight": 1},
        {"name": "TaintTolerationPriority", "weight": 1},
        {"name": "SelectorSpreadPriority", "weight": 1},
        {"name": "InterPodAffinityPriority", "weight": 1},
        {"name": "LeastRequestedPriority", "weight": 1},
        {"name": "BalancedResourceAllocation", "weight": 1},
        {"name": "NodePreferAvoidPodsPriority", "weight": 1}
      ],
      "extenders": [
        {
          "urlPrefix": "http://stork-service.kube-system.svc.cluster.local:8099",
          "apiVersion": "v1beta1",
          "filterVerb": "filter",
          "prioritizeVerb": "prioritize",
          "weight": 5,
          "enableHttps": false,
          "nodeCacheCapable": false
        }
      ]
    }
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: stork-account
  namespace: kube-system
---
kind: ClusterRole
apiVersion: rbac.authorization.k8s.io/{{.RbacAuthVer}}
metadata:
   name: stork-role
rules:
  - apiGroups: [""]
    resources: ["pods"]
    verbs: ["get", "list", "delete"]
  - apiGroups: [""]
    resources: ["persistentvolumes"]
    verbs: ["get", "list", "watch", "create", "delete"]
  - apiGroups: [""]
    resources: ["persistentvolumeclaims"]
    verbs: ["get", "list", "watch", "update"]
  - apiGroups: ["storage.k8s.io"]
    resources: ["storageclasses"]
    verbs: ["get", "list", "watch"]
  - apiGroups: [""]
    resources: ["events"]
    verbs: ["list", "watch", "create", "update", "patch"]
  - apiGroups: ["apiextensions.k8s.io"]
    resources: ["customresourcedefinitions"]
    verbs: ["create", "list", "watch", "delete"]
  - apiGroups: ["volumesnapshot.external-storage.k8s.io"]
    resources: ["volumesnapshots"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
  - apiGroups: ["volumesnapshot.external-storage.k8s.io"]
    resources: ["volumesnapshotdatas"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
  - apiGroups: [""]
    resources: ["configmaps"]
    verbs: ["get", "create", "update"]
  - apiGroups: [""]
    resources: ["services"]
    verbs: ["get"]
---
kind: ClusterRoleBinding
apiVersion: rbac.authorization.k8s.io/{{.RbacAuthVer}}
metadata:
  name: stork-role-binding
subjects:
- kind: ServiceAccount
  name: stork-account
  namespace: kube-system
{{- if lt .KubeVer "1.6.z"}}
  apiVersion: v1
{{- end}}
roleRef:
  kind: ClusterRole
  name: stork-role
  apiGroup: rbac.authorization.k8s.io
---
kind: Service
apiVersion: v1
metadata:
  name: stork-service
  namespace: kube-system
spec:
  selector:
    name: stork
  ports:
    - protocol: TCP
      port: 8099
      targetPort: 8099
---
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  annotations:
    scheduler.alpha.kubernetes.io/critical-pod: ""
  labels:
    tier: control-plane
  name: stork
  namespace: kube-system
spec:
  strategy:
    rollingUpdate:
      maxSurge: 1
      maxUnavailable: 1
    type: RollingUpdate
  replicas: 3
  template:
    metadata:
      annotations:
        scheduler.alpha.kubernetes.io/critical-pod: ""
      labels:
        name: stork
        tier: control-plane
    spec:
      containers:
      - command:
        - /stork
        - --driver=pxd
        - --verbose
        - --leader-elect=true
        imagePullPolicy: Always
        image: openstorage/stork:1.0.1
        resources:
          requests:
            cpu: '0.1'
        name: stork
      hostPID: false
      affinity:
        podAntiAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            - labelSelector:
                matchExpressions:
                  - key: "name"
                    operator: In
                    values:
                    - stork
              topologyKey: "kubernetes.io/hostname"
      serviceAccountName: stork-account
---
kind: StorageClass
apiVersion: storage.k8s.io/v1
metadata:
  name: stork-snapshot-sc
provisioner: stork-snapshot
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: stork-scheduler-account
  namespace: kube-system
---
kind: ClusterRole
apiVersion: rbac.authorization.k8s.io/{{.RbacAuthVer}}
metadata:
  name: stork-scheduler-role
rules:
  - apiGroups: [""]
    resources: ["endpoints"]
    verbs: ["get", "update"]
  - apiGroups: [""]
    resources: ["configmaps"]
    verbs: ["get"]
  - apiGroups: [""]
    resources: ["events"]
    verbs: ["create", "patch", "update"]
  - apiGroups: [""]
    resources: ["endpoints"]
    verbs: ["create"]
  - apiGroups: [""]
    resourceNames: ["kube-scheduler"]
    resources: ["endpoints"]
    verbs: ["delete", "get", "patch", "update"]
  - apiGroups: [""]
    resources: ["nodes"]
    verbs: ["get", "list", "watch"]
  - apiGroups: [""]
    resources: ["pods"]
    verbs: ["delete", "get", "list", "watch"]
  - apiGroups: [""]
    resources: ["bindings", "pods/binding"]
    verbs: ["create"]
  - apiGroups: [""]
    resources: ["pods/status"]
    verbs: ["patch", "update"]
  - apiGroups: [""]
    resources: ["replicationcontrollers", "services"]
    verbs: ["get", "list", "watch"]
  - apiGroups: ["app", "extensions"]
    resources: ["replicasets"]
    verbs: ["get", "list", "watch"]
  - apiGroups: ["apps"]
    resources: ["statefulsets"]
    verbs: ["get", "list", "watch"]
  - apiGroups: ["policy"]
    resources: ["poddisruptionbudgets"]
    verbs: ["get", "list", "watch"]
  - apiGroups: [""]
    resources: ["persistentvolumeclaims", "persistentvolumes"]
    verbs: ["get", "list", "watch"]
---
kind: ClusterRoleBinding
apiVersion: rbac.authorization.k8s.io/{{.RbacAuthVer}}
metadata:
  name: stork-scheduler-role-binding
subjects:
- kind: ServiceAccount
  name: stork-scheduler-account
  namespace: kube-system
{{- if lt .KubeVer "1.6.z"}}
  apiVersion: v1
{{- end}}
roleRef:
  kind: ClusterRole
  name: stork-scheduler-role
  apiGroup: rbac.authorization.k8s.io
---
apiVersion: apps/v1beta1
kind: Deployment
metadata:
  labels:
    component: scheduler
    tier: control-plane
  name: stork-scheduler
  namespace: kube-system
spec:
  replicas: 3
  template:
    metadata:
      labels:
        component: scheduler
        tier: control-plane
      name: stork-scheduler
    spec:
      containers:
      - command:
        - /usr/local/bin/kube-scheduler
        - --address=0.0.0.0
        - --leader-elect=true
        - --scheduler-name=stork
        - --policy-configmap=stork-config
        - --policy-configmap-namespace=kube-system
        - --lock-object-name=stork-scheduler
        image: gcr.io/google_containers/kube-scheduler-amd64:v{{- if .KubeVer}}{{.KubeVer}}{{- else}}1.7.8{{- end}}
        livenessProbe:
          httpGet:
            path: /healthz
            port: 10251
          initialDelaySeconds: 15
        name: stork-scheduler
        readinessProbe:
          httpGet:
            path: /healthz
            port: 10251
        resources:
          requests:
            cpu: '0.1'
      affinity:
        podAntiAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            - labelSelector:
                matchExpressions:
                  - key: "name"
                    operator: In
                    values:
                    - stork-scheduler
              topologyKey: "kubernetes.io/hostname"
      hostPID: false
      serviceAccountName: stork-scheduler-account
---
{{- end}}{{/* <--------------------------------------- END .StartStork */}}
{{- if .NeedController}}
apiVersion: v1
kind: ServiceAccount
metadata:
  name: portworx-pvc-controller-account
  namespace: kube-system
---
kind: ClusterRole
apiVersion: rbac.authorization.k8s.io/{{.RbacAuthVer}}
metadata:
   name: portworx-pvc-controller-role
rules:
- apiGroups: [""]
  resources: ["persistentvolumes"]
  verbs: ["create","delete","get","list","update","watch"]
- apiGroups: [""]
  resources: ["persistentvolumes/status"]
  verbs: ["update"]
- apiGroups: [""]
  resources: ["persistentvolumeclaims"]
  verbs: ["get", "list", "update", "watch"]
- apiGroups: [""]
  resources: ["persistentvolumeclaims/status"]
  verbs: ["update"]
- apiGroups: [""]
  resources: ["pods"]
  verbs: ["create", "delete", "get", "list", "watch"]
- apiGroups: ["storage.k8s.io"]
  resources: ["storageclasses"]
  verbs: ["get", "list", "watch"]
- apiGroups: [""]
  resources: ["endpoints", "services"]
  verbs: ["create", "delete", "get"]
- apiGroups: [""]
  resources: ["secrets"]
  verbs: ["get", "list"]
- apiGroups: [""]
  resources: ["nodes"]
  verbs: ["get", "list"]
- apiGroups: [""]
  resources: ["events"]
  verbs: ["watch"]
- apiGroups: [""]
  resources: ["events"]
  verbs: ["create", "patch", "update"]
- apiGroups: [""]
  resources: ["serviceaccounts"]
  verbs: ["get"]
- apiGroups: [""]
  resources: ["configmaps"]
  verbs: ["get", "create", "update"]
---
kind: ClusterRoleBinding
apiVersion: rbac.authorization.k8s.io/{{.RbacAuthVer}}
metadata:
  name: portworx-pvc-controller-role-binding
subjects:
- kind: ServiceAccount
  name: portworx-pvc-controller-account
  namespace: kube-system
{{- if lt .KubeVer "1.6.z"}}
  apiVersion: v1
{{- end}}
roleRef:
  kind: ClusterRole
  name: portworx-pvc-controller-role
  apiGroup: rbac.authorization.k8s.io
---
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  annotations:
    scheduler.alpha.kubernetes.io/critical-pod: ""
  labels:
    tier: control-plane
  name: portworx-pvc-controller
  namespace: kube-system
spec:
  replicas: 3
  strategy:
    rollingUpdate:
      maxSurge: 1
      maxUnavailable: 1
    type: RollingUpdate
  template:
    metadata:
      annotations:
        scheduler.alpha.kubernetes.io/critical-pod: ""
      labels:
        name: portworx-pvc-controller
        tier: control-plane
    spec:
      containers:
      - command:
        - kube-controller-manager
        - --leader-elect=true
        - --address=0.0.0.0
        - --controllers=persistentvolume-binder
        - --use-service-account-credentials=true
        - --leader-elect-resource-lock=configmaps
        image: gcr.io/google_containers/kube-controller-manager-amd64:v{{- if .KubeVer}}{{.KubeVer}}{{- else}}1.7.8{{- end}}
        livenessProbe:
          failureThreshold: 8
          httpGet:
            host: 127.0.0.1
            path: /healthz
            port: 10252
            scheme: HTTP
          initialDelaySeconds: 15
          timeoutSeconds: 15
        name: portworx-pvc-controller-manager
        resources:
          requests:
            cpu: 200m
      hostNetwork: true
      affinity:
        podAntiAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            - labelSelector:
                matchExpressions:
                  - key: "name"
                    operator: In
                    values:
                    - portworx-pvc-controller
              topologyKey: "kubernetes.io/hostname"
      serviceAccountName: portworx-pvc-controller-account
---
{{- end}}{{/* <--------------------------------------- END .NeedController */}}
apiVersion: v1
kind: ServiceAccount
metadata:
  name: px-account
  namespace: kube-system
---
kind: ClusterRole
apiVersion: rbac.authorization.k8s.io/{{.RbacAuthVer}}
metadata:
   name: node-get-put-list-role
rules:
- apiGroups: [""]
  resources: ["nodes"]
  verbs: ["watch", "get", "update", "list"]
- apiGroups: [""]
  resources: ["pods"]
  verbs: ["delete", "get", "list"]
- apiGroups: [""]
  resources: ["persistentvolumeclaims"]
  verbs: ["get", "list"]
---
kind: ClusterRoleBinding
apiVersion: rbac.authorization.k8s.io/{{.RbacAuthVer}}
metadata:
  name: node-role-binding
subjects:
- kind: ServiceAccount
  name: px-account
  namespace: kube-system
{{- if lt .KubeVer "1.6.z"}}
  apiVersion: v1
{{- end}}
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
---
apiVersion: extensions/v1beta1
kind: DaemonSet
metadata:
  name: portworx-zerostorage
  namespace: kube-system
spec:
  minReadySeconds: 0
  updateStrategy:
    {{- if .IsRunC}}
    type: RollingUpdate
    rollingUpdate:
      maxUnavailable: 1
    {{- else}}
    type: OnDelete
    {{- end}}
  template:
    metadata:
      labels:
        app:  portworx-zerostorage
        name: portworx
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
              {{- if .Openshift}}
              - key: openshift-infra
                operator: Exists
              {{- else}}
              - key: node-role.kubernetes.io/master
                operator: Exists
              {{- end}}
              - key: px/enabled
                operator: NotIn
                values:
                - "false"
      hostNetwork: true
      hostPID: true
      containers:
        - name: portworx
          image: {{.PxImage}}
          terminationMessagePath: "/tmp/px-termination-log"
          imagePullPolicy: Always
          args:
            [{{- if .Kvdb}}"-k", "{{.Kvdb}}", {{end}}
             {{- if .Cluster}}"-c", "{{.Cluster}}", {{end}}
             {{- if .DIface}}"-d", "{{.DIface}}", {{end}}
             {{- if .MIface}}"-m", "{{.MIface}}", {{end}}
             {{- if .SecretType}}"-secret_type", "{{.SecretType}}", {{end}}
             {{- if .EtcdPasswd}}"-userpwd", "{{.EtcdPasswd}}", {{end}}
             {{- if .EtcdCa}}"-ca", "{{.EtcdCa}}", {{end}}
             {{- if .EtcdCert}}"-cert", "{{.EtcdCert}}", {{end}}
             {{- if .EtcdKey}}"-key", "{{.EtcdKey}}", {{end}}
             {{- if .Acltoken}}"-acltoken", "{{.Acltoken}}", {{end}}
             {{- if .Token}}"-t", "{{.Token}}",{{end}}
             {{- if .JournalDev}}"-j", "{{.JournalDev}}",{{end}}
             "-x", "kubernetes", "-z"]
          env:
            - name: "PX_TEMPLATE_VERSION"
              value: "{{.TmplVer}}"
            {{if .Env}}{{.Env}}{{end}}
          livenessProbe:
            periodSeconds: 30
            initialDelaySeconds: 840 # allow image pull in slow networks
            httpGet:
              host: 127.0.0.1
              path: /status
              port: 9001
          readinessProbe:
            periodSeconds: 10
            httpGet:
              host: 127.0.0.1
            {{- if .IsRunC}}
              path: /health
              port: 9015
            {{- else}}
              path: /v1/cluster/nodehealth
              port: 9001
            {{- end}}
          securityContext:
            privileged: true
          volumeMounts:
            - name: dockersock
              mountPath: /var/run/docker.sock
            - name: kubelet
              mountPath: {{if .Openshift}}/var/lib/origin/openshift.local.volumes:shared{{else}}/var/lib/kubelet:shared{{end}}
            - name: libosd
              mountPath: /var/lib/osd:shared
            - name: etcpwx
              mountPath: /etc/pwx
            {{- if .IsRunC}}
            - name: optpwx
              mountPath: /opt/pwx
            - name: proc1nsmount
              mountPath: {{if .Openshift}}/host_proc{{else}}/host_proc/1/ns{{end}}
            - name: sysdmount
              mountPath: /etc/systemd/system
            {{- else}}
            - name: dev
              mountPath: /dev
            - name: optpwx
              mountPath: /export_bin
            - name: cores
              mountPath: /var/cores
            - name: src
              mountPath: {{if .Coreos}}/lib/modules{{else}}/usr/src{{end}}
            - name: dockerplugins
              mountPath: /run/docker/plugins
            - name: hostproc
              mountPath: /hostproc
            {{- end}}
      restartPolicy: Always
      serviceAccountName: px-account
      volumes:
        - name: dockersock
          hostPath:
            path: /var/run/docker.sock
        - name: kubelet
          hostPath:
            path: {{if .Openshift}}/var/lib/origin/openshift.local.volumes{{else}}/var/lib/kubelet{{end}}
        - name: libosd
          hostPath:
            path: /var/lib/osd
        - name: etcpwx
          hostPath:
            path: /etc/pwx
        {{- if .IsRunC}}
        - name: optpwx
          hostPath:
            path: /opt/pwx
        - name: proc1nsmount
          hostPath:
            path: {{if .Openshift}}/proc{{else}}/proc/1/ns{{end}}
        - name: sysdmount
          hostPath:
            path: /etc/systemd/system
        {{- else}}
        - name: dev
          hostPath:
            path: /dev
        - name: optpwx
          hostPath:
            path: /opt/pwx/bin
        - name: cores
          hostPath:
            path: /var/cores
        - name: src
          hostPath:
            path: {{if .Coreos}}/lib/modules{{else}}/usr/src{{end}}
        - name: dockerplugins
          hostPath:
            path: /run/docker/plugins
        - name: hostproc
          hostPath:
            path: /proc
        {{- end}}
---
apiVersion: extensions/v1beta1
kind: DaemonSet
metadata:
  name: portworx
  namespace: kube-system
spec:
  minReadySeconds: 0
  updateStrategy:
    {{- if .IsRunC}}
    type: RollingUpdate
    rollingUpdate:
      maxUnavailable: 1
    {{- else}}
    type: OnDelete
    {{- end}}
  template:
    metadata:
      labels:
        app: portworx
        name: portworx
      namespace: kube-system
    spec:
      affinity:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
            - matchExpressions:
              {{- if .Openshift}}
              - key: openshift-infra
                operator: DoesNotExist
              {{- else}}
              - key: node-role.kubernetes.io/master
                operator: DoesNotExist
              {{- end}}
              - key: px/enabled
                operator: NotIn
                values:
                - "false"
      hostNetwork: true
      hostPID: true
      containers:
        - name: portworx
          image: {{.PxImage}}
          terminationMessagePath: "/tmp/px-termination-log"
          imagePullPolicy: Always
          args:
            [{{- if .Kvdb}}"-k", "{{.Kvdb}}", {{end}}
             {{- if .Cluster}}"-c", "{{.Cluster}}", {{end}}
             {{- if .DIface}}"-d", "{{.DIface}}", {{end}}
             {{- if .MIface}}"-m", "{{.MIface}}", {{end}}
             {{- if .Drives}}{{.Drives}}, {{end}}
             {{- if .SecretType}}"-secret_type", "{{.SecretType}}", {{end}}
             {{- if .EtcdPasswd}}"-userpwd", "{{.EtcdPasswd}}", {{end}}
             {{- if .EtcdCa}}"-ca", "{{.EtcdCa}}", {{end}}
             {{- if .EtcdCert}}"-cert", "{{.EtcdCert}}", {{end}}
             {{- if .EtcdKey}}"-key", "{{.EtcdKey}}", {{end}}
             {{- if .Acltoken}}"-acltoken", "{{.Acltoken}}", {{end}}
             {{- if .Token}}"-t", "{{.Token}}",{{end}}
             {{- if .JournalDev}}"-j", "{{.JournalDev}}",{{end}}
             "-x", "kubernetes"]
          env:
            - name: "PX_TEMPLATE_VERSION"
              value: "{{.TmplVer}}"
            {{if .Env}}{{.Env}}{{end}}
          livenessProbe:
            periodSeconds: 30
            initialDelaySeconds: 840 # allow image pull in slow networks
            httpGet:
              host: 127.0.0.1
              path: /status
              port: 9001
          readinessProbe:
            periodSeconds: 10
            httpGet:
              host: 127.0.0.1
            {{- if .IsRunC}}
              path: /health
              port: 9015
            {{- else}}
              path: /v1/cluster/nodehealth
              port: 9001
            {{- end}}
          securityContext:
            privileged: true
          volumeMounts:
            - name: dockersock
              mountPath: /var/run/docker.sock
            - name: kubelet
              mountPath: {{if .Openshift}}/var/lib/origin/openshift.local.volumes:shared{{else}}/var/lib/kubelet:shared{{end}}
            - name: libosd
              mountPath: /var/lib/osd:shared
            - name: etcpwx
              mountPath: /etc/pwx
            {{- if .IsRunC}}
            - name: optpwx
              mountPath: /opt/pwx
            - name: proc1nsmount
              mountPath: {{if .Openshift}}/host_proc{{else}}/host_proc/1/ns{{end}}
            - name: sysdmount
              mountPath: /etc/systemd/system
            {{- else}}
            - name: dev
              mountPath: /dev
            - name: optpwx
              mountPath: /export_bin
            - name: cores
              mountPath: /var/cores
            - name: src
              mountPath: {{if .Coreos}}/lib/modules{{else}}/usr/src{{end}}
            - name: dockerplugins
              mountPath: /run/docker/plugins
            - name: hostproc
              mountPath: /hostproc
            {{- end}}
      restartPolicy: Always
      serviceAccountName: px-account
      volumes:
        - name: dockersock
          hostPath:
            path: /var/run/docker.sock
        - name: kubelet
          hostPath:
            path: {{if .Openshift}}/var/lib/origin/openshift.local.volumes{{else}}/var/lib/kubelet{{end}}
        - name: libosd
          hostPath:
            path: /var/lib/osd
        - name: etcpwx
          hostPath:
            path: /etc/pwx
        {{- if .IsRunC}}
        - name: optpwx
          hostPath:
            path: /opt/pwx
        - name: proc1nsmount
          hostPath:
            path: {{if .Openshift}}/proc{{else}}/proc/1/ns{{end}}
        - name: sysdmount
          hostPath:
            path: /etc/systemd/system
        {{- else}}
        - name: dev
          hostPath:
            path: /dev
        - name: optpwx
          hostPath:
            path: /opt/pwx/bin
        - name: cores
          hostPath:
            path: /var/cores
        - name: src
          hostPath:
            path: {{if .Coreos}}/lib/modules{{else}}/usr/src{{end}}
        - name: dockerplugins
          hostPath:
            path: /run/docker/plugins
        - name: hostproc
          hostPath:
            path: /proc
        {{- end}}
