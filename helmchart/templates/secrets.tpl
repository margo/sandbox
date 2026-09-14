{{- if .Values.secrets.create }}
apiVersion: v1
kind: Secret
metadata:
  name: {{ include "agentchart.certsecretname" . }}
  namespace: {{ include "agentchart.namespace" . }}
type: Opaque
data:
  harbor.crt: {{ .Files.Get "harbor.crt" | b64enc | quote }}
{{- end }}
