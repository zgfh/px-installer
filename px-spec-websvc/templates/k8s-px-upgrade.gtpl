apiVersion: v1
kind: ServiceAccount
metadata:
  name: talisman-account
  namespace: kube-system
---
kind: ClusterRoleBinding
apiVersion: rbac.authorization.k8s.io/{{.RbacAuthVer}}
metadata:
  name: talisman-role-binding
subjects:
- kind: ServiceAccount
  name: talisman-account
  namespace: kube-system
{{- if lt .KubeVer "1.6.z"}}
  apiVersion: v1
{{- end}}
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
        image: {{.TalismanImage}}:{{.TalismanTag}}
        args:
          ["-operation","upgrade",
           {{- if .OCIMonImage}}"-ocimonimage", "{{.OCIMonImage}}",{{end}}
           {{- if .OCIMonTag}}"-ocimontag" ,"{{.OCIMonTag}}",{{end}}
           {{- if .PXImage}}"-pximage", "{{.PXImage}}",{{end}}
           {{- if .PXTag}}"-pxtag", "{{.PXTag}}",{{end}}
          ]
      restartPolicy: Never
