---
kind: ClusterRoleBinding
apiVersion: rbac.authorization.k8s.io/v1beta1
metadata:
  name: portworx
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: portworx
subjects:
- kind: ServiceAccount
  name: portworx
  namespace: kube-system
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
          image: portworx/monitor
          command:
            - /portworx-mon
          args:
            ["-k", "{{.etcd}}", "-c", "{{.cluster}}", "-a", "-f"]
          livenessProbe:
            initialDelaySeconds: 30
            httpGet:
              host: 127.0.0.1
              path: /status
              port: 9001
          securityContext:
            privileged: true
          volumeMounts:
            - name: varrun
              mountPath: /var/run
          resources:
            requests:
              cpu: 10m
      restartPolicy: Always
      volumes:
      - name: varrun
        hostPath:
          path: /var/run
