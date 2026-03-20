{{- define "carbide-rest-cert-manager.namespace" -}}
{{- default .Release.Namespace .Values.namespaceOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "carbide-rest-cert-manager.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "carbide-rest-cert-manager.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "carbide-rest-cert-manager.labels" -}}
helm.sh/chart: {{ include "carbide-rest-cert-manager.chart" . }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/part-of: carbide-rest
app.kubernetes.io/name: carbide-rest-cert-manager
app.kubernetes.io/component: cert-manager
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{- define "carbide-rest-cert-manager.selectorLabels" -}}
app: carbide-rest-cert-manager
app.kubernetes.io/name: carbide-rest-cert-manager
app.kubernetes.io/component: cert-manager
{{- end }}

{{- define "carbide-rest-cert-manager.image" -}}
{{ .Values.global.image.repository }}/{{ .Values.image.name }}:{{ .Values.global.image.tag }}
{{- end }}
