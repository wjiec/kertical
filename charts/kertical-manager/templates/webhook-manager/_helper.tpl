{{/*
Expand the name of the webhook service.
*/}}
{{- define "kertical-manager.webhook.service.name" -}}
{{- include "kertical-manager.fullname" . }}-webhook
{{- end }}

{{/*
Webhook-Manager component labels
*/}}
{{- define "kertical-manager.webhook.componentLabels" -}}
app.kubernetes.io/component: webhook-manager
{{- end }}

{{/*
Selector labels
*/}}
{{- define "kertical-manager.webhook.selectorLabels" -}}
{{ include "kertical-manager.selectorLabels" . }}
{{ include "kertical-manager.webhook.componentLabels" . }}
{{- end }}

{{/*
Webhook-Manager labels
*/}}
{{- define "kertical-manager.webhook.labels" -}}
{{ include "kertical-manager.labels" . }}
{{ include "kertical-manager.webhook.componentLabels" . }}
{{- end }}

{{/*
Expand the image of the webhook-manager.
*/}}
{{- define "kertical-manager.webhook.image" -}}
{{ .Values.global.imageRegistry }}/{{ .Values.controller.webhook.image.repository }}:{{ .Values.controller.webhook.image.tag | default .Chart.AppVersion }}
{{- end }}

{{/*
Expand the name of the webhook-certs secrets.
*/}}
{{- define "kertical-manager.webhook.service-certs.name" -}}
{{- include "kertical-manager.fullname" . }}-webhook-certs
{{- end }}

{{/*
Expand the name of the webhook metrics service.
*/}}
{{- define "kertical-manager.webhook.metrics-service.name" -}}
{{- include "kertical-manager.fullname" . }}-webhook-metrics
{{- end }}

{{/*
Expand the name of the webhook metrics-certs secrets.
*/}}
{{- define "kertical-manager.webhook.metrics-certs.name" -}}
{{- include "kertical-manager.fullname" . }}-webhook-metrics-certs
{{- end }}
