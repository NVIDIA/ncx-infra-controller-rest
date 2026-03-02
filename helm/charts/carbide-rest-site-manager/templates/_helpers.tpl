{{- define "carbide-rest-site-manager.namespace" -}}
{{- default .Release.Namespace .Values.namespaceOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "carbide-rest-site-manager.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "carbide-rest-site-manager.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "carbide-rest-site-manager.labels" -}}
helm.sh/chart: {{ include "carbide-rest-site-manager.chart" . }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/part-of: carbide-rest
app.kubernetes.io/name: carbide-rest-site-manager
app.kubernetes.io/component: site-manager
{{- end }}

{{- define "carbide-rest-site-manager.selectorLabels" -}}
app.kubernetes.io/name: carbide-rest-site-manager
app.kubernetes.io/component: site-manager
{{- end }}

{{- define "carbide-rest-site-manager.image" -}}
{{ .Values.global.image.repository }}/{{ .Values.image.name }}:{{ .Values.global.image.tag }}
{{- end }}
