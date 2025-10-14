module github.com/paketo-buildpacks/github-config/actions/tag/calculate-semver-tag/entrypoint

go 1.24.0

toolchain go1.25.3

require (
	github.com/Masterminds/semver/v3 v3.4.0
	github.com/onsi/gomega v1.38.0
	github.com/sclevine/spec v1.4.0
	golang.org/x/oauth2 v0.31.0
)

require (
	github.com/google/go-cmp v0.7.0 // indirect
	golang.org/x/net v0.41.0 // indirect
	golang.org/x/text v0.26.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
