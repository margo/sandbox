{{- if .Values.persistence.enabled }}
{{- if not .Values.persistence.existingClaim }}
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: {{ include "agentchart.pvcname" . }}
  namespace: {{ include "agentchart.namespace" . }}
  labels:
    {{- include "agentchart.labels" . | nindent 4 }}
  annotations:
    "helm.sh/resource-policy": "keep"  
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: {{ .Values.persistence.size | default "1Gi" }}
  {{- if .Values.persistence.storageClassName }}
  storageClassName: {{ .Values.persistence.storageClassName }}
  {{- end }}
{{- end }}
{{- end }}
