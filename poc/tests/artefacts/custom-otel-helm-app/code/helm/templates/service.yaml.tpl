apiVersion: v1
kind: Service
metadata:
  name: {{ include "custom-otel-helm.fullname" . }}
  labels:
    {{- include "custom-otel-helm.labels" . | nindent 4 }}
spec:
  type: {{ .Values.service.type }}
  selector:
    {{- include "custom-otel-helm.selectorLabels" . | nindent 4 }}
  ports:
    - protocol: TCP
      port: {{ .Values.service.port }}
      targetPort: 80
