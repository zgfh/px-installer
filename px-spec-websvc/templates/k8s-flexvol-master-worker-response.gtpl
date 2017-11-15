apiVersion: v1
kind: ServiceAccount
metadata:
  name: px-account
  namespace: kube-system
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
          image: {{.PxImage}}
          terminationMessagePath: "/tmp/px-termination-log"
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
              mountPath: {{if .Openshift}}/var/lib/origin/openshift.local.volumes:shared{{else}}/var/lib/kubelet:shared{{end}}
            - name: src
              mountPath: {{if .Coreos}}/lib/modules{{else}}/usr/src{{end}}
            - name: dockerplugins
              mountPath: /run/docker/plugins
            - name: flexvol
              mountPath: /export_flexvolume:shared
      restartPolicy: Always
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
            path: {{if .Openshift}}/var/lib/origin/openshift.local.volumes{{else}}/var/lib/kubelet{{end}}
        - name: src
          hostPath:
            path: {{if .Coreos}}/lib/modules{{else}}/usr/src{{end}}
        - name: dockerplugins
          hostPath:
            path: /run/docker/plugins
        - name: dockersock
          hostPath:
            path: /var/run/docker.sock
        - name: flexvol
          hostPath:
            path: /usr/libexec/kubernetes/kubelet-plugins/volume/exec/px~flexvolume

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
      affinity:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
            - matchExpressions:
              - key: node-role.kubernetes.io/master
                operator: DoesNotExist
      hostNetwork: true
      hostPID: true
      containers:
        - name: portworx
          image: {{.PxImage}}
          terminationMessagePath: "/tmp/px-termination-log"
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
              mountPath: {{if .Openshift}}/var/lib/origin/openshift.local.volumes:shared{{else}}/var/lib/kubelet:shared{{end}}
            - name: src
              mountPath: {{if .Coreos}}/lib/modules{{else}}/usr/src{{end}}
            - name: dockerplugins
              mountPath: /run/docker/plugins
            - name: flexvol
              mountPath: /export_flexvolume:shared
      restartPolicy: Always
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
            path: {{if .Openshift}}/var/lib/origin/openshift.local.volumes{{else}}/var/lib/kubelet{{end}}
        - name: src
          hostPath:
            path: {{if .Coreos}}/lib/modules{{else}}/usr/src{{end}}
        - name: dockerplugins
          hostPath:
            path: /run/docker/plugins
        - name: dockersock
          hostPath:
            path: /var/run/docker.sock
        - name: flexvol
          hostPath:
            path: /usr/libexec/kubernetes/kubelet-plugins/volume/exec/px~flexvolume
