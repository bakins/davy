{{- define "foo.name" -}}
{{- default .Values.foo "default foo" -}}
{{- end -}}
