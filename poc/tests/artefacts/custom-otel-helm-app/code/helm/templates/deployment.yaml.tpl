apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ include "custom-otel-helm.fullname" . }}
  labels:
    {{- include "custom-otel-helm.labels" . | nindent 4 }}
spec:
  replicas: {{ .Values.replicaCount }}
  selector:
    matchLabels:
      {{- include "custom-otel-helm.selectorLabels" . | nindent 6 }}
  template:
    metadata:
      labels:
        {{- include "custom-otel-helm.selectorLabels" . | nindent 8 }}
    spec:
      containers:
        - name: {{ .Chart.Name }}
          image: "{{ .Values.image.repository }}:{{ .Values.image.tag }}"
          imagePullPolicy: {{ .Values.image.pullPolicy }}
          env:
            - name: OTEL_EXPORTER_OTLP_ENDPOINT
              value: {{ .Values.env.OTEL_EXPORTER_OTLP_ENDPOINT }}
