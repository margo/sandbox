apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ include "agentchart.deploymentname" . }}
  namespace: {{ include "agentchart.namespace" . }}
  labels:
    {{- include "agentchart.labels" . | nindent 4 }}
spec:
  replicas: 1
  selector:
    matchLabels:
      app: {{ include "agentchart.podname" . }}
  template:
    metadata:
      labels:
        app: {{ include "agentchart.podname" . }}
    spec:
      serviceAccountName: {{ include "agentchart.serviceaccountname" . }}
      containers:
        - name: {{ include "agentchart.podname" . }}
          image: "{{ .Values.image.repository }}:{{ .Values.image.tag }}"
          imagePullPolicy: {{ .Values.image.pullPolicy }}          
          command:
            - /bin/sh
            - -c
          args:
            - |
              update-ca-certificates
              exec ./device-agent -config /config/config.yaml
          env:
            - name: KUBERNETES_SERVICE_HOST
              value: "kubernetes.default.svc"
            - name: KUBERNETES_SERVICE_PORT
              value: "443"
          volumeMounts:
            - name: agent-config-volume
              mountPath: /config
              readOnly: true
            - name: data-volume
              mountPath: /data
            - name: certs
              mountPath: /certs
              readOnly: true
            
            - name: certs
               mountPath: /usr/local/share/ca-certificates/harbor.crt
               subPath: harbor.crt
               readOnly: true

              
      volumes:
        - name: agent-config-volume
          configMap:
            name: {{ include "agentchart.configmapname" . }}
        - name: data-volume
          {{- if .Values.persistence.enabled }}
          persistentVolumeClaim:
            claimName: {{ .Values.persistence.existingClaim | default (include "agentchart.pvcname" .) }}
          {{- else }}
          emptyDir: {}
          {{- end }}
        - name: certs
          secret:
            secretName: {{ include "agentchart.certsecretname" . }}
