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
          imagePullPolicy: Always
          command:
            - /px-mon
          args:
             ["{{if .Etcd}}-k {{.Etcd}}{{end}}",
              "{{if .Cluster}}-c {{.Cluster}}{{end}}",
              "{{if .DIface}}-d {{.DIface}}{{end}}",
              "{{if .MIface}}-m {{.MIface}}{{end}}",
              "{{if .Drive}}-s {{.Drive}}{{end}}", "-a", "-f", "-x", "kubernetes"]
          livenessProbe:
            initialDelaySeconds: 840
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
