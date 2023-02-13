## Images
Build: `{{- .BuildImage -}}`
Run: `{{- .RunImage -}}`

## Patched USNs
{{- if ne (len .PatchedArray) 0 }}
{{ range .PatchedArray }}
- [{{- .Title -}}]({{- .URL -}})
{{- end }}
{{- else }}
No USNs patched in this release.
{{- end }}

## Build Image Package Changes
### Added
{{- if ne (len .BuildAdded) 0 }}
```
{{- range .BuildAdded }}
{{ .Name }} {{ .Version }} (PURL: {{ .PURL }})
{{- end }}
```
{{- else }}
No packages added.
{{- end }}

### Modified
{{- if ne (len .BuildModified) 0 }}
```
{{- range .BuildModified }}
{{ .Name }} {{ .PreviousVersion }} ==> {{ .CurrentVersion }} (PURL: {{ .PreviousPURL }} ==> {{ .CurrentPURL }})
{{- end }}
```
{{- else }}
No packages modified.
{{- end }}

### Removed
{{- if ne (len .BuildRemoved) 0 }}
```
{{- range .BuildRemoved }}
{{ .Name }} {{ .Version }} (PURL: {{ .PURL }})
{{- end }}
```
{{- else }}
No packages removed.
{{- end }}

## Run Image Package Changes
### Added
{{- if ne (len .RunAdded) 0 }}
```
{{- range .RunAdded }}
{{ .Name }} {{ .Version }} (PURL: {{ .PURL }})
{{- end }}
```
{{- else }}
No packages added.
{{- end }}

### Modified
{{- if ne (len .RunModified) 0 }}
```
{{- range .RunModified }}
{{ .Name }} {{ .PreviousVersion }} ==> {{ .CurrentVersion }} (PURL: {{ .PreviousPURL }} ==> {{ .CurrentPURL }})
{{- end }}
```
{{- else }}
No packages modified.
{{- end }}

### Removed
{{- if ne (len .RunRemoved) 0 }}
```
{{- range .RunRemoved }}
{{ .Name }} {{ .Version }} (PURL: {{ .PURL }})
{{- end }}
```
{{- else }}
No packages removed.
{{- end }}

## Known CVEs
This section lists known CVEs of Critical, High and Unknown severity.

### Build Image
{{- if .BuildCveReport }}
<details>
<summary>Table</summary>
{{.BuildCveReport}}
</details>
{{- else }}
No known CVEs.
{{- end }}

### Run Image
{{- if .RunCveReport }}
<details>
<summary>Table</summary>
{{.RunCveReport}}
</details>
{{- else }}
No known CVEs.
{{- end }}
