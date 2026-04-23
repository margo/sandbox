{{- if .Values.secrets.create }}
apiVersion: v1
kind: Secret
metadata:
  name: {{ include "agentchart.certsecretname" . }}
  namespace: {{ include "agentchart.namespace" . }}
type: Opaque
data:
  device-private.key: {{ .Files.Get "device-private.key" | b64enc | quote }}
  device-public.crt: {{ .Files.Get "device-public.crt" | b64enc | quote }}
  device-ecdsa.key: {{ .Files.Get "device-ecdsa.key" | b64enc | quote }}
  device-ecdsa.crt: {{ .Files.Get "device-ecdsa.crt" | b64enc | quote }}
  harbor.crt: {{ .Files.Get "harbor.crt" | b64enc | quote }}
{{- end }}
