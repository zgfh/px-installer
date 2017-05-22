version: '3'

services:
  px-monitor:
    image: harshpx/monitor2
    volumes:
      - /:/media/host
      - /var/run/docker.sock:/var/run/docker.sock
    deploy:
      mode: global
      update_config:
        parallelism: 1
        delay: 10s
        failure_action: continue
        monitor: 60s
        max_failure_ratio: 0.3
    command:
      ["{{if .Etcd}}-k {{.Etcd}}{{end}}",
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
       "-x", "swarm"]
    restart: always
    privileged: true
