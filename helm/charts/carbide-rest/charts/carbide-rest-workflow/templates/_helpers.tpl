{{- define "carbide-rest-workflow.namespace" -}}
{{- default .Release.Namespace .Values.namespaceOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "carbide-rest-workflow.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "carbide-rest-workflow.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "carbide-rest-workflow.labels" -}}
helm.sh/chart: {{ include "carbide-rest-workflow.chart" . }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/part-of: carbide-rest
app.kubernetes.io/name: carbide-rest-workflow
{{- end }}

{{- define "carbide-rest-workflow.image" -}}
{{ .Values.global.image.repository }}/{{ .Values.image.name }}:{{ .Values.global.image.tag }}
{{- end }}

{{- define "carbide-rest-workflow.dbCredsVolumeMount" -}}
{{- if .Values.secrets.dbCreds }}
- name: db-creds
  mountPath: /var/secrets/db
  readOnly: true
{{- end }}
{{- end }}

{{- define "carbide-rest-workflow.dbCredsVolume" -}}
{{- if .Values.secrets.dbCreds }}
- name: db-creds
  secret:
    secretName: {{ .Values.secrets.dbCreds }}
    items:
      - key: password
        path: password
{{- end }}
{{- end }}
