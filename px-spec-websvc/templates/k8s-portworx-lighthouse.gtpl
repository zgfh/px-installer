apiVersion: v1
kind: Service
metadata:
  creationTimestamp: null
  labels:
    service: influx-px
  name: influx-px
spec:
  ports:
  - name: "8083"
    port: 8083
    targetPort: 8083
  - name: "8086"
    port: 8086
    targetPort: 8086
  selector:
    service: influx-px
---
apiVersion: v1
kind: Service
metadata:
  name: px-lighthouse
  labels:
    service: px-lighthouse
spec:
  type: NodePort
  ports:
    - port: 80
      nodePort: 30062
  selector:
    io.kompose.service: px-lighthouse
---
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  creationTimestamp: null
  labels:
    service: influx-px
  name: influx-px
spec:
  replicas: 1
  strategy:
    type: Recreate
  template:
    metadata:
      creationTimestamp: null
      labels:
        service: influx-px
    spec:
      containers:
      - env:
        - name: MYSQL_ALLOW_EMPTY_PASSWORD
          value: "true"
        - name: ADMIN_USER
          value: '"admin"'
        - name: INFLUXDB_INIT_PWD
          value: '"password"'
        - name: PRE_CREATE_DB
          value: '"px_stats"'
        image: tutum/influxdb
        name: influx-px
        ports:
        - containerPort: 8083
        - containerPort: 8086
        resources: {}
        volumeMounts:
        - mountPath: /data
          name: influx-px-claim0
      restartPolicy: Always
      volumes:
      - name: influx-px-claim0
        emptyDir: {}
---
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  creationTimestamp: null
  labels:
    service: px-lighthouse
  name: px-lighthouse
spec:
  replicas: 1
  strategy:
    type: Recreate
  template:
    metadata:
      creationTimestamp: null
      labels:
        io.kompose.service: px-lighthouse
    spec:
      containers:
      - command:
        - /bin/bash
        - /lighthouse/on-prem-entrypoint.sh
        - -k
        {{if .Kvdb}}- {{.Kvdb}}{{end}}
        - -d
        - http://admin:password@influx-px:8086
        env:
        - name: PWX_INFLUXDB
          value: '"http://influx-px:8086"'
        - name: PWX_INFLUXUSR
          value: '"admin"'
        - name: PWX_INFLUXPW
          value: '"password"'
        - name: PWX_PX_PRECREATE_ADMIN
          value: "true"
        {{if .EtcdPasswd}}{{.EtcdPasswd}}{{end}}
        {{if .EtcdCa}}{{.EtcdCa}}{{end}}
        {{if .EtcdCert}}{{.EtcdCert}}{{end}}
        {{if .EtcdKey}}{{.EtcdKey}}{{end}}
        {{if .AdminEmail}}{{.AdminEmail}}{{end}}
        {{if .Company}}{{.Company}}{{end}}
        {{if .EtcdAuth}}{{.EtcdAuth}}{{end}}
        image: {{.LighthouseImage}}
        name: px-lighthouse
        ports:
        - containerPort: 80
        resources: {}
        volumeMounts:
        - mountPath: /var/log
          name: px-lighthouse-claim0
      restartPolicy: Always
      volumes:
      - name: px-lighthouse-claim0
        emptyDir: {}