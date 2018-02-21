#!/bin/sh

TALISMAN_IMAGE=portworx/talisman
TALISMAN_TAG=latest
WIPE_CLUSTER="--wipecluster"

usage()
{
	echo "
	usage:  curl https://install.portworx.com/px-wipe | sh -s [-- [S | --skipmetadata] ]
	examples:
            # Along with deleting Portworx Kubernetes components, also wipe Portworx cluster metadata
            curl https://install.portworx.com/px-wipe | sh -s -- --skipmetadata
      "
}

fatal() {
  echo "" 2>&1
  echo "$@" 2>&1
  exit 1
}

# derived from https://gist.github.com/davejamesmiller/1965569
ask() {
  # https://djm.me/ask
  local prompt default reply
  if [ "${2:-}" = "Y" ]; then
    prompt="Y/n"
    default=Y
  elif [ "${2:-}" = "N" ]; then
    prompt="y/N"
    default=N
  else
    prompt="y/n"
    default=
  fi

  # Ask the question (not using "read -p" as it uses stderr not stdout)<Paste>
  echo -n "$1 [$prompt]:"

  # Read the answer (use /dev/tty in case stdin is redirected from somewhere else)
  read reply </dev/tty
  if [ $? -ne 0 ]; then
    fatal "ERROR: Could not ask for user input - please run via interactive shell"
  fi

  # Default? (e.g user presses enter)
  if [ -z "$reply" ]; then
    reply=$default
  fi

  # Check if the reply is valid
  case "$reply" in
    Y*|y*) return 0 ;;
    N*|n*) return 1 ;;
    * )    echo "invalid reply: $reply"; return 1 ;;
  esac
}

while [ "$1" != "" ]; do
    case $1 in
        -I | --talismanimage ) shift
                                TALISMAN_IMAGE=$1
                                ;;
        -T | --talismantag )   shift
                                TALISMAN_TAG=$1
                                ;;
        -S | --skipmetadata )   WIPE_CLUSTER=""
                                ;;
        -h | --help )           usage
                                ;;
        * )                     usage
    esac
    shift
done

if [ -z "$WIPE_CLUSTER" ]; then
  if ! ask "The operation will delete Portworx components from the cluster. Do you want to continue?" N; then
    fatal "Aborting Portworx wipe from the cluster..."
  fi
else
  if ! ask "The operation will delete Portworx components and metadata from the cluster. The operation is irreversible and will lead to DATA LOSS. Do you want to continue?" N; then
    fatal "Aborting Portworx wipe from the cluster..."
  fi
fi

VER=$(kubectl version --short | awk -Fv '/Server Version: /{print $3}')
if [ -z "$VER" ]; then
	fatal "failed to get kubernetes version. Make sure you have kubectl setup on current machine."
fi

if [ "${VER[0]}.${VER[1]}" == "1.7" ] || [ "${VER[0]}.${VER[1]}" == "1.6" ]; then
  fatal "This script doesn't support wiping Portworx from Kubernetes $VER clusters. Refer to https://docs.portworx.com/scheduler/kubernetes/install.html for instructions"
fi

kubectl delete -n kube-system job talisman || true

cat <<EOF | kubectl apply -f -
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: talisman-account
  namespace: kube-system
---
kind: ClusterRoleBinding
apiVersion: rbac.authorization.k8s.io/v1beta1
metadata:
  name: talisman-role-binding
subjects:
- kind: ServiceAccount
  name: talisman-account
  namespace: kube-system
roleRef:
  kind: ClusterRole
  name: cluster-admin
  apiGroup: rbac.authorization.k8s.io
---

apiVersion: batch/v1
kind: Job
metadata:
  name: talisman
  namespace: kube-system
spec:
  backoffLimit: 1
  template:
    spec:
      serviceAccount: talisman-account
      containers:
      - name: talisman
        image: $TALISMAN_IMAGE:$TALISMAN_TAG
        args: ["-operation",  "delete", "$WIPE_CLUSTER"]
        volumeMounts:
        - name: etcpwx
          mountPath: /etc/pwx
      volumes:
      - name: etcpwx
        hostPath:
          path: /etc/pwx
      restartPolicy: Never
EOF

echo "Talisman job for wiping Portworx started. Monitor logs using: 'kubectl logs -n kube-system -l job-name=talisman'"
