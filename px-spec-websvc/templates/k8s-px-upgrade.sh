OCI_MON_IMAGE=portworx/oci-monitor
OCI_MON_TAG=1.3.0-rc4
TALISMAN_IMAGE=portworx/talisman
SCALE_DOWN_SHARED_APPS_MODE=auto
OPERATION=upgrade

usage()
{
  echo "
  usage: [ -op <upgrade|restoresharedapps> -t <new oci tag> --scaledownsharedapps <auto|on|off> ]
  examples:
            # (Default for no arguments) Upgrade Portworx using default image ($OCI_MON_IMAGE:$OCI_MON_TAG)

            # Upgrade Portworx but disable scaling down of Portworx shared applications
            --scaledownsharedapps off

            # Upgrade Portworx using oci monitor tag 1.3.0-rc5
            -t 1.3.0-rc5

            # Restore shared Portworx applications back to their original replica counts in situation where previous upgrade job failed to restore them
            -op restoresharedapps
       "
  exit
}


while [ "$1" != "" ]; do
    case $1 in
        -i | --ocimonimage )    shift
                                OCI_MON_IMAGE=$1
                                ;;
        -t | --ocimontag )      shift
                                OCI_MON_TAG=$1
                                ;;
        --scaledownsharedapps ) shift
                                SCALE_DOWN_SHARED_APPS_MODE=$1
                                ;;
        -ti | --talismanimage ) shift
                                TALISMAN_IMAGE=$1
                                ;;
        -op | --operation )     shift
                                OPERATION=$1
                                ;;
        -tt | --talismantag )   shift
                                TALISMAN_TAG=$1
                                ;;
        -h | --help )           usage
                                ;;
        * )                     shift
                                fatal "unsupported argument: $1"
    esac
    shift
done

fatal() {
  echo "" 2>&1
  echo "$@" 2>&1
  exit 1
}

# derived from https://gist.github.com/davejamesmiller/1965569
ask() {
    # https://djm.me/ask
    local prompt default reply

    while true; do

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

        # Ask the question (not using "read -p" as it uses stderr not stdout)
        echo -n "$1 [$prompt] "

        # Read the answer (use /dev/tty in case stdin is redirected from somewhere else)
        read reply </dev/tty

        # Default?
        if [ -z "$reply" ]; then
            reply=$default
        fi

        # Check if the reply is valid
        case "$reply" in
            Y*|y*) return 0 ;;
            N*|n*) return 1 ;;
        esac

    done
}

if [ "$OPERATION" == "upgrade" ]; then
  if ! ask "The operation will upgrade Portworx to $OCI_MON_IMAGE:$OCI_MON_TAG. Do you want to continue?" N; then
    fatal "Aborting $OPERATION..."
  fi
fi

command -v oc
if [ $? -eq 0 ]; then
  echo "Detected openshift system. Adding talisman-account user to privileged scc"
  oc adm policy add-scc-to-user privileged system:serviceaccount:kube-system:talisman-account
  if [ $? -ne 0 ]; then
    fatal "failed to add talisman-account to privileged scc. exit code: $?"
  fi
fi

VER=$(kubectl version --short | awk -Fv '/Server Version: /{print $3}')
if [ -z "$VER" ]; then
	fatal "failed to get kubernetes version. Make sure you have kubectl setup on current machine."
fi

kubectl delete -n kube-system job talisman || true

VER=( ${VER//./ } )
echo "Parsed version is "${VER[0]}.${VER[1]}""

if [ "${VER[0]}.${VER[1]}" == "1.6" ]; then
  RBAC_VER="v1alpha1"
  SUBJECTS_API_VER="apiVersion: v1"
  TALISMAN_DEFAULT_TAG="1.6"
elif [ "${VER[0]}.${VER[1]}" == "1.7" ]; then
  RBAC_VER="v1beta1"
  TALISMAN_DEFAULT_TAG="1.7"
else # 1.8 and greater
  RBAC_VER="v1"
  JOB_BACKOFF="backoffLimit: 1"
  TALISMAN_DEFAULT_TAG=latest
fi

if [ -z "$TALISMAN_TAG" ]; then
  TALISMAN_TAG=$TALISMAN_DEFAULT_TAG
fi

ARGS="-operation, $OPERATION,"
if [ "$OPERATION" == "upgrade" ]; then
  ARGS="$ARGS -ocimonimage, $OCI_MON_IMAGE, -ocimontag, $OCI_MON_TAG, -scaledownsharedapps, $SCALE_DOWN_SHARED_APPS_MODE"
fi

cat <<EOF | kubectl apply -f -
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: talisman-account
  namespace: kube-system
---
kind: ClusterRoleBinding
apiVersion: rbac.authorization.k8s.io/$RBAC_VER
metadata:
  name: talisman-role-binding
subjects:
- kind: ServiceAccount
  name: talisman-account
  namespace: kube-system
  $SUBJECTS_API_VER
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
  $JOB_BACKOFF
  template:
    spec:
      serviceAccount: talisman-account
      containers:
      - name: talisman
        image: $TALISMAN_IMAGE:$TALISMAN_TAG
        args: [ $ARGS ]
      restartPolicy: Never
EOF

echo "Talisman job for $OPERATION started. Monitor logs using: 'kubectl logs -n kube-system -l job-name=talisman'"
