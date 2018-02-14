OCI_MON_IMAGE=portworx/oci-monitor
OCI_MON_TAG=1.3.0-rc4
TALISMAN_IMAGE=portworx/talisman
SCALE_DOWN_SHARED_APPS_MODE=auto
OPERATION=upgrade

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
        * )                     shift
                                echo "unsupported argument: $1"
                                exit 1
    esac
    shift
done

VER=$(kubectl version --short | awk -Fv '/Server Version: /{print $3}')
if [ -z "$VER" ]; then
	echo "failed to get kubernetes version. Make sure you have kubectl setup on current machine."
	exit $?
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
