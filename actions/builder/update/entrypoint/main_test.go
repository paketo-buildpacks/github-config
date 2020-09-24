package main_test

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
	. "github.com/paketo-buildpacks/occam/matchers"
)

func TestEntrypoint(t *testing.T) {
	var Expect = NewWithT(t).Expect

	SetDefaultEventuallyTimeout(5 * time.Second)

	var err error
	entrypoint, err := gexec.Build("github.com/paketo-buildpacks/github-config/actions/builder/update/entrypoint")
	Expect(err).NotTo(HaveOccurred())

	spec.Run(t, "actions/builder/update", func(t *testing.T, context spec.G, it spec.S) {
		var (
			Expect     = NewWithT(t).Expect
			Eventually = NewWithT(t).Eventually

			mockRegistryServer    *httptest.Server
			mockRegistryServerURI string
			builderTomlContents   string
			builderToml           *os.File
		)

		it.Before(func() {
			mockRegistryServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				if req.Method == http.MethodHead {
					http.Error(w, "NotFound", http.StatusNotFound)

					return
				}

				switch req.URL.Path {
				case "/v2/":
					w.WriteHeader(http.StatusOK)
				case "/v2/some-repo/some-buildpack-1/tags/list":
					w.WriteHeader(http.StatusOK)
					fmt.Fprintln(w, `{
						  "tags": [
								"0.0.10",
								"0.20.1",
								"0.20.12",
								"latest"
							]
					}`)
				case "/v2/some-repo/some-buildpack-2/tags/list":
					w.WriteHeader(http.StatusOK)
					fmt.Fprintln(w, `{
						  "tags": [
								"0.0.10",
								"0.1.0",
								"0.20.2",
								"0.20.22"
							]
					}`)
				case "/v2/another-repo/some-build-image/tags/list":
					w.WriteHeader(http.StatusOK)
					fmt.Fprintln(w, `{
						  "tags": [
								"0.1.0",
								"0.20.2-some-stack-image-tag",
								"0.20.3-some-stack-image-tag",
								"0.40.2-different-stack-image-tag"
							]
					}`)

				default:
					t.Fatal(fmt.Sprintf("unknown path: %s", req.URL.Path))
				}
			}))
			uri, err := url.Parse(mockRegistryServer.URL)
			Expect(err).NotTo(HaveOccurred())
			mockRegistryServerURI = uri.Host
			builderToml, err = ioutil.TempFile("", "builder.toml")
			Expect(err).NotTo(HaveOccurred())

			builderTomlContents = strings.ReplaceAll(`
description = "Some Description"

[[buildpacks]]
image = "gcr.io/some-repo/some-buildpack-1"
version = "0.20.1"

[[buildpacks]]
image = "gcr.io/some-repo/some-buildpack-2"
version = "0.20.2"

[lifecycle]
version = "0.9.1"

[[order]]
  [[order.group]]
  id = "some-repo/some-buildpack-2"
	version = "0.20.2"

[[order]]
  [[order.group]]
  id = "some-repo/some-buildpack-1"
	version = "0.20.1"

[stack]
id = "some-stack-id"
build-image = "gcr.io/another-repo/some-build-image:19.9-some-stack-image-tag"
run-image = "gcr.io/another-repo/some-run-image:some-stack-image-tag"
run-image-mirrors = ["gcr.io/another-repo/mirror-run:some-stack-image-tag"]
`, "gcr.io", mockRegistryServerURI)

			_, err = builderToml.WriteString(builderTomlContents)
			Expect(err).NotTo(HaveOccurred())

			Expect(builderToml.Close()).To(Succeed())
		})

		it.After(func() {
			mockRegistryServer.Close()
		})

		context("given valid arguments", func() {
			it("creates a valid builder.toml", func() {
				command := exec.Command(
					entrypoint,
					"--builder-file", builderToml.Name(),
					"--registry-server", mockRegistryServerURI,
				)

				buffer := gbytes.NewBuffer()
				session, err := gexec.Start(command, buffer, buffer)
				Expect(err).NotTo(HaveOccurred())

				Eventually(session).Should(gexec.Exit(0), func() string { return string(buffer.Contents()) })

				// Reversing the output due to github action output multiline issue
				out := string(buffer.Contents())
				out = strings.ReplaceAll(out, "%25", "%")
				out = strings.ReplaceAll(out, "%0A", "\n")
				out = strings.ReplaceAll(out, "%0D", "\r")
				Expect(out).To(ContainLines(
					`::set-output name=builder_toml::description = "Some Description"`,
					``,
					`[[buildpacks]]`,
					fmt.Sprintf(`  image = "%s/some-repo/some-buildpack-1:0.20.12"`, mockRegistryServerURI),
					`  version = "0.20.12"`,
					``,
					`[[buildpacks]]`,
					fmt.Sprintf(`  image = "%s/some-repo/some-buildpack-2:0.20.22"`, mockRegistryServerURI),
					`  version = "0.20.22"`,
					``,
					`[lifecycle]`,
					MatchRegexp(`  version = "\d+\.\d+\.\d+"`),
					``,
					`[[order]]`,
					``,
					`  [[order.group]]`,
					`    id = "some-repo/some-buildpack-2"`,
					`    version = "0.20.22"`,
					``,
					`[[order]]`,
					``,
					`  [[order.group]]`,
					`    id = "some-repo/some-buildpack-1"`,
					`    version = "0.20.12"`,
					``,
					`[stack]`,
					`  id = "some-stack-id"`,
					fmt.Sprintf(`  build-image = "%s/another-repo/some-build-image:0.20.3-some-stack-image-tag"`, mockRegistryServerURI),
					fmt.Sprintf(`  run-image = "%s/another-repo/some-run-image:some-stack-image-tag"`, mockRegistryServerURI),
					fmt.Sprintf(`  run-image-mirrors = ["%s/another-repo/mirror-run:some-stack-image-tag"]`, mockRegistryServerURI),
				))
			})
		})

		context("given invalid arguments", func() {
			context("invalid build image", func() {
				context("malformed", func() {
					it("should fail with an appropriate error message", func() {
						malformedToml := strings.ReplaceAll(builderTomlContents, fmt.Sprintf(`build-image = "%s/another-repo/some-build-image:19.9-some-stack-image-tag"`, mockRegistryServerURI), `build-image = "%%%"`)

						err := ioutil.WriteFile(builderToml.Name(), []byte(malformedToml), 0600)
						Expect(err).NotTo(HaveOccurred())

						command := exec.Command(
							entrypoint,
							"--builder-file", builderToml.Name(),
							"--registry-server", mockRegistryServerURI,
						)

						buffer := gbytes.NewBuffer()
						session, err := gexec.Start(command, buffer, buffer)
						Expect(err).NotTo(HaveOccurred())
						Eventually(session).Should(gexec.Exit(1), func() string { return string(buffer.Contents()) })
						Expect(string(buffer.Contents())).To(ContainSubstring(`invalid build image reference "%%%": failed to parse reference "%%%": invalid reference format`))
					})
				})
			})

			context("invalid run image", func() {
				context("malformed", func() {
					it("should fail with an appropriate error message", func() {
						malformedToml := strings.ReplaceAll(builderTomlContents, fmt.Sprintf(`run-image = "%s/another-repo/some-run-image:some-stack-image-tag"`, mockRegistryServerURI), `run-image = "%%%"`)
						err := ioutil.WriteFile(builderToml.Name(), []byte(malformedToml), 0600)
						Expect(err).NotTo(HaveOccurred())
						command := exec.Command(
							entrypoint,
							"--builder-file", builderToml.Name(),
							"--registry-server", mockRegistryServerURI,
						)

						buffer := gbytes.NewBuffer()
						session, err := gexec.Start(command, buffer, buffer)
						Expect(err).NotTo(HaveOccurred())
						Eventually(session).Should(gexec.Exit(1), func() string { return string(buffer.Contents()) })
						Expect(string(buffer.Contents())).To(ContainSubstring(`invalid run image reference "%%%": failed to parse reference "%%%": invalid reference format`))
					})
				})
			})

			context("an invalid run image mirror", func() {
				context("malformed", func() {
					it("should fail with an appropriate error message", func() {
						malformedToml := strings.ReplaceAll(builderTomlContents, fmt.Sprintf(`run-image-mirrors = ["%s/another-repo/mirror-run:some-stack-image-tag"]`, mockRegistryServerURI), `run-image-mirrors = ["%%%"]`)
						err := ioutil.WriteFile(builderToml.Name(), []byte(malformedToml), 0600)
						Expect(err).NotTo(HaveOccurred())
						command := exec.Command(
							entrypoint,
							"--builder-file", builderToml.Name(),
							"--registry-server", mockRegistryServerURI,
						)

						buffer := gbytes.NewBuffer()
						session, err := gexec.Start(command, buffer, buffer)
						Expect(err).NotTo(HaveOccurred())
						Eventually(session).Should(gexec.Exit(1), func() string { return string(buffer.Contents()) })
						Expect(string(buffer.Contents())).To(ContainSubstring(`invalid run-image mirror "%%%": failed to parse reference "%%%": invalid reference format`))
					})
				})
			})

			context("invalid builder toml", func() {
				it("should fail with an appropriate error message", func() {
					invalidBuilderToml, err := ioutil.TempFile("", "builder.toml")
					Expect(err).NotTo(HaveOccurred())

					_, err = invalidBuilderToml.WriteString("\n    ivalid-\n   toml\n   ")
					Expect(err).NotTo(HaveOccurred())
					Expect(invalidBuilderToml.Close()).To(Succeed())
					command := exec.Command(
						entrypoint,
						"--builder-file", invalidBuilderToml.Name(),
						"--registry-server", mockRegistryServerURI,
					)

					buffer := gbytes.NewBuffer()
					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())
					Eventually(session).Should(gexec.Exit(1), func() string { return string(buffer.Contents()) })
					Expect(string(buffer.Contents())).To(ContainSubstring("invalid builder toml"))
				})
			})

			context("invalid path to builder.toml", func() {
				it("should fail with an appropriate error message", func() {
					command := exec.Command(
						entrypoint,
						"--builder-file", "nonexistent/path",
						"--registry-server", mockRegistryServerURI,
					)

					buffer := gbytes.NewBuffer()
					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())
					Eventually(session).Should(gexec.Exit(1), func() string { return string(buffer.Contents()) })
					Expect(string(buffer.Contents())).To(ContainSubstring("invalid path to builder.toml (nonexistent/path)"))
				})
			})

			context("invalid registry server", func() {
				it("should fail with an appropriate error message", func() {
					command := exec.Command(
						entrypoint,
						"--builder-file", builderToml.Name(),
						"--registry-server", "invalid-server-uri",
					)

					buffer := gbytes.NewBuffer()
					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())
					Eventually(session).Should(gexec.Exit(1), func() string { return string(buffer.Contents()) })
					Expect(string(buffer.Contents())).To(ContainSubstring("Error: unable to list image (some-repo/some-buildpack-2) from domain (invalid-server-uri)"))
				})
			})
		})
	})
}
