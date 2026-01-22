{{/*
Expand the name of the chart.
*/}}
{{- define "hcs.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
*/}}
{{- define "hcs.fullname" -}}
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
{{- define "hcs.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Namespace to use
*/}}
{{- define "hcs.namespace" -}}
{{- if .Values.global.namespace }}
{{- .Values.global.namespace }}
{{- else }}
{{- .Release.Namespace }}
{{- end }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "hcs.labels" -}}
helm.sh/chart: {{ include "hcs.chart" . }}
{{ include "hcs.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "hcs.selectorLabels" -}}
app.kubernetes.io/name: {{ include "hcs.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Node Agent labels
*/}}
{{- define "hcs.nodeAgent.labels" -}}
{{ include "hcs.labels" . }}
app.kubernetes.io/component: node-agent
{{- end }}

{{/*
Node Agent selector labels
*/}}
{{- define "hcs.nodeAgent.selectorLabels" -}}
{{ include "hcs.selectorLabels" . }}
app.kubernetes.io/component: node-agent
{{- end }}

{{/*
Scheduler labels
*/}}
{{- define "hcs.scheduler.labels" -}}
{{ include "hcs.labels" . }}
app.kubernetes.io/component: scheduler
{{- end }}

{{/*
Scheduler selector labels
*/}}
{{- define "hcs.scheduler.selectorLabels" -}}
{{ include "hcs.selectorLabels" . }}
app.kubernetes.io/component: scheduler
{{- end }}

{{/*
Webhook labels
*/}}
{{- define "hcs.webhook.labels" -}}
{{ include "hcs.labels" . }}
app.kubernetes.io/component: webhook
{{- end }}

{{/*
Webhook selector labels
*/}}
{{- define "hcs.webhook.selectorLabels" -}}
{{ include "hcs.selectorLabels" . }}
app.kubernetes.io/component: webhook
{{- end }}

{{/*
Node Agent service account name
*/}}
{{- define "hcs.nodeAgent.serviceAccountName" -}}
{{- if .Values.nodeAgent.serviceAccount.create }}
{{- default (printf "%s-node-agent" (include "hcs.fullname" .)) .Values.nodeAgent.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.nodeAgent.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Scheduler service account name
*/}}
{{- define "hcs.scheduler.serviceAccountName" -}}
{{- if .Values.scheduler.serviceAccount.create }}
{{- default (printf "%s-scheduler" (include "hcs.fullname" .)) .Values.scheduler.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.scheduler.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Webhook service account name
*/}}
{{- define "hcs.webhook.serviceAccountName" -}}
{{- if .Values.webhook.serviceAccount.create }}
{{- default (printf "%s-webhook" (include "hcs.fullname" .)) .Values.webhook.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.webhook.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Node Agent image
*/}}
{{- define "hcs.nodeAgent.image" -}}
{{- $registry := .Values.global.imageRegistry | default "" -}}
{{- $repository := .Values.nodeAgent.image.repository -}}
{{- $tag := .Values.nodeAgent.image.tag | default .Chart.AppVersion -}}
{{- if $registry -}}
{{- printf "%s/%s:%s" $registry $repository $tag -}}
{{- else -}}
{{- printf "%s:%s" $repository $tag -}}
{{- end -}}
{{- end }}

{{/*
Scheduler image
*/}}
{{- define "hcs.scheduler.image" -}}
{{- $registry := .Values.global.imageRegistry | default "" -}}
{{- $repository := .Values.scheduler.image.repository -}}
{{- $tag := .Values.scheduler.image.tag | default .Chart.AppVersion -}}
{{- if $registry -}}
{{- printf "%s/%s:%s" $registry $repository $tag -}}
{{- else -}}
{{- printf "%s:%s" $repository $tag -}}
{{- end -}}
{{- end }}

{{/*
Webhook image
*/}}
{{- define "hcs.webhook.image" -}}
{{- $registry := .Values.global.imageRegistry | default "" -}}
{{- $repository := .Values.webhook.image.repository -}}
{{- $tag := .Values.webhook.image.tag | default .Chart.AppVersion -}}
{{- if $registry -}}
{{- printf "%s/%s:%s" $registry $repository $tag -}}
{{- else -}}
{{- printf "%s:%s" $repository $tag -}}
{{- end -}}
{{- end }}

{{/*
Webhook service name
*/}}
{{- define "hcs.webhook.serviceName" -}}
{{- printf "%s-webhook" (include "hcs.fullname" .) }}
{{- end }}

{{/*
Webhook certificate name
*/}}
{{- define "hcs.webhook.certificateName" -}}
{{- printf "%s-webhook-cert" (include "hcs.fullname" .) }}
{{- end }}

{{/*
Webhook TLS secret name
*/}}
{{- define "hcs.webhook.tlsSecretName" -}}
{{- if .Values.webhook.certManager.enabled }}
{{- printf "%s-webhook-tls" (include "hcs.fullname" .) }}
{{- else if .Values.webhook.tls.secretName }}
{{- .Values.webhook.tls.secretName }}
{{- else }}
{{- printf "%s-webhook-tls" (include "hcs.fullname" .) }}
{{- end }}
{{- end }}

{{/*
Image pull secrets
*/}}
{{- define "hcs.imagePullSecrets" -}}
{{- if .Values.global.imagePullSecrets }}
imagePullSecrets:
{{- range .Values.global.imagePullSecrets }}
  - name: {{ . }}
{{- end }}
{{- end }}
{{- end }}
