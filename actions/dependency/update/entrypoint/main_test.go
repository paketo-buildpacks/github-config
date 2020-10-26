package main_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"

	. "github.com/onsi/gomega"
	. "github.com/paketo-buildpacks/packit/matchers"
)

var entrypoint string

func TestEntrypoint(t *testing.T) {
	var Expect = NewWithT(t).Expect

	SetDefaultEventuallyTimeout(5 * time.Second)

	var err error
	entrypoint, err = gexec.Build("github.com/paketo-buildpacks/github-config/actions/dependency/update/entrypoint")
	Expect(err).NotTo(HaveOccurred())

	spec.Run(t, "dependency/update", func(t *testing.T, context spec.G, it spec.S) {
		var (
			Expect     = NewWithT(t).Expect
			Eventually = NewWithT(t).Eventually

			eventPath     string
			workspacePath string
		)

		it.Before(func() {
			file, err := ioutil.TempFile("", "event.json")
			Expect(err).NotTo(HaveOccurred())

			_, err = file.WriteString(`{
				"client_payload": {
					"dependency": {
						"id": "some-registry/some-dependency-id",
						"sha256": "some-updated-sha256",
						"source": "some-updated-source",
						"source_sha256": "some-updated-source-sha256",
						"stacks": ["some-updated-stack"],
						"uri": "some-updated-uri",
						"version": "some-updated-version"
					},
					"strategy": "replace"
				}
			}`)
			Expect(err).NotTo(HaveOccurred())

			Expect(file.Close()).To(Succeed())

			eventPath = file.Name()

			workspacePath, err = ioutil.TempDir("", "workspace")
			Expect(err).NotTo(HaveOccurred())

			file, err = os.Create(filepath.Join(workspacePath, "buildpack.toml"))
			Expect(err).NotTo(HaveOccurred())

			_, err = file.WriteString(`api = "0.2"
[buildpack]
  id = "some-buildpack"
	name = "Some Buildpack"
	version = "some-buildpack-version"

[metadata]
  include-files = ["buildpack.toml"]

	[[metadata.dependencies]]
	  id = "some-registry/some-dependency-id-2"
		sha256 = "other-sha256"
		source = "other-source"
		source_sha256 = "other-source-sha256"
		stacks = ["other-stack"]
		uri = "other-uri"
		version = "other-version"

	[[metadata.dependencies]]
		id = "some-registry/some-dependency-id"
		sha256 = "some-sha256"
		source = "some-source"
		source_sha256 = "some-source-sha256"
		stacks = ["some-stack"]
		uri = "some-uri"
		version = "some-version"

[[order]]
  [[order.group]]
	  id = "some-registry/some-dependency-id"
		version = "some-version"

[[order]]
  [[order.group]]
		id = "some-registry/some-dependency-id-2"
		version = "other-version"
		optional = true
`)
			Expect(err).NotTo(HaveOccurred())

			Expect(file.Close()).To(Succeed())
		})

		it.After(func() {
			Expect(os.RemoveAll(eventPath)).To(Succeed())
			Expect(os.RemoveAll(workspacePath)).To(Succeed())
		})

		context("when there is NOT a package.toml file", func() {
			it("outputs the dependency details of a release", func() {
				command := exec.Command(entrypoint, "--workspace-path", workspacePath)
				command.Env = append(command.Env, fmt.Sprintf("GITHUB_EVENT_PATH=%s", eventPath))
				buffer := gbytes.NewBuffer()

				session, err := gexec.Start(command, buffer, buffer)
				Expect(err).NotTo(HaveOccurred())

				Eventually(session).Should(gexec.Exit(0), func() string { return fmt.Sprintf("output:\n%s\n", buffer.Contents()) })

				Expect(buffer).To(gbytes.Say(`Parsing dispatch event`))
				Expect(buffer).To(gbytes.Say(`  Dependency: some-registry/some-dependency-id`))
				Expect(buffer).To(gbytes.Say(`  Strategy:   replace`))
				Expect(buffer).To(gbytes.Say(`Updating buildpack.toml`))

				contents, err := ioutil.ReadFile(filepath.Join(workspacePath, "buildpack.toml"))
				Expect(err).NotTo(HaveOccurred())

				Expect(string(contents)).To(MatchTOML(`api = "0.2"

[buildpack]
  id = "some-buildpack"
	name = "Some Buildpack"
	version = "some-buildpack-version"

[metadata]
  include-files = ["buildpack.toml"]

	[[metadata.dependencies]]
	  id = "some-registry/some-dependency-id-2"
		sha256 = "other-sha256"
		source = "other-source"
		source_sha256 = "other-source-sha256"
		stacks = ["other-stack"]
		uri = "other-uri"
		version = "other-version"

	[[metadata.dependencies]]
		id = "some-registry/some-dependency-id"
		sha256 = "some-updated-sha256"
		source = "some-updated-source"
		source_sha256 = "some-updated-source-sha256"
		stacks = ["some-updated-stack"]
		uri = "some-updated-uri"
		version = "some-updated-version"

[[order]]
  [[order.group]]
	  id = "some-registry/some-dependency-id"
		version = "some-updated-version"

[[order]]
  [[order.group]]
		id = "some-registry/some-dependency-id-2"
		version = "other-version"
		optional = true
`))
			})

			context("when the dependency does not exist in the current buildpack.toml", func() {
				it.Before(func() {
					err := ioutil.WriteFile(filepath.Join(workspacePath, "buildpack.toml"), []byte(`api = "0.2"
[buildpack]
  id = "some-buildpack"
	name = "Some Buildpack"
	version = "some-buildpack-version"

[metadata]
  include-files = ["buildpack.toml"]

	[[metadata.dependencies]]
	  id = "some-registry/some-dependency-id-2"
		sha256 = "other-sha256"
		source = "other-source"
		source_sha256 = "other-source-sha256"
		stacks = ["other-stack"]
		uri = "other-uri"
		version = "other-version"

[[order]]
  [[order.group]]
	  id = "some-registry/some-dependency-id"
		version = "some-version"

[[order]]
  [[order.group]]
		id = "some-registry/some-dependency-id-2"
		version = "other-version"
		optional = true
`), 0644)
					Expect(err).NotTo(HaveOccurred())
				})

				it("outputs the dependency details of a release", func() {
					command := exec.Command(entrypoint, "--workspace-path", workspacePath)
					command.Env = append(command.Env, fmt.Sprintf("GITHUB_EVENT_PATH=%s", eventPath))
					buffer := gbytes.NewBuffer()

					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())

					Eventually(session).Should(gexec.Exit(0), func() string { return fmt.Sprintf("output:\n%s\n", buffer.Contents()) })

					contents, err := ioutil.ReadFile(filepath.Join(workspacePath, "buildpack.toml"))
					Expect(err).NotTo(HaveOccurred())

					Expect(string(contents)).To(MatchTOML(`api = "0.2"

[buildpack]
  id = "some-buildpack"
	name = "Some Buildpack"
	version = "some-buildpack-version"

[metadata]
  include-files = ["buildpack.toml"]

	[[metadata.dependencies]]
	  id = "some-registry/some-dependency-id-2"
		sha256 = "other-sha256"
		source = "other-source"
		source_sha256 = "other-source-sha256"
		stacks = ["other-stack"]
		uri = "other-uri"
		version = "other-version"

	[[metadata.dependencies]]
		id = "some-registry/some-dependency-id"
		sha256 = "some-updated-sha256"
		source = "some-updated-source"
		source_sha256 = "some-updated-source-sha256"
		stacks = ["some-updated-stack"]
		uri = "some-updated-uri"
		version = "some-updated-version"

[[order]]
  [[order.group]]
	  id = "some-registry/some-dependency-id"
		version = "some-updated-version"

[[order]]
  [[order.group]]
		id = "some-registry/some-dependency-id-2"
		version = "other-version"
		optional = true
`))
				})
			})
		})

		context("when there is a package.toml file", func() {
			it.Before(func() {
				err := ioutil.WriteFile(filepath.Join(workspacePath, "buildpack.toml"), []byte(`api = "0.2"
[buildpack]
  id = "some-buildpack"
	name = "Some Buildpack"
	version = "some-buildpack-version"

[metadata]
  include-files = ["buildpack.toml"]

[[order]]
  [[order.group]]
	  id = "some-registry/some-dependency-id"
		version = "some-version"

[[order]]
  [[order.group]]
		id = "some-registry/some-dependency-id-2"
		version = "other-version"
		optional = true
`), 0644)
				Expect(err).NotTo(HaveOccurred())

				err = ioutil.WriteFile(filepath.Join(workspacePath, "package.toml"), []byte(`
[buildpack]
uri = "build/buildpack.tgz"

[[dependencies]]
image = "gcr.io/some-registry/some-dependency-id:some-version"

[[dependencies]]
image = "gcr.io/some-registry/some-dependency-id-2:other-version"
`), 0644)
				Expect(err).NotTo(HaveOccurred())
			})

			it("updates both the package.toml and buildpack.toml files", func() {
				command := exec.Command(entrypoint, "--workspace-path", workspacePath)
				command.Env = append(command.Env, fmt.Sprintf("GITHUB_EVENT_PATH=%s", eventPath))
				buffer := gbytes.NewBuffer()

				session, err := gexec.Start(command, buffer, buffer)
				Expect(err).NotTo(HaveOccurred())

				Eventually(session).Should(gexec.Exit(0), func() string { return fmt.Sprintf("output:\n%s\n", buffer.Contents()) })

				Expect(buffer).To(gbytes.Say(`Parsing dispatch event`))
				Expect(buffer).To(gbytes.Say(`  Dependency: some-registry/some-dependency-id`))
				Expect(buffer).To(gbytes.Say(`  Strategy:   replace`))
				Expect(buffer).To(gbytes.Say(`Updating buildpack.toml`))
				Expect(buffer).To(gbytes.Say(`Updating package.toml`))

				bpTOMLContents, err := ioutil.ReadFile(filepath.Join(workspacePath, "buildpack.toml"))
				Expect(err).NotTo(HaveOccurred())

				Expect(string(bpTOMLContents)).To(MatchTOML(`api = "0.2"

[buildpack]
  id = "some-buildpack"
  name = "Some Buildpack"
  version = "some-buildpack-version"

[metadata]
  include-files = ["buildpack.toml"]

[[order]]
  [[order.group]]
    id = "some-registry/some-dependency-id"
    version = "some-updated-version"

[[order]]
  [[order.group]]
		id = "some-registry/some-dependency-id-2"
    version = "other-version"
    optional = true
`))
				packageTOMLContents, err := ioutil.ReadFile(filepath.Join(workspacePath, "package.toml"))
				Expect(err).NotTo(HaveOccurred())

				Expect(string(packageTOMLContents)).To(MatchTOML(`
[buildpack]
  uri = "build/buildpack.tgz"

[[dependencies]]
  image = "gcr.io/some-registry/some-dependency-id:some-updated-version"

[[dependencies]]
  image = "gcr.io/some-registry/some-dependency-id-2:other-version"
`))
			})
		})

		context("failure cases", func() {
			context("when the event file does not exist", func() {
				it.Before(func() {
					Expect(os.Remove(eventPath)).To(Succeed())
				})

				it("returns an error", func() {
					command := exec.Command(entrypoint, "--workspace-path", workspacePath)
					command.Env = append(command.Env, fmt.Sprintf("GITHUB_EVENT_PATH=%s", eventPath))
					buffer := gbytes.NewBuffer()

					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())

					Eventually(session).Should(gexec.Exit(1), func() string { return fmt.Sprintf("output:\n%s\n", buffer.Contents()) })

					Expect(buffer).To(gbytes.Say(`Error: failed to read \$GITHUB_EVENT_PATH:`))
					Expect(buffer).To(gbytes.Say(`no such file or directory`))
				})
			})

			context("when the event file is not parsable", func() {
				it.Before(func() {
					err := ioutil.WriteFile(eventPath, []byte(`%%%`), 0644)
					Expect(err).NotTo(HaveOccurred())
				})

				it("returns an error", func() {
					command := exec.Command(entrypoint, "--workspace-path", workspacePath)
					command.Env = append(command.Env, fmt.Sprintf("GITHUB_EVENT_PATH=%s", eventPath))
					buffer := gbytes.NewBuffer()

					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())

					Eventually(session).Should(gexec.Exit(1), func() string { return fmt.Sprintf("output:\n%s\n", buffer.Contents()) })

					Expect(buffer).To(gbytes.Say(`Error: failed to decode \$GITHUB_EVENT_PATH:`))
					Expect(buffer).To(gbytes.Say(`invalid character '%' looking for beginning of value`))
				})
			})

			context("when the buildpack.toml file does not exist", func() {
				it.Before(func() {
					Expect(os.Remove(filepath.Join(workspacePath, "buildpack.toml"))).To(Succeed())
				})

				it("returns an error", func() {
					command := exec.Command(entrypoint, "--workspace-path", workspacePath)
					command.Env = append(command.Env, fmt.Sprintf("GITHUB_EVENT_PATH=%s", eventPath))
					buffer := gbytes.NewBuffer()

					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())

					Eventually(session).Should(gexec.Exit(1), func() string { return fmt.Sprintf("output:\n%s\n", buffer.Contents()) })

					Expect(buffer).To(gbytes.Say(`Error: failed to read buildpack.toml:`))
					Expect(buffer).To(gbytes.Say(`no such file or directory`))
				})
			})

			context("when the buildpack.toml file is not parseable", func() {
				it.Before(func() {
					err := ioutil.WriteFile(filepath.Join(workspacePath, "buildpack.toml"), []byte(`%%%`), 0644)
					Expect(err).NotTo(HaveOccurred())
				})

				it("returns an error", func() {
					command := exec.Command(entrypoint, "--workspace-path", workspacePath)
					command.Env = append(command.Env, fmt.Sprintf("GITHUB_EVENT_PATH=%s", eventPath))
					buffer := gbytes.NewBuffer()

					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())

					Eventually(session).Should(gexec.Exit(1), func() string { return fmt.Sprintf("output:\n%s\n", buffer.Contents()) })

					Expect(buffer).To(gbytes.Say(`Error: failed to decode buildpack.toml:`))
					Expect(buffer).To(gbytes.Say(`bare keys cannot contain '%'`))
				})
			})
			context("when the package.toml file is not readable", func() {
				it.Before(func() {
					Expect(ioutil.WriteFile(filepath.Join(workspacePath, "package.toml"), []byte(``), os.ModePerm)).To(Succeed())

					Expect(os.Chmod(filepath.Join(workspacePath, "package.toml"), 0000)).To(Succeed())
				})

				it("returns an error", func() {
					command := exec.Command(entrypoint, "--workspace-path", workspacePath)
					command.Env = append(command.Env, fmt.Sprintf("GITHUB_EVENT_PATH=%s", eventPath))
					buffer := gbytes.NewBuffer()

					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())

					Eventually(session).Should(gexec.Exit(1), func() string { return fmt.Sprintf("output:\n%s\n", buffer.Contents()) })

					Expect(buffer).To(gbytes.Say(`Error: failed to read package.toml:`))
				})
			})

			context("when the package.toml file is malformed", func() {
				it.Before(func() {
					Expect(ioutil.WriteFile(filepath.Join(workspacePath, "package.toml"), []byte(`%%%`), os.ModePerm)).To(Succeed())
				})

				it("returns an error", func() {
					command := exec.Command(entrypoint, "--workspace-path", workspacePath)
					command.Env = append(command.Env, fmt.Sprintf("GITHUB_EVENT_PATH=%s", eventPath))
					buffer := gbytes.NewBuffer()

					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())

					Eventually(session).Should(gexec.Exit(1), func() string { return fmt.Sprintf("output:\n%s\n", buffer.Contents()) })

					Expect(buffer).To(gbytes.Say(`Error: failed to decode package.toml:`))
				})
			})

			context("when the update strategy is unknown", func() {
				it.Before(func() {
					err := ioutil.WriteFile(eventPath, []byte(`{
						"client_payload": {
							"dependency": {
								"id": "some-registry/some-dependency-id",
								"sha256": "some-updated-sha256",
								"source": "some-updated-source",
								"source_sha256": "some-updated-source-sha256",
								"stacks": ["some-updated-stack"],
								"uri": "some-updated-uri",
								"version": "some-updated-version"
							},
							"strategy": "unknown-strategy"
						}
					}`), 0644)
					Expect(err).NotTo(HaveOccurred())
				})

				it("returns an error", func() {
					command := exec.Command(entrypoint, "--workspace-path", workspacePath)
					command.Env = append(command.Env, fmt.Sprintf("GITHUB_EVENT_PATH=%s", eventPath))
					buffer := gbytes.NewBuffer()

					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())

					Eventually(session).Should(gexec.Exit(1), func() string { return fmt.Sprintf("output:\n%s\n", buffer.Contents()) })

					Expect(buffer).To(gbytes.Say(`Error: unknown update strategy "unknown-strategy"`))
				})
			})

			context("when the buildpack.toml cannot be written", func() {
				it.Before(func() {
					Expect(os.Chmod(filepath.Join(workspacePath, "buildpack.toml"), 0444)).To(Succeed())
				})

				it("returns an error", func() {
					command := exec.Command(entrypoint, "--workspace-path", workspacePath)
					command.Env = append(command.Env, fmt.Sprintf("GITHUB_EVENT_PATH=%s", eventPath))
					buffer := gbytes.NewBuffer()

					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())

					Eventually(session).Should(gexec.Exit(1), func() string { return fmt.Sprintf("output:\n%s\n", buffer.Contents()) })

					Expect(buffer).To(gbytes.Say(`Error: failed to write buildpack.toml:`))
					Expect(buffer).To(gbytes.Say(`permission denied`))
				})
			})

			context("when the package.toml cannot be written", func() {
				it.Before(func() {
					Expect(ioutil.WriteFile(filepath.Join(workspacePath, "package.toml"), []byte(``), os.ModePerm)).To(Succeed())

					Expect(os.Chmod(filepath.Join(workspacePath, "package.toml"), 0444)).To(Succeed())
				})

				it("returns an error", func() {
					command := exec.Command(entrypoint, "--workspace-path", workspacePath)
					command.Env = append(command.Env, fmt.Sprintf("GITHUB_EVENT_PATH=%s", eventPath))
					buffer := gbytes.NewBuffer()

					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())

					Eventually(session).Should(gexec.Exit(1), func() string { return fmt.Sprintf("output:\n%s\n", buffer.Contents()) })

					Expect(buffer).To(gbytes.Say(`Error: failed to write package.toml:`))
					Expect(buffer).To(gbytes.Say(`permission denied`))
				})
			})
		})
	}, spec.Report(report.Terminal{}), spec.Parallel())
}
