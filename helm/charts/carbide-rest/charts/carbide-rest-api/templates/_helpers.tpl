{{- define "carbide-rest-api.namespace" -}}
{{- default .Release.Namespace .Values.namespaceOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "carbide-rest-api.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "carbide-rest-api.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "carbide-rest-api.labels" -}}
helm.sh/chart: {{ include "carbide-rest-api.chart" . }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/part-of: carbide-rest
app.kubernetes.io/name: carbide-rest-api
app.kubernetes.io/component: api
{{- end }}

{{- define "carbide-rest-api.selectorLabels" -}}
app: carbide-rest-api
app.kubernetes.io/name: carbide-rest-api
app.kubernetes.io/component: api
{{- end }}

{{- define "carbide-rest-api.image" -}}
{{ .Values.global.image.repository }}/{{ .Values.image.name }}:{{ .Values.global.image.tag }}
{{- end }}

{{- define "carbide-rest-api.validateAuth" -}}
{{- if and (not .Values.config.keycloak.enabled) (not .Values.config.issuers) -}}
{{- fail "Either keycloak must be enabled or at least one JWT issuer must be configured in config.issuers" -}}
{{- end -}}
{{- if and .Values.config.keycloak.enabled .Values.config.issuers -}}
{{- fail "keycloak and issuers are mutually exclusive — enable only one" -}}
{{- end -}}
{{- end -}}
