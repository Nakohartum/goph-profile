{{- define "goph-profile.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- define "goph-profile.fullname" -}}
{{- default (printf "%s-%s" .Release.Name (include "goph-profile.name" .)) .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- define "goph-profile.labels" -}}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" }}
app.kubernetes.io/name: {{ include "goph-profile.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}
{{- define "goph-profile.selectorLabels" -}}
app.kubernetes.io/name: {{ include "goph-profile.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}
{{- define "goph-profile.secretName" -}}
{{- default (include "goph-profile.fullname" .) .Values.secrets.existingSecret }}
{{- end }}
{{- define "goph-profile.env" -}}
- name: DATABASE_URL
  valueFrom: {secretKeyRef: {name: {{ include "goph-profile.secretName" . }}, key: database-url}}
- name: S3_ACCESS_KEY
  valueFrom: {secretKeyRef: {name: {{ include "goph-profile.secretName" . }}, key: s3-access-key}}
- name: S3_SECRET_KEY
  valueFrom: {secretKeyRef: {name: {{ include "goph-profile.secretName" . }}, key: s3-secret-key}}
- name: RABBITMQ_URL
  valueFrom: {secretKeyRef: {name: {{ include "goph-profile.secretName" . }}, key: rabbitmq-url}}
- name: S3_ENDPOINT
  valueFrom: {configMapKeyRef: {name: {{ include "goph-profile.fullname" . }}, key: s3-endpoint}}
- name: S3_BUCKET
  valueFrom: {configMapKeyRef: {name: {{ include "goph-profile.fullname" . }}, key: s3-bucket}}
- name: S3_USE_SSL
  valueFrom: {configMapKeyRef: {name: {{ include "goph-profile.fullname" . }}, key: s3-use-ssl}}
- name: LOG_LEVEL
  valueFrom: {configMapKeyRef: {name: {{ include "goph-profile.fullname" . }}, key: log-level}}
- name: OTEL_EXPORTER_OTLP_ENDPOINT
  valueFrom: {configMapKeyRef: {name: {{ include "goph-profile.fullname" . }}, key: otlp-endpoint}}
{{- end }}
