{{- define "carbide-rest-site-agent.namespace" -}}
{{- default .Release.Namespace .Values.namespaceOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "carbide-rest-site-agent.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "carbide-rest-site-agent.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "carbide-rest-site-agent.labels" -}}
helm.sh/chart: {{ include "carbide-rest-site-agent.chart" . }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/part-of: carbide-rest
app.kubernetes.io/name: carbide-rest-site-agent
app.kubernetes.io/component: site-agent
{{- end }}

{{- define "carbide-rest-site-agent.selectorLabels" -}}
app.kubernetes.io/name: carbide-rest-site-agent
app.kubernetes.io/component: site-agent
{{- end }}

{{- define "carbide-rest-site-agent.image" -}}
{{ .Values.global.image.repository }}/{{ .Values.image.name }}:{{ .Values.global.image.tag }}
{{- end }}
