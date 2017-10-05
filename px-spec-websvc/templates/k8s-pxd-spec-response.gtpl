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
  verbs: ["watch", "get", "update", "list"]
- apiGroups: [""]
  resources: ["pods"]
  verbs: ["get", "list"]
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
---
apiVersion: extensions/v1beta1
kind: DaemonSet
metadata:
  name: portworx
  namespace: kube-system
spec:
  minReadySeconds: 0
  updateStrategy:
    type: {{if .IsRunC}}RollingUpdate{{else}}OnDelete{{end}}
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
              {{if .MasterLess}}
              - key: node-role.kubernetes.io/master
                operator: DoesNotExist
              {{end}}
      hostNetwork: true
      hostPID: true
      containers:
        - name: portworx
          image: {{.PxImage}}
          terminationMessagePath: "/tmp/px-termination-log"
          imagePullPolicy: Always
          args:
            [{{if .IsRunC}}"install", {{end}}
             {{- if .Kvdb}}"-k", "{{.Kvdb}}", {{end}}
             {{- if .Cluster}}"-c", "{{.Cluster}}", {{end}}
             {{- if .DIface}}"-d", "{{.DIface}}", {{end}}
             {{- if .MIface}}"-m", "{{.MIface}}", {{end}}
             {{- if .Drives}}{{.Drives}}{{end}},
             {{- if .EtcdPasswd}}"-userpwd", "{{.EtcdPasswd}}", {{end}}
             {{- if .EtcdCa}}"-ca", "{{.EtcdCa}}", {{end}}
             {{- if .EtcdCert}}"-cert", "{{.EtcdCert}}", {{end}}
             {{- if .EtcdKey}}"-key", "{{.EtcdKey}}", {{end}}
             {{- if .Acltoken}}"-acltoken", "{{.Acltoken}}", {{end}}
             {{- if .Token}}"-t", "{{.Token}}",{{end}}
             "-x", "kubernetes"]
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
              path: /status
              port: 9001
          securityContext:
            privileged: true
          volumeMounts:
            - name: dockersock
              mountPath: /var/run/docker.sock
            - name: kubelet
              mountPath: /var/lib/kubelet:shared
            - name: libosd
              mountPath: /var/lib/osd:shared
            {{- if .IsRunC}}
            - name: proc1nsmount
              mountPath: /host_proc/1/ns
            {{- else}}
            - name: dev
              mountPath: /dev
            - name: etcpwx
              mountPath: /etc/pwx/
            - name: optpwx
              mountPath: /export_bin:shared
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
      {{if .MasterLess}}{{else}}tolerations:
      - key: node-role.kubernetes.io/master
        effect: NoSchedule{{end}}
      serviceAccountName: px-account
      volumes:
        - name: dockersock
          hostPath:
            path: /var/run/docker.sock
        - name: kubelet
          hostPath:
            path: /var/lib/kubelet
        - name: libosd
          hostPath:
            path: /var/lib/osd
        {{- if .IsRunC}}
        - name: proc1nsmount
          hostPath:
            path: /proc/1/ns
        {{- else}}
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
