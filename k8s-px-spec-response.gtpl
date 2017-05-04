kind: ClusterRole
apiVersion: rbac.authorization.k8s.io/v1beta1
metadata:
   name: node-list-role
rules:
- apiGroups: [""]
  resources: ["nodes"]
  verbs: ["list"]

---

kind: ClusterRoleBinding
apiVersion: rbac.authorization.k8s.io/v1beta1
metadata:
  name: node-list-binding
subjects:
- kind: ServiceAccount
  name: persistent-volume-binder
  namespace: kube-system
roleRef:
  kind: ClusterRole
  name: node-list-role
  apiGroup: rbac.authorization.k8s.io

---

apiVersion: extensions/v1beta1
kind: DaemonSet
metadata:
  name: portworx
  namespace: kube-system
spec:
  template:
    metadata:
      labels:
        name: portworx
    spec:
      hostNetwork: true
      hostPID: true
      containers:
        - name: portworx
          image: harshpx/monitor
          command:
            - /px-mon
          args:
             ["{{if .Etcd}}-k {{.Etcd}}{{end}}",
              "{{if .Cluster}}-c {{.Cluster}}{{end}}",
              "{{if .DIface}}-d {{.DIface}}{{end}}",
              "{{if .MIface}}-m {{.MIface}}{{end}}",
              "{{if .Drive}}-s {{.Drive}}{{end}}", "-a", "-f", "-x", "kubernetes"]
          livenessProbe:
            initialDelaySeconds: 600
            httpGet:
              host: 127.0.0.1
              path: /status
              port: 9001
          securityContext:
            privileged: true
          volumeMounts:
            - name: hostroot
              mountPath: /media/host
            - name: varrun
              mountPath: /var/run
          resources:
            requests:
              cpu: 10m
      restartPolicy: Always
      volumes:
        - name: hostroot
          hostPath:
            path: /
        - name: varrun
          hostPath:
            path: /var/run
