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
	"testing"
	"time"

	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"

	. "github.com/onsi/gomega"
)

var entrypoint string

func TestEntrypoint(t *testing.T) {
	var Expect = NewWithT(t).Expect

	SetDefaultEventuallyTimeout(5 * time.Second)

	var err error
	entrypoint, err = gexec.Build("github.com/paketo-buildpacks/github-config/actions/dependency/parse/entrypoint")
	Expect(err).NotTo(HaveOccurred())

	spec.Run(t, "dependency/parse", func(t *testing.T, context spec.G, it spec.S) {
		var (
			Expect     = NewWithT(t).Expect
			Eventually = NewWithT(t).Eventually

			eventPath string
			api       *httptest.Server
			requests  []*http.Request
		)

		it.Before(func() {
			requests = []*http.Request{}
			api = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				dump, _ := httputil.DumpRequest(req, true)
				receivedRequest, _ := http.ReadRequest(bufio.NewReader(bytes.NewBuffer(dump)))

				requests = append(requests, receivedRequest)

				if req.Header.Get("Authorization") != "token some-github-token" {
					w.WriteHeader(http.StatusForbidden)
					return
				}

				switch req.URL.Path {
				case "/assets/some-asset.tgz":
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

				case "/assets/loop-asset.tgz":
					w.Header().Set("Location", "/assets/loop-asset.tgz")
					w.WriteHeader(http.StatusFound)

				case "/assets/missing-asset.tgz":
					w.WriteHeader(http.StatusNotFound)

				case "/assets/malformed-asset.tgz":
					if _, err := w.Write([]byte("malformed-content")); err != nil {
						t.Fatal(err)
					}

				case "/assets/malformed-toml-asset.tgz":
					buf := bytes.NewBuffer(nil)
					zw := gzip.NewWriter(buf)
					tw := tar.NewWriter(zw)

					file := bytes.NewBuffer(nil)
					_, err := file.WriteString("%%%")
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

				case "/some-org/some-repo/archive/some-tag-name.tar.gz":
					if _, err := w.Write([]byte("archive-content")); err != nil {
						t.Fatal(err)
					}

				default:
					t.Fatal(fmt.Sprintf("unknown request: %s", dump))
				}
			}))

			file, err := ioutil.TempFile("", "event.json")
			Expect(err).NotTo(HaveOccurred())

			_, err = file.WriteString(fmt.Sprintf(`{
				"release": {
					"assets": [
						{
						  "url": "%s/assets/other-asset.cnb",
							"name": "other-asset.cnb",
							"browser_download_url": "browser/other-asset.cnb"
						},
						{
						  "url": "%s/assets/some-asset.tgz",
							"name": "some-asset.tgz",
							"browser_download_url": "browser/some-asset.tgz"
						}
					],
					"name": "Release v1.2.3",
					"tag_name": "some-tag-name"
				},
				"repository": {
					"full_name": "some-org/some-repo"
				}
			}`, api.URL, api.URL))
			Expect(err).NotTo(HaveOccurred())

			Expect(file.Close()).To(Succeed())

			eventPath = file.Name()
		})

		it.After(func() {
			api.Close()

			Expect(os.RemoveAll(eventPath)).To(Succeed())
		})

		it("outputs the dependency details of a release", func() {
			command := exec.Command(
				entrypoint,
				"--github-uri", api.URL,
				"--github-token", "some-github-token",
			)
			command.Env = append(command.Env, fmt.Sprintf("GITHUB_EVENT_PATH=%s", eventPath))
			buffer := gbytes.NewBuffer()

			session, err := gexec.Start(command, buffer, buffer)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session).Should(gexec.Exit(0), func() string { return fmt.Sprintf("output:\n%s\n", buffer.Contents()) })

			Expect(buffer).To(gbytes.Say(`Parsing release event`))
			Expect(buffer).To(gbytes.Say(`  Repository: "some-org/some-repo"`))
			Expect(buffer).To(gbytes.Say(`  Release:    "Release v1.2.3"`))
			Expect(buffer).To(gbytes.Say(`  Tag:        "some-tag-name"`))
			Expect(buffer).To(gbytes.Say(`Downloading assets`))
			Expect(buffer).To(gbytes.Say(fmt.Sprintf(`  Release: "%s/assets/some-asset.tgz"`, api.URL)))
			Expect(buffer).To(gbytes.Say(fmt.Sprintf(`  Source:  "%s/some-org/some-repo/archive/some-tag-name.tar.gz"`, api.URL)))

			Expect(buffer).To(gbytes.Say(`Setting outputs`))
			Expect(buffer).To(gbytes.Say(`::set-output name=id::some-buildpack-id`))
			Expect(buffer).To(gbytes.Say(`::set-output name=sha256::ef51de88354adfd23342a3350ce71284ad8fd0226a4cb53a9d228d08ef6490d7`))
			Expect(buffer).To(gbytes.Say(fmt.Sprintf(`::set-output name=source::%s/some-org/some-repo/archive/some-tag-name.tar.gz`, api.URL)))
			Expect(buffer).To(gbytes.Say(`::set-output name=source_sha256::7cb3e37a8c71a287586d099b9c310df90eac69d7f99c0042af4ce67ebc504c22`))
			Expect(buffer).To(gbytes.Say(`::set-output name=stacks::\["some-stack","other-stack"\]`))
			Expect(buffer).To(gbytes.Say(`::set-output name=uri::browser/some-asset.tgz`))
			Expect(buffer).To(gbytes.Say(`::set-output name=version::some-version`))

			Expect(requests).To(HaveLen(2))

			assetDownloadRequest := requests[0]
			Expect(assetDownloadRequest.Method).To(Equal("GET"))
			Expect(assetDownloadRequest.URL.Path).To(Equal("/assets/some-asset.tgz"))
			Expect(assetDownloadRequest.Header.Get("Accept")).To(Equal("application/octet-stream"))
			Expect(assetDownloadRequest.Header.Get("Authorization")).To(Equal("token some-github-token"))

			sourceDownloadRequest := requests[1]
			Expect(sourceDownloadRequest.Method).To(Equal("GET"))
			Expect(sourceDownloadRequest.URL.Path).To(Equal("/some-org/some-repo/archive/some-tag-name.tar.gz"))
			Expect(assetDownloadRequest.Header.Get("Authorization")).To(Equal("token some-github-token"))
		})

		context("failure cases", func() {
			context("when the event file does not exist", func() {
				it.Before(func() {
					Expect(os.Remove(eventPath)).To(Succeed())
				})

				it("prints an error message and exits non-zero", func() {
					command := exec.Command(
						entrypoint,
						"--github-uri", api.URL,
						"--github-token", "some-github-token",
					)
					command.Env = append(command.Env, fmt.Sprintf("GITHUB_EVENT_PATH=%s", eventPath))
					buffer := gbytes.NewBuffer()

					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())

					Eventually(session).Should(gexec.Exit(1), func() string { return fmt.Sprintf("output:\n%s\n", buffer.Contents()) })

					Expect(buffer).To(gbytes.Say(`Error: failed to read \$GITHUB_EVENT_PATH:`))
					Expect(buffer).To(gbytes.Say(`no such file or directory`))
				})
			})

			context("when the event file contains malformed json", func() {
				it.Before(func() {
					Expect(ioutil.WriteFile(eventPath, []byte("%%%"), 0644)).To(Succeed())
				})

				it("prints an error message and exits non-zero", func() {
					command := exec.Command(
						entrypoint,
						"--github-uri", api.URL,
						"--github-token", "some-github-token",
					)
					command.Env = append(command.Env, fmt.Sprintf("GITHUB_EVENT_PATH=%s", eventPath))
					buffer := gbytes.NewBuffer()

					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())

					Eventually(session).Should(gexec.Exit(1), func() string { return fmt.Sprintf("output:\n%s\n", buffer.Contents()) })

					Expect(buffer).To(gbytes.Say(`Error: failed to decode \$GITHUB_EVENT_PATH:`))
					Expect(buffer).To(gbytes.Say(`invalid character`))
				})
			})

			context("when the download request cannot be created", func() {
				it("prints an error message and exits non-zero", func() {
					command := exec.Command(
						entrypoint,
						"--github-uri", "%%%",
						"--github-token", "some-github-token",
					)
					command.Env = append(command.Env, fmt.Sprintf("GITHUB_EVENT_PATH=%s", eventPath))
					buffer := gbytes.NewBuffer()

					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())

					Eventually(session).Should(gexec.Exit(1), func() string { return fmt.Sprintf("output:\n%s\n", buffer.Contents()) })

					Expect(buffer).To(gbytes.Say(`Error: failed to download asset:`))
					Expect(buffer).To(gbytes.Say(`invalid URL escape`))
				})
			})

			context("when the download request cannot be completed", func() {
				it.Before(func() {
					err := ioutil.WriteFile(eventPath, []byte(fmt.Sprintf(`{
						"release": {
							"assets": [
								{
									"url": "%s/assets/loop-asset.tgz",
									"name": "loop-asset.tgz",
									"browser_download_url": "browser/loop-asset.tgz"
								}
							],
							"name": "Release v1.2.3",
							"tag_name": "some-tag-name"
						},
						"repository": {
							"full_name": "some-org/some-repo"
						}
					}`, api.URL)), 0644)
					Expect(err).NotTo(HaveOccurred())
				})

				it("prints an error message and exits non-zero", func() {
					command := exec.Command(
						entrypoint,
						"--github-uri", api.URL,
						"--github-token", "some-github-token",
					)
					command.Env = append(command.Env, fmt.Sprintf("GITHUB_EVENT_PATH=%s", eventPath))
					buffer := gbytes.NewBuffer()

					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())

					Eventually(session).Should(gexec.Exit(1), func() string { return fmt.Sprintf("output:\n%s\n", buffer.Contents()) })

					Expect(buffer).To(gbytes.Say(`Error: failed to download asset:`))
					Expect(buffer).To(gbytes.Say(`stopped after 10 redirects`))
				})
			})

			context("when the download status is unexpected", func() {
				it.Before(func() {
					err := ioutil.WriteFile(eventPath, []byte(fmt.Sprintf(`{
						"release": {
							"assets": [
								{
									"url": "%s/assets/missing-asset.tgz",
									"name": "missing-asset.tgz",
									"browser_download_url": "browser/missing-asset.tgz"
								}
							],
							"name": "Release v1.2.3",
							"tag_name": "some-tag-name"
						},
						"repository": {
							"full_name": "some-org/some-repo"
						}
					}`, api.URL)), 0644)
					Expect(err).NotTo(HaveOccurred())
				})

				it("prints an error message and exits non-zero", func() {
					command := exec.Command(
						entrypoint,
						"--github-uri", api.URL,
						"--github-token", "some-github-token",
					)
					command.Env = append(command.Env, fmt.Sprintf("GITHUB_EVENT_PATH=%s", eventPath))
					buffer := gbytes.NewBuffer()

					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())

					Eventually(session).Should(gexec.Exit(1), func() string { return fmt.Sprintf("output:\n%s\n", buffer.Contents()) })

					Expect(buffer).To(gbytes.Say(`Error: failed to download asset:`))
					Expect(buffer).To(gbytes.Say(`unexpected response`))
				})
			})

			context("when the asset tarball is malformed", func() {
				it.Before(func() {
					err := ioutil.WriteFile(eventPath, []byte(fmt.Sprintf(`{
						"release": {
							"assets": [
								{
									"url": "%s/assets/malformed-asset.tgz",
									"name": "malformed-asset.tgz",
									"browser_download_url": "browser/malformed-asset.tgz"
								}
							],
							"name": "Release v1.2.3",
							"tag_name": "some-tag-name"
						},
						"repository": {
							"full_name": "some-org/some-repo"
						}
					}`, api.URL)), 0644)
					Expect(err).NotTo(HaveOccurred())
				})

				it("prints an error message and exits non-zero", func() {
					command := exec.Command(
						entrypoint,
						"--github-uri", api.URL,
						"--github-token", "some-github-token",
					)
					command.Env = append(command.Env, fmt.Sprintf("GITHUB_EVENT_PATH=%s", eventPath))
					buffer := gbytes.NewBuffer()

					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())

					Eventually(session).Should(gexec.Exit(1), func() string { return fmt.Sprintf("output:\n%s\n", buffer.Contents()) })

					Expect(buffer).To(gbytes.Say(`Error: failed to read asset:`))
					Expect(buffer).To(gbytes.Say(`invalid header`))
				})
			})

			context("when the asset buildpack.toml is malformed", func() {
				it.Before(func() {
					err := ioutil.WriteFile(eventPath, []byte(fmt.Sprintf(`{
						"release": {
							"assets": [
								{
									"url": "%s/assets/malformed-toml-asset.tgz",
									"name": "malformed-toml-asset.tgz",
									"browser_download_url": "browser/malformed-toml-asset.tgz"
								}
							],
							"name": "Release v1.2.3",
							"tag_name": "some-tag-name"
						},
						"repository": {
							"full_name": "some-org/some-repo"
						}
					}`, api.URL)), 0644)
					Expect(err).NotTo(HaveOccurred())
				})

				it("prints an error message and exits non-zero", func() {
					command := exec.Command(
						entrypoint,
						"--github-uri", api.URL,
						"--github-token", "some-github-token",
					)
					command.Env = append(command.Env, fmt.Sprintf("GITHUB_EVENT_PATH=%s", eventPath))
					buffer := gbytes.NewBuffer()

					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())

					Eventually(session).Should(gexec.Exit(1), func() string { return fmt.Sprintf("output:\n%s\n", buffer.Contents()) })

					Expect(buffer).To(gbytes.Say(`Error: failed to read buildpack.toml:`))
					Expect(buffer).To(gbytes.Say(`bare keys cannot contain '%'`))
				})
			})
		})
	}, spec.Report(report.Terminal{}), spec.Parallel())
}
