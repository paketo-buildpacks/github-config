{{- if not .BuildImage -}}
## Image - `{{- .RunImage -}}`
{{- else }}
## Images
Build: `{{- .BuildImage -}}`
Run: `{{- .RunImage -}}`
{{- end }}

{{- if .SupportsUsns }}

## Patched USNs
{{- if ne (len .PatchedArray) 0 }}
{{ range .PatchedArray }}
- [{{- .Title -}}]({{- .URL -}})
{{- end }}
{{- else }}
No USNs patched in this release.
{{- end }}
{{- end }}

{{- if .BuildImage}}

## Build Image Package Changes
### Added
{{- if and (gt (len .BuildAdded) 0) (lt (len .BuildAdded) .ReceiptsShowLimit) }}
```
{{- range .BuildAdded }}
{{ .Name }} {{ .Version }} (PURL: {{ .PURL }})
{{- end }}
```
{{- else if and (gt (len .BuildAdded) 0) (gt (len .BuildAdded) .ReceiptsShowLimit) }}
```
❌ TOO large to include
```
{{- else }}
No packages added.
{{- end }}

### Modified
{{- if and (gt (len .BuildModified) 0) (lt (len .BuildModified) .ReceiptsShowLimit) }}
```
{{- range .BuildModified }}
{{ .Name }} {{ .PreviousVersion }} ==> {{ .CurrentVersion }} (PURL: {{ .PreviousPURL }} ==> {{ .CurrentPURL }})
{{- end }}
```
{{- else if and (gt (len .BuildModified) 0) (gt (len .BuildModified) .ReceiptsShowLimit) }}
```
❌ TOO large to include
```
{{- else }}
No packages modified.
{{- end }}

### Removed
{{- if and (gt (len .BuildRemoved) 0) (lt (len .BuildRemoved) .ReceiptsShowLimit) }}
```
{{- range .BuildRemoved }}
{{ .Name }} {{ .Version }} (PURL: {{ .PURL }})
{{- end }}
```
{{- else if and (gt (len .BuildRemoved) 0) (gt (len .BuildRemoved) .ReceiptsShowLimit) }}
```
❌ TOO large to include
```
{{- else }}
No packages removed.
{{- end }}
{{- end }}

## Run Image Package Changes
### Added
{{- if and (gt (len .RunAdded) 0) (lt (len .RunAdded) .ReceiptsShowLimit) }}
```
{{- range .RunAdded }}
{{ .Name }} {{ .Version }} (PURL: {{ .PURL }})
{{- end }}
```
{{- else if and (gt (len .RunAdded) 0) (gt (len .RunAdded) .ReceiptsShowLimit) }}
```
❌ TOO large to include
```
{{- else }}
No packages added.
{{- end }}

### Modified
{{- if and (gt (len .RunModified) 0) (lt (len .RunModified) .ReceiptsShowLimit) }}
```
{{- range .RunModified }}
{{ .Name }} {{ .PreviousVersion }} ==> {{ .CurrentVersion }} (PURL: {{ .PreviousPURL }} ==> {{ .CurrentPURL }})
{{- end }}
```
{{- else if and (gt (len .RunModified) 0) (gt (len .RunModified) .ReceiptsShowLimit) }}
```
❌ TOO large to include
```
{{- else }}
No packages modified.
{{- end }}

### Removed
{{- if and (gt (len .RunRemoved) 0) (lt (len .RunRemoved) .ReceiptsShowLimit) }}
```
{{- range .RunRemoved }}
{{ .Name }} {{ .Version }} (PURL: {{ .PURL }})
{{- end }}
```
{{- else if and (gt (len .RunRemoved) 0) (gt (len .RunRemoved) .ReceiptsShowLimit) }}
```
❌ TOO large to include
```
{{- else }}
No packages removed.
{{- end }}

{{- if or .BuildCveReport .RunCveReport}}
## Known CVEs
This section lists known CVEs of Critical, High and Unknown severity.

{{if .BuildCveReport }}
### Build Image
<details>
<summary>Table</summary>
{{.BuildCveReport}}
</details>
{{- end }}

{{if .RunCveReport }}
### Run Image
<details>
<summary>Table</summary>
{{.RunCveReport}}
</details>
{{- end }}
{{- end }}
