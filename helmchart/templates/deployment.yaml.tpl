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

      # Assembles the final /config tree the main container reads, into a plain
      # writable emptyDir - NOT by nesting a hostPath bind-mount inside the
      # projected ConfigMap+Secret volume (that pattern produces the
      # "not a directory" runc/ENOTDIR error you hit: projected volumes are
      # implemented via an atomic symlink-swap, not a static directory, and
      # bind-mounting a separate volume on top of/inside that is unreliable).
      #
      # config.yaml / capabilities.json / identity / mis are copied once at pod
      # start (they don't need to change without a redeploy). authorized.json is
      # SYMLINKED, not copied, from the hostPath mount - reads always resolve
      # through to the live host file, so host edits still propagate without a
      # restart, without ever nesting a mount inside another mount.
      initContainers:
        - name: assemble-config
          image: "{{ .Values.image.repository }}:{{ .Values.image.tag }}"
          imagePullPolicy: {{ .Values.image.pullPolicy }}
          command: ["/bin/sh", "-c"]
          args:
            - |
              set -e
              mkdir -p /final-config/identity /final-config/mis
              cp /raw-config/config.yaml        /final-config/
              cp /raw-config/capabilities.json  /final-config/
              cp /raw-secret/payload-cert.pem   /final-config/identity/
              cp /raw-secret/payload-key.pem    /final-config/identity/
              cp /raw-secret/https-ca.crt       /final-config/mis/
              ln -sf /raw-authorized-json/authorized.json /final-config/authorized.json
              echo "Assembled /final-config:"
              ls -laR /final-config
          volumeMounts:
            - name: raw-config-volume
              mountPath: /raw-config
              readOnly: true
            - name: raw-secret-volume
              mountPath: /raw-secret
              readOnly: true
            - name: host-authorized-json
              mountPath: /raw-authorized-json
              readOnly: true
            - name: final-config-volume
              mountPath: /final-config

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
            # Fully assembled by the initContainer - config.yaml, capabilities.json,
            # identity/, mis/, and a live symlink for authorized.json. No nested
            # mounts here, just one plain directory.
            - name: final-config-volume
              mountPath: /config
              readOnly: true

            # authorized.json's symlink target must still be present in THIS
            # container's mount namespace for the symlink to resolve - same
            # hostPath, mounted directly (not nested under /config).
            - name: host-authorized-json
              mountPath: /raw-authorized-json
              readOnly: true

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

        # Raw ConfigMap source (config.yaml, capabilities.json), read by the
        # initContainer only - no longer mounted directly into the main container.
        - name: raw-config-volume
          configMap:
            name: {{ include "agentchart.configmapname" . }}

        # Raw Secret source (identity + MIS material), read by the initContainer only.
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

        # hostPath volume sourcing directly from your Helm values. Mounted
        # independently (not nested under /config) in both the initContainer
        # (to create the symlink target reference) and the main container (so the
        # symlink itself resolves inside the running container's mount namespace).
        - name: host-authorized-json
          hostPath:
            path: {{ .Values.hostConfig.authorizedJsonPath | quote }}
            type: File

        # Plain writable emptyDir the initContainer assembles and the main
        # container reads read-only. This is what replaces the old projected
        # volume + nested hostPath bind-mount.
        - name: final-config-volume
          emptyDir: {}

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
