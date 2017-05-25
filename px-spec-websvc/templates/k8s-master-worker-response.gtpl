apiVersion: extensions/v1beta1
kind: DaemonSet
metadata:
  name: portworx-zerostorage
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
              - key: node-role.kubernetes.io/master
                operator: Exists
      hostNetwork: true
      hostPID: true
      containers:
        - name: portworx
          image: portworx/px-enterprise:latest
          imagePullPolicy: Always
          args:
             ["{{if .Kvdb}}-k {{.Kvdb}}{{end}}",
              "{{if .Cluster}}-c {{.Cluster}}{{end}}",
              "{{if .DIface}}-d {{.DIface}}{{end}}",
              "{{if .MIface}}-m {{.MIface}}{{end}}",
              "{{if .EtcdPasswd}}-userpwd {{.EtcdPasswd}}{{end}}",
              "{{if .EtcdCa}}-ca {{.EtcdCa}}{{end}}",
              "{{if .EtcdCert}}-cert {{.EtcdCert}}{{end}}",
              "{{if .EtcdKey}}-key {{.EtcdKey}}{{end}}",
              "{{if .Acltoken}}-acltoken {{.Acltoken}}{{end}}",
              "{{if .Token}}-t {{.Token}}{{end}}",
              "{{if .Env}}{{.Env}}{{end}}",
              "-x", "kubernetes",
              "-z"]
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
              mountPath: {{if .Coreos}}/lib/modules{{else}}/usr/src{{end}}
            - name: dockerplugins
              mountPath: /run/docker/plugins
      initContainers:
        - name: px-init
          image: harshpx/px-init
          securityContext:
            privileged: true
          volumeMounts:
            - name: hostproc
              mountPath: /media/host/proc
      restartPolicy: Always
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
            path: {{if .Coreos}}/lib/modules{{else}}/usr/src{{end}}
        - name: dockerplugins
          hostPath:
            path: /run/docker/plugins
        - name: dockersock
          hostPath:
            path: /var/run/docker.sock
        - name: hostproc
          hostPath:
            path: /proc
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
        app: portworx
        name: portworx
      namespace: kube-system
    spec:
      hostNetwork: true
      hostPID: true
      containers:
        - name: portworx
          image: portworx/px-enterprise:latest
          imagePullPolicy: Always
          args:
             ["{{if .Kvdb}}-k {{.Kvdb}}{{end}}",
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
              mountPath: {{if .Coreos}}/lib/modules{{else}}/usr/src{{end}}
            - name: dockerplugins
              mountPath: /run/docker/plugins
      initContainers:
        - name: px-init
          image: harshpx/px-init
          securityContext:
            privileged: true
          volumeMounts:
            - name: hostproc
              mountPath: /media/host/proc
      restartPolicy: Always
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
            path: {{if .Coreos}}/lib/modules{{else}}/usr/src{{end}}
        - name: dockerplugins
          hostPath:
            path: /run/docker/plugins
        - name: dockersock
          hostPath:
            path: /var/run/docker.sock
        - name: hostproc
          hostPath:
            path: /proc
