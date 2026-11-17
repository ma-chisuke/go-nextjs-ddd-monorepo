{{- /* One finding per line: where, which rule, how bad, and how to fix it. */ -}}
{{- range . }}{{ $target := .Target }}
{{- range .Misconfigurations }}{{ $target }}:{{ .CauseMetadata.StartLine }}: [{{ .ID }}] {{ .Severity }} {{ .Title }}. Fix: {{ .Resolution }} ({{ .PrimaryURL }})
{{ end }}
{{- range .Vulnerabilities }}{{ $target }}: [{{ .VulnerabilityID }}] {{ .Severity }} {{ .PkgName }} {{ .InstalledVersion }}. Fix: upgrade to {{ if .FixedVersion }}{{ .FixedVersion }}{{ else }}a version without the vulnerability (none released yet){{ end }} ({{ .PrimaryURL }})
{{ end }}
{{- range .Licenses }}{{ $target }}: [license] {{ .Severity }} {{ .PkgName }} is under {{ .Name }}, which trivy.yaml forbids. Fix: replace the dependency (docs/adr/template/TPL-0019)
{{ end }}
{{- end }}
