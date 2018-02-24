#!/bin/bash -ex

MAX_RETRIES=36
TIME_BEFORE_RETRY=5 #seconds
DAEMONSET_NAME=px-pre-install-check

fatal() {
  echo "" 2>&1
  echo "$@" 2>&1
  exit 1
}

fatal_with_logs() {
  echo "Pre-install check failed."
  NOT_READY_PODS=$(kubectl get pods -n kube-system -l name=$DAEMONSET_NAME | grep -v 1/1 | awk '/^px-pre-install-check/{print $1}')
  echo "Dumping logs of failed pods: $NOT_READY_PODS"
  for pod in $NOT_READY_PODS; do
    echo "Dumping logs of pod: $pod"
    kubectl logs $pod -n kube-system
  done

  fatal $@
}

VER=$(kubectl version --short | awk -Fv '/Server Version: /{print $3}')
if [ -z "$VER" ]; then
  fatal "failed to get kubernetes version. Make sure you have kubectl setup on current machine."
fi

VERi=$(echo $VER | awk -F. '{print $1*10+$2}')

command -v oc
if [ $? -eq 0 ]; then
  echo "Detected openshift system. Adding pre-check-account user to privileged scc"
  oc adm policy add-scc-to-user privileged system:serviceaccount:kube-system:pre-check-account
  if [ $? -ne 0 ]; then
    fatal "failed to add pre-check-account to privileged scc. exit code: $?"
  fi
fi

kubectl delete -n kube-system daemonset $DAEMONSET_NAME || true

kubectl apply -f - << _EOF
apiVersion: v1
kind: ServiceAccount
metadata:
  name: px-pre-check-account
  namespace: kube-system
---
apiVersion: extensions/v1beta1
kind: DaemonSet
metadata:
  name: $DAEMONSET_NAME
  namespace: kube-system
spec:
  minReadySeconds: 0
  template:
    metadata:
      labels:
        name: $DAEMONSET_NAME
    spec:
      serviceAccount: px-pre-check-account
      hostNetwork: true
      containers:
      - name: $DAEMONSET_NAME
        image: portworx/px-pre-flight:2.0.0.0
        imagePullPolicy: Always
        args:
             [{{- if .Kvdb}}"-k", "{{.Kvdb}}", {{end}}
             {{- if .DIface}}"-d", "{{.DIface}}", {{end}}
             {{- if .MIface}}"-m", "{{.MIface}}", {{end}}
             {{- if .Drives}}{{.Drives}}, {{end}}
             {{- if .SecretType}}"-secret_type", "{{.SecretType}}", {{end}}
             {{- if .EtcdPasswd}}"-userpwd", "{{.EtcdPasswd}}", {{end}}
             {{- if .EtcdCa}}"-ca", "{{.EtcdCa}}", {{end}}
             {{- if .EtcdCert}}"-cert", "{{.EtcdCert}}", {{end}}
             {{- if .EtcdKey}}"-key", "{{.EtcdKey}}", {{end}}
             "-x", "kubernetes"]
        readinessProbe:
          exec:
            command:
            - cat
            - /tmp/px-precheck-success
        securityContext:
          privileged: true
        volumeMounts:
          - name: dockersock
            mountPath: /var/run/docker.sock
          - name: usrsrc
            mountPath: /usr/src
          - name: libmodules
            mountPath: /lib/modules
          - name: logpxcheck
            mountPath: /var/log/pxcheck:shared
          - name: etcpwx
            mountPath: /etc/pwx
      volumes:
      - name: dockersock
        hostPath:
          path: /var/run/docker.sock
      - name: usrsrc
        hostPath:
          path: /usr/src
      - name: libmodules
        hostPath:
          path: /lib/modules
      - name: logpxcheck
        hostPath:
          path: /var/log/pxcheck
      - name: etcpwx
        hostPath:
          path: /etc/pwx
_EOF
if [ $? -ne 0 ]; then
  fatal "Failed to create preflight check daemonset. exit code: $?"
fi

echo "Pre-flight check started. Use 'kubectl logs -n kube-system -l name=$DAEMONSET_NAME --tail=9999' to monitor the logs."

NUM_DESIRED=$(kubectl get daemonset -n kube-system $DAEMONSET_NAME -o jsonpath='{.status.desiredNumberScheduled}')
if [ $? -ne 0 ]; then
  fatal "Failed to get details about preflight check daemonset: $DAEMONSET_NAME"
fi

echo "Number of desired replicas: $NUM_DESIRED."

RETRY_CNT=0
while true; do
  NUM_READY=$(kubectl get daemonset -n kube-system $DAEMONSET_NAME -o jsonpath='{.status.numberReady}')
  if [ $? -ne 0 ]; then
    fatal_with_logs "Failed to get details about preflight check daemonset: $DAEMONSET_NAME"
  fi

  if [ $NUM_READY -eq $NUM_DESIRED ]; then
    break
  fi

  echo "Waiting for preflight check to pass. Current ready replicas: $NUM_READY Expected ready replicas: $NUM_DESIRED"
  kubectl get pods -n kube-system -l name=$DAEMONSET_NAME -o wide
  if [ $? -ne 0 ]; then
    fatal_with_logs "Failed to list pods for preflight check daemonset: $DAEMONSET_NAME"
  fi

  RETRY_CNT=$((RETRY_CNT+1))
  if [ $RETRY_CNT -ge $MAX_RETRIES ]; then
    fatal_with_logs "Timed out waiting for preflight check daemonset: $DAEMONSET_NAME to finish"
  fi

  sleep $TIME_BEFORE_RETRY
done

echo "Pre-install check passed on all nodes !"
_rc=0
kubectl get pods -n kube-system -l name=$DAEMONSET_NAME -o wide   || _rc=$?
kubectl logs -n kube-system -l name=$DAEMONSET_NAME --tail=9999   || _rc=$?
kubectl delete -n kube-system daemonset $DAEMONSET_NAME           || _rc=$?
kubectl delete -n kube-system serviceaccount px-pre-check-account || _rc=$?
if [ $_rc -ne 0 ]; then
   fatal "Failed to delete preflight check daemonset resources"
 fi
