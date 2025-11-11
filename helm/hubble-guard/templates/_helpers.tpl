{{/*
Expand the name of the chart.
*/}}
{{- define "hubble-guard.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "hubble-guard.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "hubble-guard.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "hubble-guard.labels" -}}
helm.sh/chart: {{ include "hubble-guard.chart" . }}
{{ include "hubble-guard.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "hubble-guard.selectorLabels" -}}
app.kubernetes.io/name: {{ include "hubble-guard.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Anomaly Detector labels
*/}}
{{- define "hubble-guard.anomalyDetector.labels" -}}
{{ include "hubble-guard.labels" . }}
app.kubernetes.io/component: anomaly-detector
{{- end }}

{{/*
Anomaly Detector selector labels
*/}}
{{- define "hubble-guard.anomalyDetector.selectorLabels" -}}
{{ include "hubble-guard.selectorLabels" . }}
app.kubernetes.io/component: anomaly-detector
{{- end }}

{{/*
Prometheus labels
*/}}
{{- define "hubble-guard.prometheus.labels" -}}
{{ include "hubble-guard.labels" . }}
app.kubernetes.io/component: prometheus
{{- end }}

{{/*
Prometheus selector labels
*/}}
{{- define "hubble-guard.prometheus.selectorLabels" -}}
{{ include "hubble-guard.selectorLabels" . }}
app.kubernetes.io/component: prometheus
{{- end }}

{{/*
Grafana labels
*/}}
{{- define "hubble-guard.grafana.labels" -}}
{{ include "hubble-guard.labels" . }}
app.kubernetes.io/component: grafana
{{- end }}

{{/*
Grafana selector labels
*/}}
{{- define "hubble-guard.grafana.selectorLabels" -}}
{{ include "hubble-guard.selectorLabels" . }}
app.kubernetes.io/component: grafana
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "hubble-guard.serviceAccountName" -}}
{{- if .Values.anomalyDetector.serviceAccount.create }}
{{- default (include "hubble-guard.fullname" .) .Values.anomalyDetector.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.anomalyDetector.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Prometheus service name
*/}}
{{- define "hubble-guard.prometheus.serviceName" -}}
{{- printf "%s-prometheus" (include "hubble-guard.fullname" .) }}
{{- end }}

{{/*
Grafana service name
*/}}
{{- define "hubble-guard.grafana.serviceName" -}}
{{- printf "%s-grafana" (include "hubble-guard.fullname" .) }}
{{- end }}

