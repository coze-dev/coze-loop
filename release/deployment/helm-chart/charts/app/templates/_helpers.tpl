{{- define "application.name" -}}
    {{ printf "%s" .Chart.Name }}
{{- end -}}

{{- define "secret.name" -}}
    {{ printf "%s-secret" (include "application.name" .) }}
{{- end -}}

{{- define "image.fullname" -}}
    {{ printf "%s/%s/%s:%s" (.Values.custom.image.registry | default .Values.image.registry) .Values.image.repository .Values.image.image .Values.image.tag }}
{{- end -}}

{{- define "configmap.name" -}}
    {{ printf "%s-configmap" (include "application.name" .) }}
{{- end -}}