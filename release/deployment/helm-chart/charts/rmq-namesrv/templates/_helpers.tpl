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

{{- define "bootstrap.path" -}}
    {{ printf "/%s/bootstrap" (include "application.name" .) }}
{{- end -}}

{{- define "entrypoint.path" -}}
    {{ printf "%s/entrypoint.sh" (include "bootstrap.path" .) }}
{{- end -}}

{{- define "healthcheck.path" -}}
    {{ printf "%s/healthcheck.sh" (include "bootstrap.path" .) }}
{{- end -}}