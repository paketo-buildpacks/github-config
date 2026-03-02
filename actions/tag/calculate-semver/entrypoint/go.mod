module github.com/paketo-buildpacks/github-config/actions/tag/calculate-semver-tag/entrypoint

go 1.24.0

toolchain go1.26.0

require (
	github.com/Masterminds/semver/v3 v3.4.0
	github.com/onsi/gomega v1.39.0
	github.com/sclevine/spec v1.4.0
	golang.org/x/oauth2 v0.34.0
)

require (
	github.com/google/go-cmp v0.7.0 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/net v0.43.0 // indirect
	golang.org/x/text v0.28.0 // indirect
)
