# SOURCE: {{.Origin}}
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
  verbs: ["get", "list"]
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
        name: portworx
    spec:
      affinity:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
            - matchExpressions:
              - key: px/enabled
                operator: NotIn
                values:
                - "false"
              {{- if .MasterLess}}
              {{if .Openshift}}
              - key: openshift-infra
                operator: DoesNotExist
              {{else}}
              - key: node-role.kubernetes.io/master
                operator: DoesNotExist
              {{- end}}
              {{- end}}
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
      {{- if not .MasterLess}}
      tolerations:
      - key: node-role.kubernetes.io/master
        effect: NoSchedule
      {{- end}}
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
