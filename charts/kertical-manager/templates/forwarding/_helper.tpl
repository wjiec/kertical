{{/*
Port-Forwarding component labels
*/}}
{{- define "kertical-manager.forwarding.componentLabels" -}}
app.kubernetes.io/component: forwarding
{{- end }}

{{/*
Selector labels
*/}}
{{- define "kertical-manager.forwarding.selectorLabels" -}}
{{ include "kertical-manager.selectorLabels" . }}
{{ include "kertical-manager.forwarding.componentLabels" . }}
{{- end }}

{{/*
Port-Forwarding labels
*/}}
{{- define "kertical-manager.forwarding.labels" -}}
{{ include "kertical-manager.labels" . }}
{{ include "kertical-manager.forwarding.componentLabels" . }}
{{- end }}

{{/*
Expand the image of the Port-Forwarding.
*/}}
{{- define "kertical-manager.forwarding.image" -}}
{{ .Values.global.imageRegistry }}/{{ .Values.controller.forwarding.image.repository }}:{{ .Values.controller.forwarding.image.tag | default .Chart.AppVersion }}
{{- end }}

{{/*
Expand the name of the forwarding-certs secrets.
*/}}
{{- define "kertical-manager.forwarding.service-certs.name" -}}
{{- include "kertical-manager.fullname" . }}-forwarding-certs
{{- end }}

{{/*
Expand the name of the forwarding metrics service.
*/}}
{{- define "kertical-manager.forwarding.metrics-service.name" -}}
{{- include "kertical-manager.fullname" . }}-forwarding-metrics
{{- end }}

{{/*
Expand the name of the controller forwarding-certs secrets.
*/}}
{{- define "kertical-manager.forwarding.metrics-certs.name" -}}
{{- include "kertical-manager.fullname" . }}-forwarding-metrics-certs
{{- end }}
