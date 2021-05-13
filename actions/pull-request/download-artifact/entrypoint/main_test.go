package main_test

import (
	"archive/zip"
	"bytes"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

func TestEntrypoint(t *testing.T) {
	var Expect = NewWithT(t).Expect

	SetDefaultEventuallyTimeout(5 * time.Second)

	entrypoint, err := gexec.Build("github.com/paketo-buildpacks/github-config/actions/pull-request/download-artifact/entrypoint")
	Expect(err).NotTo(HaveOccurred())

	spec.Run(t, "actions/pull-request/download-artifact", func(t *testing.T, context spec.G, it spec.S) {
		var (
			Expect     = NewWithT(t).Expect
			Eventually = NewWithT(t).Eventually

			mockServer *httptest.Server
			tempDir    string
		)

		it.Before(func() {
			mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				if req.Method == http.MethodHead {
					http.Error(w, "NotFound", http.StatusNotFound)

					return
				}

				switch req.URL.Path {
				case "/":
					w.WriteHeader(http.StatusOK)

				case "/repos/some-owner/some-repo/actions/runs/12345/artifacts":
					fmt.Fprintf(w, `{
						"artifacts": [
							{
								"name": "payload",
								"size_in_bytes": 28244,
								"archive_download_url": "%[1]s/repos/some-owner/some-repo/actions/artifacts/54321/zip"
							},
							{
								"name": "bad-payload",
								"size_in_bytes": 28244,
								"archive_download_url": "%[1]s/repos/some-owner/some-repo/actions/artifacts/654321/zip"
							},
							{
								"name": "another-payload",
								"size_in_bytes": 28244,
								"archive_download_url": "%[1]s/repos/some-owner/some-repo/actions/artifacts/55555/zip"
							},
							{
								"name": "other-payload",
								"size_in_bytes": 28244,
								"archive_download_url": "%[1]s/repos/some-owner/some-repo/actions/artifacts/77777/zip"
							},
							{
								"name": "last-payload",
								"size_in_bytes": 28244,
								"archive_download_url": "does-not-exist/repos/some-owner/some-repo/actions/artifacts/88888/zip"
							}
						]
					}`, mockServer.URL)

				case "/repos/some-owner/some-repo/actions/artifacts/54321/zip":
					buf := bytes.NewBuffer(nil)
					writer := zip.NewWriter(buf)
					for _, name := range []string{"some", "other"} {
						f, err := writer.Create(fmt.Sprintf("%s-file", name))
						if err != nil {
							log.Fatal(err)
						}

						fmt.Fprintf(f, "%s-contents", name)
					}

					err = writer.Close()
					if err != nil {
						log.Fatal(err)
					}

					fmt.Fprint(w, buf.String())

				case "/repos/some-owner/some-repo/actions/artifacts/654321/zip":
					buf := bytes.NewBuffer(nil)
					writer := zip.NewWriter(buf)

					f, err := writer.Create("wrong-file.json")
					if err != nil {
						log.Fatal(err)
					}

					fmt.Fprint(f, "{}")

					err = writer.Close()
					if err != nil {
						log.Fatal(err)
					}

					fmt.Fprint(w, buf.String())

				case "/repos/some-owner/nonexistent-repo/actions/runs/45678/artifacts":
					w.WriteHeader(http.StatusNotFound)

				case "/repos/some-owner/some-repo/actions/runs/1111/artifacts":
					fmt.Fprint(w, "%%%")

				case "/repos/some-owner/some-repo/actions/artifacts/55555/zip":
					w.WriteHeader(http.StatusOK)

				case "/repos/some-owner/some-repo/actions/artifacts/77777/zip":
					w.WriteHeader(http.StatusBadRequest)

				case "/repos/some-owner/some-repo/actions/artifacts/88888/zip":
					http.Error(w, "bad request", http.StatusInternalServerError)

				default:
					t.Fatal(fmt.Sprintf("unknown path: %s", req.URL.Path))
				}
			}))

			tempDir, err = os.MkdirTemp("", "output")
			Expect(err).NotTo(HaveOccurred())
		})

		it.After(func() {
			mockServer.Close()
			Expect(os.RemoveAll(tempDir)).To(Succeed())
		})

		context("given arguments that point to a repo with an artifact", func() {
			it("gets the artifact zip and unzips it onto the file system", func() {
				command := exec.Command(
					entrypoint,
					"--name", "payload",
					"--glob", "*-file",
					"--repo", "some-owner/some-repo",
					"--run-id", "12345",
					"--github-api", mockServer.URL,
					"--workspace", tempDir,
					"--token", "some-token",
				)

				buffer := gbytes.NewBuffer()
				session, err := gexec.Start(command, buffer, buffer)
				Expect(err).NotTo(HaveOccurred())

				Eventually(session).Should(gexec.Exit(0), func() string { return string(buffer.Contents()) })

				Expect(buffer).To(gbytes.Say(fmt.Sprintf("Getting workflow artifacts from %s/repos/some-owner/some-repo/actions/runs/12345/artifacts", mockServer.URL)))
				Expect(buffer).To(gbytes.Say(fmt.Sprintf("Downloading zip from %s/repos/some-owner/some-repo/actions/artifacts/54321/zip", mockServer.URL)))
				Expect(buffer).To(gbytes.Say("Unpacking file: some-file"))
				Expect(buffer).To(gbytes.Say("Unpacking file: other-file"))

				contents, err := os.ReadFile(filepath.Join(tempDir, "some-file"))
				Expect(err).NotTo(HaveOccurred())
				Expect(string(contents)).To(Equal("some-contents"))

				contents, err = os.ReadFile(filepath.Join(tempDir, "other-file"))
				Expect(err).NotTo(HaveOccurred())
				Expect(string(contents)).To(Equal("other-contents"))
			})
		})

		context("failure cases", func() {
			context("when the --name flag is missing", func() {
				it("returns an error and exits non-zero", func() {
					command := exec.Command(
						entrypoint,
						"--glob", "some-file",
						"--repo", "some-owner/some-repo",
						"--run-id", "12345",
						"--github-api", mockServer.URL,
						"--workspace", tempDir,
						"--token", "some-token",
					)

					buffer := gbytes.NewBuffer()
					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())

					Eventually(session).Should(gexec.Exit(1), func() string { return string(buffer.Contents()) })
					Expect(buffer).To(gbytes.Say(`missing required flag --name`))
				})
			})

			context("when the --repo flag is missing", func() {
				it("returns an error and exits non-zero", func() {
					command := exec.Command(
						entrypoint,
						"--name", "payload",
						"--glob", "some-file",
						"--run-id", "12345",
						"--github-api", mockServer.URL,
						"--workspace", tempDir,
						"--token", "some-token",
					)

					buffer := gbytes.NewBuffer()
					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())

					Eventually(session).Should(gexec.Exit(1), func() string { return string(buffer.Contents()) })
					Expect(buffer).To(gbytes.Say(`missing required flag --repo`))
				})
			})

			context("when the --run-id flag is missing", func() {
				it("returns an error and exits non-zero", func() {
					command := exec.Command(
						entrypoint,
						"--name", "payload",
						"--glob", "some-file",
						"--repo", "some-owner/some-repo",
						"--github-api", mockServer.URL,
						"--workspace", tempDir,
						"--token", "some-token",
					)

					buffer := gbytes.NewBuffer()
					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())

					Eventually(session).Should(gexec.Exit(1), func() string { return string(buffer.Contents()) })
					Expect(buffer).To(gbytes.Say(`missing required flag --run-id`))
				})
			})

			context("when the --workspace flag is missing", func() {
				it("returns an error and exits non-zero", func() {
					command := exec.Command(
						entrypoint,
						"--name", "payload",
						"--glob", "some-file",
						"--repo", "some-owner/some-repo",
						"--run-id", "12345",
						"--github-api", mockServer.URL,
						"--token", "some-token",
					)

					buffer := gbytes.NewBuffer()
					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())

					Eventually(session).Should(gexec.Exit(1), func() string { return string(buffer.Contents()) })
					Expect(buffer).To(gbytes.Say(`missing required flag --workspace`))
				})
			})

			context("when the --token flag is missing", func() {
				it("returns an error and exits non-zero", func() {
					command := exec.Command(
						entrypoint,
						"--name", "payload",
						"--glob", "some-file",
						"--repo", "some-owner/some-repo",
						"--run-id", "12345",
						"--github-api", mockServer.URL,
						"--workspace", tempDir,
					)

					buffer := gbytes.NewBuffer()
					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())

					Eventually(session).Should(gexec.Exit(1), func() string { return string(buffer.Contents()) })
					Expect(buffer).To(gbytes.Say(`missing required flag --token`))
				})
			})

			context("the list artifacts request fails", func() {
				it("returns an error and exits non-zero", func() {
					command := exec.Command(
						entrypoint,
						"--name", "payload",
						"--glob", "some-file",
						"--repo", "some-owner/some-repo",
						"--run-id", "45678",
						"--github-api", "does-not-exist",
						"--workspace", tempDir,
						"--token", "some-token",
					)

					buffer := gbytes.NewBuffer()
					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())

					Eventually(session).Should(gexec.Exit(1), func() string { return string(buffer.Contents()) })
					Expect(string(buffer.Contents())).To(ContainSubstring("failed to list artifacts"))
				})
			})

			context("the list artifacts response is not JSON", func() {
				it("returns an error and exits non-zero", func() {
					command := exec.Command(
						entrypoint,
						"--name", "payload",
						"--glob", "some-file",
						"--repo", "some-owner/some-repo",
						"--run-id", "1111",
						"--github-api", mockServer.URL,
						"--workspace", tempDir,
						"--token", "some-token",
					)

					buffer := gbytes.NewBuffer()
					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())

					Eventually(session).Should(gexec.Exit(1), func() string { return string(buffer.Contents()) })
					Expect(string(buffer.Contents())).To(ContainSubstring("failed to parse artifacts response: invalid character"))
				})
			})

			context("listing the artifacts returns an unexpected status code", func() {
				it("returns an error and exits non-zero", func() {
					command := exec.Command(
						entrypoint,
						"--name", "payload",
						"--glob", "some-file",
						"--repo", "some-owner/nonexistent-repo",
						"--run-id", "45678",
						"--github-api", mockServer.URL,
						"--workspace", tempDir,
						"--token", "some-token",
					)

					buffer := gbytes.NewBuffer()
					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())

					Eventually(session).Should(gexec.Exit(1), func() string { return string(buffer.Contents()) })
					Expect(string(buffer.Contents())).To(ContainSubstring("failed to list artifacts: status code 404"))
				})
			})

			context("artifacts are returned but there is no match", func() {
				it("returns an error and exits non-zero", func() {
					command := exec.Command(
						entrypoint,
						"--name", "wrong-artifact",
						"--glob", "some-file",
						"--repo", "some-owner/some-repo",
						"--run-id", "12345",
						"--github-api", mockServer.URL,
						"--workspace", tempDir,
						"--token", "some-token",
					)

					buffer := gbytes.NewBuffer()
					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())

					Eventually(session).Should(gexec.Exit(1), func() string { return string(buffer.Contents()) })
					Expect(string(buffer.Contents())).To(ContainSubstring("failed to find matching artifact"))
				})
			})

			context("retrieved artifact is not a zip file", func() {
				it("returns an error and exits non-zero", func() {
					command := exec.Command(
						entrypoint,
						"--name", "another-payload",
						"--glob", "some-file",
						"--repo", "some-owner/some-repo",
						"--run-id", "12345",
						"--github-api", mockServer.URL,
						"--workspace", tempDir,
						"--token", "some-token",
					)

					buffer := gbytes.NewBuffer()
					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())

					Eventually(session).Should(gexec.Exit(1), func() string { return string(buffer.Contents()) })
					Expect(string(buffer.Contents())).To(ContainSubstring("zip: not a valid zip file"))
				})
			})

			context("request to retrieve payload returns a bad status code", func() {
				it("returns an error and exits non-zero", func() {
					command := exec.Command(
						entrypoint,
						"--name", "other-payload",
						"--glob", "some-file",
						"--repo", "some-owner/some-repo",
						"--run-id", "12345",
						"--github-api", mockServer.URL,
						"--workspace", tempDir,
						"--token", "some-token",
					)

					buffer := gbytes.NewBuffer()
					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())

					Eventually(session).Should(gexec.Exit(1), func() string { return string(buffer.Contents()) })
					Expect(string(buffer.Contents())).To(ContainSubstring("failed to get artifact zip file: status code 400"))
				})
			})

			context("the payload zip GET request fails", func() {
				it("returns an error and exits non-zero", func() {
					command := exec.Command(
						entrypoint,
						"--name", "last-payload",
						"--glob", "some-file",
						"--repo", "some-owner/some-repo",
						"--run-id", "12345",
						"--github-api", mockServer.URL,
						"--workspace", tempDir,
						"--token", "some-token",
					)

					buffer := gbytes.NewBuffer()
					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())

					Eventually(session).Should(gexec.Exit(1), func() string { return string(buffer.Contents()) })
					Expect(string(buffer.Contents())).To(ContainSubstring("failed to get artifact zip file"))
				})
			})

			context("cannot write the file to the github workspace", func() {
				it.Before(func() {
					Expect(os.Mkdir(filepath.Join(tempDir, "bad-dir"), 0000))
				})

				it.After(func() {
					Expect(os.Remove(filepath.Join(tempDir, "bad-dir"))).To(Succeed())
				})

				it("returns an error and exits non-zero", func() {
					command := exec.Command(
						entrypoint,
						"--name", "payload",
						"--glob", "some-file",
						"--repo", "some-owner/some-repo",
						"--run-id", "12345",
						"--github-api", mockServer.URL,
						"--workspace", filepath.Join(tempDir, "bad-dir"),
						"--token", "some-token",
					)

					buffer := gbytes.NewBuffer()
					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())

					Eventually(session).Should(gexec.Exit(1), func() string { return string(buffer.Contents()) })
					Expect(string(buffer.Contents())).To(ContainSubstring("permission denied"))
				})
			})

			context("the zip file does not contain a matching file", func() {
				it("it returns an error and exits non-zero", func() {
					command := exec.Command(
						entrypoint,
						"--name", "bad-payload",
						"--glob", "some-file",
						"--repo", "some-owner/some-repo",
						"--run-id", "12345",
						"--github-api", mockServer.URL,
						"--workspace", tempDir,
						"--token", "some-token",
					)

					buffer := gbytes.NewBuffer()
					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())

					Eventually(session).Should(gexec.Exit(1), func() string { return string(buffer.Contents()) })

					Expect(buffer).To(gbytes.Say(fmt.Sprintf("Getting workflow artifacts from %s/repos/some-owner/some-repo/actions/runs/12345/artifacts", mockServer.URL)))
					Expect(buffer).To(gbytes.Say(fmt.Sprintf("Downloading zip from %s/repos/some-owner/some-repo/actions/artifacts/654321/zip", mockServer.URL)))
					Expect(buffer).To(gbytes.Say(`failed to find any files matching "some-file" in zip`))
				})
			})
		})
	})
}
