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

	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	. "github.com/paketo-buildpacks/occam/matchers"
)

func TestEntrypoint(t *testing.T) {
	var Expect = NewWithT(t).Expect

	SetDefaultEventuallyTimeout(5 * time.Second)

	var err error
	entrypoint, err := gexec.Build("github.com/paketo-buildpacks/github-config/actions/builder/generate-builder-toml/entrypoint")
	Expect(err).NotTo(HaveOccurred())

	spec.Run(t, "generate-builder-toml", func(t *testing.T, context spec.G, it spec.S) {
		var (
			Expect     = NewWithT(t).Expect
			Eventually = NewWithT(t).Eventually

			mockRegistryServer    *httptest.Server
			mockRegistryServerURI string
			orderToml             *os.File
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
								"0.1.0",
								"0.20.1",
								"latest"
							]
					}`)
				case "/v2/some-repo/some-buildpack-2/tags/list":
					w.WriteHeader(http.StatusOK)
					fmt.Fprintln(w, `{
						  "tags": [
								"0.0.10",
								"0.1.0",
								"0.20.2"
							]
					}`)
				case "/v2/another-repo/some-build-image/tags/list":
					w.WriteHeader(http.StatusOK)
					fmt.Fprintln(w, `{
						  "tags": [
								"0.0.10-some-stack-image-tag",
								"0.1.0",
								"0.20.2-some-stack-image-tag",
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
			orderToml, err = ioutil.TempFile("", "order.toml")
			Expect(err).NotTo(HaveOccurred())

			_, err = orderToml.WriteString(`
description = "Some Description"

[[order]]
group = [
  { id = "some-repo/some-buildpack-2" },
]

[[order]]
group = [
  { id = "some-repo/some-buildpack-1" },
]
`)
			Expect(err).NotTo(HaveOccurred())
			Expect(orderToml.Close()).To(Succeed())
		})

		it.After(func() {
			mockRegistryServer.Close()
		})

		context("given valid arguments", func() {
			it("creates a valid builder.toml", func() {
				command := exec.Command(
					entrypoint,
					"--stack", "some-stack-ID",
					"--build-image", fmt.Sprintf("%s/another-repo/some-build-image", mockRegistryServerURI),
					"--run-image", "some-registry-server/some-run-image",
					"--run-image-mirrors", "some-registry-server/rim1,some-registry-server/rim2",
					"--stack-image-tag", "some-stack-image-tag",
					"--order-file", orderToml.Name(),
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
					fmt.Sprintf(`  image = "%s/some-repo/some-buildpack-1:0.20.1"`, mockRegistryServerURI),
					`  version = "0.20.1"`,
					``,
					`[[buildpacks]]`,
					fmt.Sprintf(`  image = "%s/some-repo/some-buildpack-2:0.20.2"`, mockRegistryServerURI),
					`  version = "0.20.2"`,
					``,
					`[lifecycle]`,
					MatchRegexp(`  version = "\d+\.\d+\.\d+"`),
					``,
					`[[order]]`,
					``,
					`  [[order.group]]`,
					`    id = "some-repo/some-buildpack-2"`,
					`    version = "0.20.2"`,
					``,
					`[[order]]`,
					``,
					`  [[order.group]]`,
					`    id = "some-repo/some-buildpack-1"`,
					`    version = "0.20.1"`,
					``,
					`[stack]`,
					`  id = "some-stack-ID"`,
					fmt.Sprintf(`  build-image = "%s/another-repo/some-build-image:0.20.2-some-stack-image-tag"`, mockRegistryServerURI),
					`  run-image = "some-registry-server/some-run-image:some-stack-image-tag"`,
					`  run-image-mirrors = ["some-registry-server/rim1:some-stack-image-tag", "some-registry-server/rim2:some-stack-image-tag"]`,
				))
			})
		})

		context("given invalid arguments", func() {
			context("empty stack id", func() {
				it("should fail with an appropriate error message", func() {
					command := exec.Command(
						entrypoint,
						"--stack", "",
						"--build-image", fmt.Sprintf("%s/another-repo/some-build-image", mockRegistryServerURI),
						"--run-image", "some-registry-server/some-run-image",
						"--run-image-mirrors", "some-registry-server/rim1,some-registry-server/rim2",
						"--stack-image-tag", "some-stack-image-tag",
						"--order-file", orderToml.Name(),
						"--registry-server", mockRegistryServerURI,
					)

					buffer := gbytes.NewBuffer()
					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())
					Eventually(session).Should(gexec.Exit(1), func() string { return string(buffer.Contents()) })
					Expect(string(buffer.Contents())).To(ContainSubstring(`missing required input "stack"`))
				})
			})

			context("invalid build image", func() {
				context("already contains a tag", func() {
					it("should fail with an appropriate error message", func() {
						command := exec.Command(
							entrypoint,
							"--stack", "some-stack-ID",
							"--build-image", fmt.Sprintf("%s/another-repo/some-build-image:some-tag", mockRegistryServerURI),
							"--run-image", "some-registry-server/some-run-image",
							"--run-image-mirrors", "some-registry-server/rim1,some-registry-server/rim2",
							"--stack-image-tag", "some-stack-image-tag",
							"--order-file", orderToml.Name(),
							"--registry-server", mockRegistryServerURI,
						)

						buffer := gbytes.NewBuffer()
						session, err := gexec.Start(command, buffer, buffer)
						Expect(err).NotTo(HaveOccurred())
						Eventually(session).Should(gexec.Exit(1), func() string { return string(buffer.Contents()) })
						Expect(string(buffer.Contents())).To(ContainSubstring("invalid build image"))
						Expect(string(buffer.Contents())).To(ContainSubstring("image should not contain tag"))
					})
				})

				context("malformed", func() {
					it("should fail with an appropriate error message", func() {
						command := exec.Command(
							entrypoint,
							"--stack", "some-stack-ID",
							"--build-image", "invalid-image",
							"--run-image", "some-registry-server/some-run-image",
							"--run-image-mirrors", "some-registry-server/rim1,some-registry-server/rim2",
							"--stack-image-tag", "some-stack-image-tag",
							"--order-file", orderToml.Name(),
							"--registry-server", mockRegistryServerURI,
						)

						buffer := gbytes.NewBuffer()
						session, err := gexec.Start(command, buffer, buffer)
						Expect(err).NotTo(HaveOccurred())
						Eventually(session).Should(gexec.Exit(1), func() string { return string(buffer.Contents()) })
						Expect(string(buffer.Contents())).To(ContainSubstring("invalid build image"))
						Expect(string(buffer.Contents())).To(ContainSubstring("image should be in form <registry>/<repo>/<image>"))
					})
				})
			})

			context("invalid run image", func() {
				context("already contains a tag", func() {
					it("should fail with an appropriate error message", func() {
						command := exec.Command(
							entrypoint,
							"--stack", "some-stack-ID",
							"--build-image", fmt.Sprintf("%s/another-repo/some-build-image", mockRegistryServerURI),
							"--run-image", fmt.Sprintf("%s/some-run-image:some-tag", mockRegistryServerURI),
							"--run-image-mirrors", "some-registry-server/rim1,some-registry-server/rim2",
							"--stack-image-tag", "some-stack-image-tag",
							"--order-file", orderToml.Name(),
							"--registry-server", mockRegistryServerURI,
						)

						buffer := gbytes.NewBuffer()
						session, err := gexec.Start(command, buffer, buffer)
						Expect(err).NotTo(HaveOccurred())
						Eventually(session).Should(gexec.Exit(1), func() string { return string(buffer.Contents()) })
						Expect(string(buffer.Contents())).To(ContainSubstring("invalid run image"))
						Expect(string(buffer.Contents())).To(ContainSubstring("image should not contain tag"))
					})
				})

				context("malformed", func() {
					it("should fail with an appropriate error message", func() {
						command := exec.Command(
							entrypoint,
							"--stack", "some-stack-ID",
							"--build-image", fmt.Sprintf("%s/another-repo/some-build-image", mockRegistryServerURI),
							"--run-image", "invalid-image",
							"--run-image-mirrors", "some-registry-server/rim1,some-registry-server/rim2",
							"--stack-image-tag", "some-stack-image-tag",
							"--order-file", orderToml.Name(),
							"--registry-server", mockRegistryServerURI,
						)

						buffer := gbytes.NewBuffer()
						session, err := gexec.Start(command, buffer, buffer)
						Expect(err).NotTo(HaveOccurred())
						Eventually(session).Should(gexec.Exit(1), func() string { return string(buffer.Contents()) })
						Expect(string(buffer.Contents())).To(ContainSubstring("invalid run image"))
						Expect(string(buffer.Contents())).To(ContainSubstring("image should be in form <registry>/<repo>/<image>"))
					})
				})
			})

			context("an invalid run image mirror", func() {
				context("already contains a tag", func() {
					it("should fail with an appropriate error message", func() {
						command := exec.Command(
							entrypoint,
							"--stack", "some-stack-ID",
							"--build-image", fmt.Sprintf("%s/another-repo/some-build-image", mockRegistryServerURI),
							"--run-image", fmt.Sprintf("%s/some-run-image", mockRegistryServerURI),
							"--run-image-mirrors", "some-registry-server/rim1:some-tag,some-registry-server/rim2",
							"--stack-image-tag", "some-stack-image-tag",
							"--order-file", orderToml.Name(),
							"--registry-server", mockRegistryServerURI,
						)

						buffer := gbytes.NewBuffer()
						session, err := gexec.Start(command, buffer, buffer)
						Expect(err).NotTo(HaveOccurred())
						Eventually(session).Should(gexec.Exit(1), func() string { return string(buffer.Contents()) })
						Expect(string(buffer.Contents())).To(ContainSubstring("invalid run-image mirror"))
						Expect(string(buffer.Contents())).To(ContainSubstring("image should not contain tag"))
					})
				})

				context("malformed", func() {
					it("should fail with an appropriate error message", func() {
						command := exec.Command(
							entrypoint,
							"--stack", "some-stack-ID",
							"--build-image", fmt.Sprintf("%s/another-repo/some-build-image", mockRegistryServerURI),
							"--run-image", fmt.Sprintf("%s/some-run-image", mockRegistryServerURI),
							"--run-image-mirrors", "invalid-mirror,some-registry-server/rim2",
							"--stack-image-tag", "some-stack-image-tag",
							"--order-file", orderToml.Name(),
							"--registry-server", mockRegistryServerURI,
						)

						buffer := gbytes.NewBuffer()
						session, err := gexec.Start(command, buffer, buffer)
						Expect(err).NotTo(HaveOccurred())
						Eventually(session).Should(gexec.Exit(1), func() string { return string(buffer.Contents()) })
						Expect(string(buffer.Contents())).To(ContainSubstring("invalid run-image mirror"))
						Expect(string(buffer.Contents())).To(ContainSubstring("image should be in form <registry>/<repo>/<image>"))
					})
				})
			})

			context("invalid order toml", func() {
				it("should fail with an appropriate error message", func() {
					invalidOrderToml, err := ioutil.TempFile("", "order.toml")
					Expect(err).NotTo(HaveOccurred())

					_, err = invalidOrderToml.WriteString(`
ivalid-
toml
`)
					Expect(err).NotTo(HaveOccurred())
					Expect(invalidOrderToml.Close()).To(Succeed())
					command := exec.Command(
						entrypoint,
						"--stack", "some-stack-ID",
						"--build-image", fmt.Sprintf("%s/another-repo/some-build-image", mockRegistryServerURI),
						"--run-image", "some-registry-server/some-run-image",
						"--run-image-mirrors", "some-registry-server/rim1,some-registry-server/rim2",
						"--stack-image-tag", "some-stack-image-tag",
						"--order-file", invalidOrderToml.Name(),
						"--registry-server", mockRegistryServerURI,
					)

					buffer := gbytes.NewBuffer()
					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())
					Eventually(session).Should(gexec.Exit(1), func() string { return string(buffer.Contents()) })
					Expect(string(buffer.Contents())).To(ContainSubstring("invalid order toml"))
				})
			})

			context("invalid path to order.toml", func() {
				it("should fail with an appropriate error message", func() {
					command := exec.Command(
						entrypoint,
						"--stack", "some-stack-ID",
						"--build-image", fmt.Sprintf("%s/another-repo/some-build-image", mockRegistryServerURI),
						"--run-image", "some-registry-server/some-run-image",
						"--run-image-mirrors", "some-registry-server/rim1,some-registry-server/rim2",
						"--stack-image-tag", "some-stack-image-tag",
						"--order-file", "nonexistent/path",
						"--registry-server", mockRegistryServerURI,
					)

					buffer := gbytes.NewBuffer()
					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())
					Eventually(session).Should(gexec.Exit(1), func() string { return string(buffer.Contents()) })
					Expect(string(buffer.Contents())).To(ContainSubstring("invalid path to order.toml (nonexistent/path)"))
				})
			})

			context("invalid registry server", func() {
				it("should fail with an appropriate error message", func() {
					command := exec.Command(
						entrypoint,
						"--stack", "some-stack-ID",
						"--build-image", fmt.Sprintf("%s/another-repo/some-build-image", mockRegistryServerURI),
						"--run-image", "some-registry-server/some-run-image",
						"--run-image-mirrors", "some-registry-server/rim1,some-registry-server/rim2",
						"--stack-image-tag", "some-stack-image-tag",
						"--order-file", orderToml.Name(),
						"--registry-server", "invalid-server-uri",
					)

					buffer := gbytes.NewBuffer()
					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())
					Eventually(session).Should(gexec.Exit(1), func() string { return string(buffer.Contents()) })
					Expect(string(buffer.Contents())).To(ContainSubstring("Error: unable to list images from registry server (invalid-server-uri)"))
				})
			})
		})
	})
}
