{{/* Shortened name suffixed with upgrade-crd */}}
{{- define "kube-botblocker-operator.cleanupJob.name" -}}
{{- print (include "kube-botblocker-operator.fullname" .) "-cleanup-config" -}}
{{- end -}}

{{- define "kube-botblocker-operator.cleanupJob.labels" -}}
{{- include "kube-botblocker-operator.labels" . }}
app.kubernetes.io/component: cleanup-config
{{- end -}}

{{/* Create the name of cleanupJob service account to use */}}
{{- define "kube-botblocker-operator.cleanupJob.serviceAccountName" -}}
{{- if .Values.cleanupJob.serviceAccount.create -}}
    {{ default (include "kube-botblocker-operator.cleanupJob.name" .) .Values.cleanupJob.serviceAccount.name }}
{{- else -}}
    {{ default "default" .Values.cleanupJob.serviceAccount.name }}
{{- end -}}
{{- end -}}
