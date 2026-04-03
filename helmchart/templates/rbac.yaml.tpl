{{- if .Values.rbac.create }}
apiVersion: v1
kind: ServiceAccount
metadata:
  name: {{ include "agentchart.serviceaccountname" . }}
  namespace: {{ include "agentchart.namespace" . }}
  labels:
    {{- include "agentchart.labels" . | nindent 4 }}
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: {{ include "agentchart.rolename" . }}
  labels:
    {{- include "agentchart.labels" . | nindent 4 }}
rules:
# Read permissions
- apiGroups: [""]
  resources: ["pods", "services", "endpoints", "nodes", "namespaces", "events"]
  verbs: ["get", "list", "watch"]
- apiGroups: ["apps"]
  resources: ["deployments", "replicasets", "daemonsets", "statefulsets"]
  verbs: ["get", "list", "watch"]
- apiGroups: ["extensions", "networking.k8s.io"]
  resources: ["ingresses", "ingressclasses"]
  verbs: ["get", "list", "watch"]
- apiGroups: ["metrics.k8s.io"]
  resources: ["pods", "nodes"]
  verbs: ["get", "list"]

# Write permissions for workload resources
- apiGroups: [""]
  resources: ["secrets", "configmaps", "services", "pods", "serviceaccounts", "events"]
  verbs: ["create", "get", "list", "update", "patch", "delete"]
- apiGroups: ["apps"]
  resources: ["deployments", "replicasets", "daemonsets", "statefulsets"]
  verbs: ["create", "get", "list", "update", "patch", "delete"]
- apiGroups: ["extensions", "networking.k8s.io"]
  resources: ["ingresses"]
  verbs: ["create", "get", "list", "update", "patch", "delete"]

# Storage permissions
- apiGroups: [""]
  resources: ["persistentvolumes", "persistentvolumeclaims"]
  verbs: ["create", "get", "list", "update", "patch", "delete"]

# RBAC permissions (needed for Helm charts that create RBAC resources)
- apiGroups: ["rbac.authorization.k8s.io"]
  resources: ["roles", "rolebindings", "clusterroles", "clusterrolebindings"]
  verbs: ["create", "get", "list", "update", "patch", "delete", "bind", "escalate"]

# CRD management permissions
- apiGroups: ["apiextensions.k8s.io"]
  resources: ["customresourcedefinitions"]
  verbs: ["create", "get", "list", "update", "patch", "delete"]

# Networking permissions
- apiGroups: ["networking.k8s.io"]
  resources: ["ingressclasses", "networkpolicies"]
  verbs: ["create", "get", "list", "update", "patch", "delete"]

# Coordination permissions (for leader election)
- apiGroups: ["coordination.k8s.io"]
  resources: ["leases"]
  verbs: ["create", "get", "list", "update", "patch", "delete"]

# Allow managing custom resources created by CRDs
- apiGroups: ["appprotect.f5.com"]
  resources: ["*"]
  verbs: ["*"]
- apiGroups: ["appprotectdos.f5.com"]
  resources: ["*"]
  verbs: ["*"]
- apiGroups: ["externaldns.nginx.org"]
  resources: ["*"]
  verbs: ["*"]
- apiGroups: ["k8s.nginx.org"]
  resources: ["*"]
  verbs: ["*"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: {{ include "agentchart.rolebindingname" . }}
  labels:
    {{- include "agentchart.labels" . | nindent 4 }}
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: {{ include "agentchart.rolename" . }}
subjects:
- kind: ServiceAccount
  name: {{ include "agentchart.serviceaccountname" . }}
  namespace: {{ include "agentchart.namespace" . }}
{{- end }}
