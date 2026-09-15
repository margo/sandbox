{{- if .Values.secrets.create }}
apiVersion: v1
kind: Secret
metadata:
  name: {{ include "agentchart.certsecretname" . }}
  namespace: {{ include "agentchart.namespace" . }}
type: Opaque
data:
  harbor.crt: {{ .Files.Get "harbor.crt" | b64enc | quote }}
  payload-cert.pem: {{ .Files.Get "payload-cert.pem" | b64enc | quote }}
  payload-key.pem: {{ .Files.Get "payload-key.pem" | b64enc | quote }}
{{- end }}
