apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ include "agentchart.configmapname" . }}
  namespace: {{ include "agentchart.namespace" . }}
data:
  config.yaml: |
{{- .Files.Get "config/config.yaml" | nindent 4 }}
  profiles.json: |
{{- .Files.Get "config/profiles.json" | nindent 4 }}
