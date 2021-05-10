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

	var err error
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
					filename := "artifact.json"
					w.Header().Set("Content-Type", "application/json")
					w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
					body := []byte(`{
  "total_count": 3,
  "artifacts": [
    {
      "id": 12345,
      "name": "payload",
      "size_in_bytes": 28244,
      "url": "/repos/some-owner/some-repo/actions/artifacts/54321",
      "archive_download_url": "/repos/some-owner/some-repo/actions/artifacts/54321/zip",
      "expired": false
    },
    {
      "id": 23456,
      "name": "another-payload",
      "size_in_bytes": 28244,
      "url": "/repos/some-owner/some-repo/actions/artifacts/55555",
      "archive_download_url": "/repos/some-owner/some-repo/actions/artifacts/55555/zip",
      "expired": false
    },
    {
      "id": 34567,
      "name": "other-payload",
      "size_in_bytes": 28244,
      "url": "/repos/some-owner/some-repo/actions/artifacts/77777",
      "archive_download_url": "/repos/some-owner/some-repo/actions/artifacts/77777/zip",
      "expired": false
    },
    {
      "id": 45678,
      "name": "last-payload",
      "size_in_bytes": 28244,
      "url": "/repos/some-owner/some-repo/actions/artifacts/88888",
      "archive_download_url": "/repos/some-owner/some-repo/actions/artifacts/88888/zip",
      "expired": false
    }
  ]
}`)
					_, _ = w.Write(body)
					w.WriteHeader(http.StatusOK)

				case "/repos/some-owner/some-repo/actions/artifacts/54321/zip":
					// serving a zip file
					filename := "payload"
					buf := new(bytes.Buffer)
					writer := zip.NewWriter(buf)
					data := []byte(`{
  "action": "synchronize",
  "pull_request": {
    "_links": {
      "comments": {
        "href": "https://api.github.com/repos/some-org/some-repo/issues/1/comments"
      },
      "commits": {
        "href": "https://api.github.com/repos/some-org/some-repo/pulls/1/commits"
      }
    },
    "body": "Body of PR",
    "changed_files": 2,
    "closed_at": null,
    "comments": 0,
    "commits": 5,
    "deletions": 52,
    "labels": [],
    "number": 1,
    "state": "open",
    "title": "Title",
    "user": {
      "id": 98765,
      "login": "paketo-bot"
    }
  }
}`)
					f, err := writer.Create(filename)
					if err != nil {
						log.Fatal(err)
					}
					_, err = f.Write(data)
					if err != nil {
						log.Fatal(err)
					}
					err = writer.Close()
					if err != nil {
						log.Fatal(err)
					}
					w.Header().Set("Content-Type", "application/zip")
					w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
					_, _ = w.Write(buf.Bytes())
					w.WriteHeader(http.StatusOK)

				case "/repos/some-owner/bad-endpoint/actions/runs/45678/artifacts":
					http.Error(w, "bad request", 0)

				case "/repos/some-owner/nonexistent-repo/actions/runs/45678/artifacts":
					w.WriteHeader(http.StatusNotFound)

				case "/repos/some-owner/some-repo/actions/artifacts/55555/zip":
					filename := "another-payload"
					w.Header().Set("Content-Type", "application/json")
					w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
					w.WriteHeader(http.StatusOK)

				case "/repos/some-owner/some-repo/actions/artifacts/77777/zip":
					filename := "other-payload"
					w.Header().Set("Content-Type", "application/json")
					w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
					w.WriteHeader(http.StatusBadRequest)

				case "/repos/some-owner/some-repo/actions/artifacts/88888/zip":
					http.Error(w, "bad request", 0)

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

				Expect(buffer).To(gbytes.Say(`Getting workflow artifacts from http://127.0.0.1:\d+/repos/some-owner/some-repo/actions/runs/12345/artifacts`))
				Expect(buffer).To(gbytes.Say("Getting workflow artifact zip file"))
				Expect(buffer).To(gbytes.Say("Reading file: payload"))

				contents, _ := os.ReadFile(filepath.Join(tempDir, "event.json"))
				Expect(string(contents)).To(MatchJSON(`{
				"action": "synchronize",
				"pull_request": {
					"_links": {
						"comments": {
							"href": "https://api.github.com/repos/some-org/some-repo/issues/1/comments"
						},
						"commits": {
							"href": "https://api.github.com/repos/some-org/some-repo/pulls/1/commits"
						}
					},
					"body": "Body of PR",
					"changed_files": 2,
					"closed_at": null,
					"comments": 0,
					"commits": 5,
					"deletions": 52,
					"labels": [],
					"number": 1,
					"state": "open",
					"title": "Title",
					"user": {
						"id": 98765,
						"login": "paketo-bot"
					}
				}
				}`))
			})
		})

		context("failure cases", func() {
			context("the artifact GET request fails", func() {
				it("returns an error and exits non-zero", func() {
					command := exec.Command(
						entrypoint,
						"--name", "payload",
						"--repo", "some-owner/bad-endpoint",
						"--run-id", "45678",
						"--github-api", mockServer.URL,
						"--workspace", tempDir,
						"--token", "some-token",
					)

					buffer := gbytes.NewBuffer()
					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())

					Eventually(session).Should(gexec.Exit(1), func() string { return string(buffer.Contents()) })
					Expect(string(buffer.Contents())).To(ContainSubstring("failed making a request to get artifacts"))
				})
			})

			context("getting the artifact returns a bad status code", func() {
				it("returns an error and exits non-zero", func() {
					command := exec.Command(
						entrypoint,
						"--name", "payload",
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
					Expect(string(buffer.Contents())).To(ContainSubstring("failed getting workflow artifacts with status code: 404"))
				})
			})

			context("artifacts are returned but there is no match", func() {
				it("returns an error and exits non-zero", func() {
					command := exec.Command(
						entrypoint,
						"--name", "wrong-artifact",
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
					Expect(string(buffer.Contents())).To(ContainSubstring("no exact workflow artifact found"))
				})
			})

			context("retrieved artifact is not a zip file", func() {
				it("returns an error and exits non-zero", func() {
					command := exec.Command(
						entrypoint,
						"--name", "another-payload",
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
					Expect(string(buffer.Contents())).To(ContainSubstring("failed getting payload with status code: 400"))
				})
			})

			context("the payload zip GET request fails", func() {
				it("returns an error and exits non-zero", func() {
					command := exec.Command(
						entrypoint,
						"--name", "last-payload",
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
		})
	})
}
