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

      # authorized.json is NOT copied. It remains hostPath-backed and
      # /config/authorized.json is a symlink to the host-backed file.
      initContainers:
        - name: assemble-config
          image: "{{ .Values.image.repository }}:{{ .Values.image.tag }}"
          imagePullPolicy: {{ .Values.image.pullPolicy }}

          command:
            - /bin/sh
            - -c

          args:
            - |
              set -e

              echo "Assembling device-agent configuration..."

              mkdir -p /final-config/identity
              mkdir -p /final-config/mis

              # ConfigMap files
              cp /raw-config/config.yaml \
                 /final-config/config.yaml

              cp /raw-config/capabilities.json \
                 /final-config/capabilities.json

              # Identity certificates
              cp /raw-secret/payload-cert.pem \
                 /final-config/identity/payload-cert.pem

              cp /raw-secret/payload-key.pem \
                 /final-config/identity/payload-key.pem

              # MIS CA certificate
              cp /raw-secret/https-ca.crt \
                 /final-config/mis/https-ca.crt

              # authorized.json remains host-backed.
              # Create a symlink instead of copying the file.
              ln -sf /host-config/authorized.json \
                     /final-config/authorized.json

              echo "Final configuration:"
              ls -laR /final-config

              echo "authorized.json:"
              ls -l /final-config/authorized.json

          volumeMounts:
            - name: raw-config-volume
              mountPath: /raw-config
              readOnly: true

            - name: raw-secret-volume
              mountPath: /raw-secret
              readOnly: true

            # Host-backed authorized.json.
            # Mounted outside /config to avoid nested-volume mount issues.
            - name: host-authorized-json
              mountPath: /host-config/authorized.json
              readOnly: true

            - name: final-config-volume
              mountPath: /final-config

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

            # Final assembled configuration.
            #
            # /config/
            # ├── config.yaml
            # ├── capabilities.json
            # ├── authorized.json -> /host-config/authorized.json
            # ├── identity/
            # │   ├── payload-cert.pem
            # │   └── payload-key.pem
            # └── mis/
            #     └── https-ca.crt
            - name: final-config-volume
              mountPath: /config
              readOnly: true

            # Same hostPath must be available in the main container so
            # /config/authorized.json symlink can resolve.
            - name: host-authorized-json
              mountPath: /host-config/authorized.json
              readOnly: true

            # Persistent application data.
            - name: data-volume
              mountPath: /data

            # Certificate Secret.
            - name: certs
              mountPath: /certs
              readOnly: true

            # Harbor CA certificate.
            - name: certs
              mountPath: /usr/local/share/ca-certificates/harbor.crt
              subPath: harbor.crt
              readOnly: true

      volumes:

        # ConfigMap containing:
        #   config.yaml
        #   capabilities.json
        - name: raw-config-volume
          configMap:
            name: {{ include "agentchart.configmapname" . }}

        # Secret containing device-agent identity and MIS certificate.
        - name: raw-secret-volume
          secret:
            secretName: {{ .Values.secrets.existingSecret }}
            items:
              - key: payload-cert.pem
                path: payload-cert.pem

              - key: payload-key.pem
                path: payload-key.pem

              - key: https-ca.crt
                path: https-ca.crt

        # Canonical host authorized.json.
        # The actual host path comes from: .Values.hostConfig.authorizedJsonPath
        # Example:
        # /home/runner/sandbox/poc/device/agent/config/authorized.json
        - name: host-authorized-json
          hostPath:
            path: {{ .Values.hostConfig.authorizedJsonPath | quote }}
            type: File

        # Final /config directory assembled by the initContainer.
        - name: final-config-volume
          emptyDir: {}

        # Persistent application data.
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