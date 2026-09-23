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

          command: ["/bin/sh", "-c"]

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
            # ConfigMap + device-agent Secret are combined here.
            - name: agent-config-volume
              mountPath: /config
              readOnly: true

            # Mount the host file directly over the expected container path
            - name: host-authorized-json
              mountPath: /config/authorized.json
              subPath: authorized.json

            # Persistent application data.
            - name: data-volume
              mountPath: /data

            # Harbor CA certificate.
            - name: certs
              mountPath: /certs
              readOnly: true

            - name: certs
              mountPath: /usr/local/share/ca-certificates/harbor.crt
              subPath: harbor.crt
              readOnly: true

      volumes:

        # Combine the non-sensitive ConfigMap files with the
        # sensitive device-agent configuration Secret.
        - name: agent-config-volume
          projected:
            sources:

              # config.yaml and capabilities.json
              - configMap:
                  name: {{ include "agentchart.configmapname" . }}

              # Identity, MIS  files
              - secret:
                  name: {{ .Values.secrets.existingSecret }}
                  items:
                    - key: payload-cert.pem
                      path: identity/payload-cert.pem

                    - key: payload-key.pem
                      path: identity/payload-key.pem

                    - key: https-ca.crt
                      path: mis/https-ca.crt


        # hostPath volume sourcing directly from your Helm values
        - name: host-authorized-json
          hostPath:
            path: {{ .Values.hostConfig.authorizedJsonPath | quote }}
            type: File

        # Persistent storage.
        - name: data-volume
{{- if .Values.persistence.enabled }}
          persistentVolumeClaim:
            claimName: {{ .Values.persistence.existingClaim | default (include "agentchart.pvcname" .) }}
{{- else }}
          emptyDir: {}
{{- end }}

        # Harbor certificate Secret.
        - name: certs
          secret:
            secretName: {{ include "agentchart.certsecretname" . }}
