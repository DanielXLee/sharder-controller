{{/*
Expand the name of the chart.
*/}}
{{- define "shard-controller.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "shard-controller.fullname" -}}
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
{{- define "shard-controller.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "shard-controller.labels" -}}
helm.sh/chart: {{ include "shard-controller.chart" . }}
{{ include "shard-controller.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "shard-controller.selectorLabels" -}}
app.kubernetes.io/name: {{ include "shard-controller.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Manager labels
*/}}
{{- define "shard-controller.manager.labels" -}}
{{ include "shard-controller.labels" . }}
app.kubernetes.io/component: manager
{{- end }}

{{/*
Worker labels
*/}}
{{- define "shard-controller.worker.labels" -}}
{{ include "shard-controller.labels" . }}
app.kubernetes.io/component: worker
{{- end }}

{{/*
Create the name of the service account for manager
*/}}
{{- define "shard-controller.manager.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (printf "%s-manager" (include "shard-controller.fullname" .)) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Create the name of the service account for worker
*/}}
{{- define "shard-controller.worker.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (printf "%s-worker" (include "shard-controller.fullname" .)) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Manager image
*/}}
{{- define "shard-controller.manager.image" -}}
{{- $registry := .Values.global.imageRegistry | default "" }}
{{- $repository := .Values.manager.image.repository }}
{{- $tag := .Values.manager.image.tag | default .Chart.AppVersion }}
{{- if $registry }}
{{- printf "%s/%s:%s" $registry $repository $tag }}
{{- else }}
{{- printf "%s:%s" $repository $tag }}
{{- end }}
{{- end }}

{{/*
Worker image
*/}}
{{- define "shard-controller.worker.image" -}}
{{- $registry := .Values.global.imageRegistry | default "" }}
{{- $repository := .Values.worker.image.repository }}
{{- $tag := .Values.worker.image.tag | default .Chart.AppVersion }}
{{- if $registry }}
{{- printf "%s/%s:%s" $registry $repository $tag }}
{{- else }}
{{- printf "%s:%s" $repository $tag }}
{{- end }}
{{- end }}