package main_test

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"

	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var entrypoint string

func TestEntrypoint(t *testing.T) {
	var Expect = NewWithT(t).Expect

	RegisterTestingT(t)
	SetDefaultEventuallyTimeout(5 * time.Second)

	var err error
	entrypoint, err = gexec.Build("github.com/paketo-buildpacks/github-config/actions/release/notes/language-family/entrypoint")
	Expect(err).NotTo(HaveOccurred())

	spec.Run(t, "create", func(t *testing.T, context spec.G, it spec.S) {
		var (
			Expect = NewWithT(t).Expect

			workspacePath string
			api           *httptest.Server
			requests      []*http.Request
		)

		it.Before(func() {
			requests = []*http.Request{}
			api = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				dump, _ := httputil.DumpRequest(req, true)
				receivedRequest, _ := http.ReadRequest(bufio.NewReader(bytes.NewBuffer(dump)))

				requests = append(requests, receivedRequest)

				switch req.URL.Path {
				case "/some-uri.tgz":
					buf := bytes.NewBuffer(nil)
					zw := gzip.NewWriter(buf)
					tw := tar.NewWriter(zw)

					file := bytes.NewBuffer(nil)
					_, err := file.WriteString(`api = "0.2"
			[buildpack]
			  id = "some-buildpack-id"
				name = "Some Buildpack"
				version = "some-version"

			[[stacks]]
			  id = "some-stack"

			[[stacks]]
			  id = "other-stack"
			`)
					if err != nil {
						t.Fatal(err)
					}

					err = tw.WriteHeader(&tar.Header{
						Name: "buildpack.toml",
						Mode: 0644,
						Size: int64(file.Len()),
					})
					if err != nil {
						t.Fatal(err)
					}

					if _, err := io.Copy(tw, file); err != nil {
						t.Fatal(err)
					}

					if err := tw.Close(); err != nil {
						t.Fatal(err)
					}

					if err := zw.Close(); err != nil {
						t.Fatal(err)
					}

					if _, err := io.Copy(w, buf); err != nil {
						t.Fatal(err)
					}
				case "/some-other-uri.tgz":
					buf := bytes.NewBuffer(nil)
					zw := gzip.NewWriter(buf)
					tw := tar.NewWriter(zw)

					file := bytes.NewBuffer(nil)
					_, err := file.WriteString(`api = "0.2"
			[buildpack]
			  id = "some-other-buildpack-id"
				name = "Some Other Buildpack"
				version = "some-version"

			[[stacks]]
			  id = "some-stack"

			[[stacks]]
			  id = "other-stack"
			`)
					if err != nil {
						t.Fatal(err)
					}

					err = tw.WriteHeader(&tar.Header{
						Name: "buildpack.toml",
						Mode: 0644,
						Size: int64(file.Len()),
					})
					if err != nil {
						t.Fatal(err)
					}

					if _, err := io.Copy(tw, file); err != nil {
						t.Fatal(err)
					}

					if err := tw.Close(); err != nil {
						t.Fatal(err)
					}

					if err := zw.Close(); err != nil {
						t.Fatal(err)
					}

					if _, err := io.Copy(w, buf); err != nil {
						t.Fatal(err)
					}

				default:
					t.Fatal(fmt.Sprintf("unknown request: %s", dump))
				}
			}))

			workspacePath, err = ioutil.TempDir("", "workspace")
			Expect(err).NotTo(HaveOccurred())

			file, err := os.Create(filepath.Join(workspacePath, "buildpack.toml"))
			Expect(err).NotTo(HaveOccurred())

			bpTOML := `api = "0.2"
[buildpack]
	id = "some-meta-buildpack"
	name = "Some Buildpack"
	version = "some-buildpack-version"

[metadata]
	include_files = ["buildpack.toml"]

	[[metadata.dependencies]]
		id = "some-buildpack-id"
		sha256 = "some-sha256"
		source = "some-source"
		source_sha256 = "some-source-sha256"
		stacks = ["some-stack"]
		uri = "%s/some-uri.tgz"
		version = "some-version"

	[[metadata.dependencies]]
		id = "some-other-buildpack-id"
		sha256 = "other-sha256"
		source = "other-source"
		source_sha256 = "other-source-sha256"
		stacks = ["other-stack"]
		uri = "%s/some-other-uri.tgz"
		version = "other-version"

[[order]]
	[[order.group]]
		id = "some-dependency-id"
		version = "some-version"

[[order]]
	[[order.group]]
		id = "other-dependency-id"
		version = "other-version"
		optional = true
`
			_, err = file.WriteString(fmt.Sprintf(bpTOML, api.URL, api.URL))
			Expect(err).NotTo(HaveOccurred())

			Expect(file.Close()).To(Succeed())
		})

		it("outputs the concatenation of all the `jam summarizes` of the implementation buildpacks", func() {
			command := exec.Command(
				entrypoint,
				"--workspace-path", workspacePath,
			)
			buffer := gbytes.NewBuffer()

			session, err := gexec.Start(command, buffer, buffer)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session).Should(gexec.Exit(0), func() string { return fmt.Sprintf("output:\n%s\n", buffer.Contents()) })

			expectedOutput := `Setting outputs
::set-output name=release_body::This buildpack contains the following dependencies%0A#### some-buildpack-id version some-version%0ASupported stacks:%0A| name |%0A|-|%0A| other-stack |%0A| some-stack |%0A%0A#### some-other-buildpack-id version other-version%0ASupported stacks:%0A| name |%0A|-|%0A| other-stack |%0A| some-stack |%0A
`
			Expect(string(buffer.Contents())).To(Equal(expectedOutput))

			Expect(requests).To(HaveLen(2))

			assetDownloadRequest := requests[0]
			Expect(assetDownloadRequest.Method).To(Equal("GET"))
			Expect(assetDownloadRequest.URL.Path).To(Equal("/some-uri.tgz"))

			sourceDownloadRequest := requests[1]
			Expect(sourceDownloadRequest.Method).To(Equal("GET"))
			Expect(sourceDownloadRequest.URL.Path).To(Equal("/some-other-uri.tgz"))
		})

		context("error cases", func() {
			context("when it can not parse the builpack.toml", func() {
				it.Before(func() {
					file, err := os.Create(filepath.Join(workspacePath, "buildpack.toml"))
					Expect(err).NotTo(HaveOccurred())

					bpTOML := `some bad toml`
					_, err = file.WriteString(bpTOML)
					Expect(err).NotTo(HaveOccurred())

					Expect(file.Close()).To(Succeed())
				})
				it("errors", func() {
					command := exec.Command(
						entrypoint,
						"--workspace-path", workspacePath,
					)
					buffer := gbytes.NewBuffer()

					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())

					Eventually(session).Should(gexec.Exit(1), func() string { return fmt.Sprintf("output:\n%s\n", buffer.Contents()) })
					Expect(buffer).To(gbytes.Say(`Error: failed to parse buildpack.toml:`))

				})
			})
			context("when it can not download the tarball", func() {
				it.Before(func() {
					file, err := os.Create(filepath.Join(workspacePath, "buildpack.toml"))
					Expect(err).NotTo(HaveOccurred())

					bpTOML := `api = "0.2"
[buildpack]
	id = "some-buildpack"
	name = "Some Buildpack"
	version = "some-buildpack-version"

[metadata]
	include_files = ["buildpack.toml"]

	[[metadata.dependencies]]
		id = "some-buildpack-id"
		sha256 = "some-sha256"
		source = "some-source"
		source_sha256 = "some-source-sha256"
		stacks = ["some-stack"]
		uri = "some-non-existant-uri.tgz"
		version = "some-version"

[[order]]
	[[order.group]]
		id = "some-dependency-id"
		version = "some-version"

`
					_, err = file.WriteString(bpTOML)
					Expect(err).NotTo(HaveOccurred())

					Expect(file.Close()).To(Succeed())
				})
				it("errors", func() {
					command := exec.Command(
						entrypoint,
						"--workspace-path", workspacePath,
					)
					buffer := gbytes.NewBuffer()

					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())

					Eventually(session).Should(gexec.Exit(1), func() string { return fmt.Sprintf("output:\n%s\n", buffer.Contents()) })
					Expect(buffer).To(gbytes.Say(`error: failed to download tarball:`))

				})
			})

			context("when the `jam summarize` call fails", func() {
				it.Before(func() {
					requests = []*http.Request{}
					api = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
						dump, _ := httputil.DumpRequest(req, true)
						receivedRequest, _ := http.ReadRequest(bufio.NewReader(bytes.NewBuffer(dump)))

						requests = append(requests, receivedRequest)

						switch req.URL.Path {
						case "/some-uri.tgz":
							buf := bytes.NewBuffer(nil)
							zw := gzip.NewWriter(buf)
							tw := tar.NewWriter(zw)

							file := bytes.NewBuffer(nil)
							_, err := file.WriteString(`some-malformed-buildpack`)
							if err != nil {
								t.Fatal(err)
							}

							err = tw.WriteHeader(&tar.Header{
								Name: "buildpack.toml",
								Mode: 0644,
								Size: int64(file.Len()),
							})
							if err != nil {
								t.Fatal(err)
							}

							if _, err := io.Copy(tw, file); err != nil {
								t.Fatal(err)
							}

							if err := tw.Close(); err != nil {
								t.Fatal(err)
							}

							if err := zw.Close(); err != nil {
								t.Fatal(err)
							}

							if _, err := io.Copy(w, buf); err != nil {
								t.Fatal(err)
							}
						default:
							t.Fatal(fmt.Sprintf("unknown request: %s", dump))
						}
					}))
					workspacePath, err = ioutil.TempDir("", "workspace")
					Expect(err).NotTo(HaveOccurred())

					file, err := os.Create(filepath.Join(workspacePath, "buildpack.toml"))
					Expect(err).NotTo(HaveOccurred())

					bpTOML := `api = "0.2"
[buildpack]
	id = "some-buildpack"
	name = "Some Buildpack"
	version = "some-buildpack-version"

[metadata]
	include_files = ["buildpack.toml"]

	[[metadata.dependencies]]
		id = "some-buildpack-id"
		sha256 = "some-sha256"
		source = "some-source"
		source_sha256 = "some-source-sha256"
		stacks = ["some-stack"]
		uri = "%s/some-uri.tgz"
		version = "some-version"

[[order]]
	[[order.group]]
		id = "some-dependency-id"
		version = "some-version"
`
					_, err = file.WriteString(fmt.Sprintf(bpTOML, api.URL))
					Expect(err).NotTo(HaveOccurred())

					Expect(file.Close()).To(Succeed())
				})

				it("errors", func() {
					command := exec.Command(
						entrypoint,
						"--workspace-path", workspacePath,
					)
					buffer := gbytes.NewBuffer()

					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())

					Eventually(session).Should(gexec.Exit(1), func() string { return fmt.Sprintf("output:\n%s\n", buffer.Contents()) })
					Expect(buffer).To(gbytes.Say(`error: failed to run 'jam summarize'`))
				})
			})
		})
	}, spec.Report(report.Terminal{}), spec.Parallel())
}
