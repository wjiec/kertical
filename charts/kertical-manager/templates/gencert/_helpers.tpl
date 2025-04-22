{{/*
Init hook annotations
*/}}
{{- define "kertical-manager.gencert.annotations" -}}
helm.sh/hook: pre-install,pre-upgrade
helm.sh/hook-delete-policy: before-hook-creation,hook-succeeded
{{- end }}

{{/*
Expand the name of the gencert.
*/}}
{{- define "kertical-manager.gencert.name" -}}
{{ include "kertical-manager.fullname" . }}-init
{{- end }}

{{/*
Expand the image of the gencert.
*/}}
{{- define "kertical-manager.gencert.image" -}}
{{ .Values.global.imageRegistry }}/{{ .Values.gencert.image.repository }}:{{ .Values.gencert.image.tag | default .Chart.AppVersion }}
{{- end }}
