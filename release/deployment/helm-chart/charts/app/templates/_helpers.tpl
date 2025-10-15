{{- define "application.name" -}}
    {{ printf "%s" .Chart.Name }}
{{- end -}}

{{- define "secret.name" -}}
    {{ printf "%s-secret" (include "application.name" .) }}
{{- end -}}

{{- define "image.fullname" -}}
    {{- if .Values.custom.image.registry -}}
        {{ printf "%s/%s/%s:%s" .Values.custom.image.registry .Values.image.repository .Values.image.image .Values.image.tag }}
    {{- else if .Values.image.registry -}}
        {{ printf "%s/%s/%s:%s" .Values.image.registry .Values.image.repository .Values.image.image .Values.image.tag }}
    {{- else -}}
        {{ printf "%s/%s:%s" .Values.image.repository .Values.image.image .Values.image.tag }}
    {{- end -}}
{{- end -}}

{{- define "configmap.name" -}}
    {{ printf "%s-configmap" (include "application.name" .) }}
{{- end -}}
