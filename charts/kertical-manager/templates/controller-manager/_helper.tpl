{{/*
Expand the name of the controller metrics service.
*/}}
{{- define "kertical-manager.controller.metrics-service.name" -}}
{{- include "kertical-manager.fullname" . }}-controller-metrics
{{- end }}

{{/*
Expand the name of the controller metrics-certs secrets.
*/}}
{{- define "kertical-manager.controller.metrics-certs.name" -}}
{{- include "kertical-manager.fullname" . }}-controller-metrics-certs
{{- end }}

{{/*
Controller-Manager component labels
*/}}
{{- define "kertical-manager.controller.componentLabels" -}}
app.kubernetes.io/component: controller-manager
{{- end }}

{{/*
Controller-Manager selector labels
*/}}
{{- define "kertical-manager.controller.selectorLabels" -}}
{{ include "kertical-manager.selectorLabels" . }}
{{ include "kertical-manager.controller.componentLabels" . }}
{{- end }}

{{/*
Controller-Manager selector labels
*/}}
{{- define "kertical-manager.controller.labels" -}}
{{ include "kertical-manager.labels" . }}
{{ include "kertical-manager.controller.componentLabels" . }}
{{- end }}

{{/*
Expand the image of the controller-manager.
*/}}
{{- define "kertical-manager.controller.image" -}}
{{ .Values.global.imageRegistry }}/{{ .Values.controller.manager.image.repository }}:{{ .Values.controller.manager.image.tag | default .Chart.AppVersion }}
{{- end }}

