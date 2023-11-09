package main_test

import (
	"bufio"
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
	entrypoint, err := gexec.Build("github.com/paketo-buildpacks/github-config/actions/release/reset-draft/entrypoint")
	Expect(err).NotTo(HaveOccurred())

	spec.Run(t, "reset-draft", func(t *testing.T, context spec.G, it spec.S) {
		var (
			Expect     = NewWithT(t).Expect
			Eventually = NewWithT(t).Eventually

			api      *httptest.Server
			requests []*http.Request
			tempDir  string
		)

		it.Before(func() {
			api = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				dump, _ := httputil.DumpRequest(req, true)
				receivedRequest, _ := http.ReadRequest(bufio.NewReader(bytes.NewBuffer(dump)))

				requests = append(requests, receivedRequest)

				if req.Header.Get("Authorization") != "token some-github-token" {
					w.WriteHeader(http.StatusForbidden)
					return
				}

				switch req.URL.Path {
				case "/repos/some-org/published-repo/releases":
					fmt.Fprintln(w, `[
						{
							"draft": false,
							"id": 2,
							"tag_name": "random-version"
						},
						{
							"draft": false,
							"id": 3,
							"tag_name": "1.2.3"
						}
					]`)

				case "/repos/some-org/draft-repo/releases":
					fmt.Fprintln(w, `[
						{
							"draft": true,
							"id": 2,
							"tag_name": "some-version"
						},
						{
							"draft": true,
							"id": 3,
							"tag_name": "1.2.3"
						}
					]`)

				case "/repos/some-org/loop-repo/releases":
					fmt.Fprintln(w, `[
						{
							"draft": true,
							"id": 3
						}
					]`)

				case "/repos/some-org/empty-repo/releases":
					fmt.Fprintln(w, `[]`)

				case "/repos/some-org/draft-repo/releases/2":
					if req.Method == http.MethodDelete {
						w.WriteHeader(http.StatusNoContent)
					}

				case "/repos/some-org/draft-repo/releases/3":
					if req.Method == http.MethodDelete {
						w.WriteHeader(http.StatusNoContent)
					}

				case "/repos/redirect-org/loop-repo/releases":
					w.Header().Set("Location", "/repos/redirect-org/loop-repo/releases")
					w.WriteHeader(http.StatusFound)

				case "/repos/some-org/loop-repo/releases/3":
					w.Header().Set("Location", "/repos/some-org/loop-repo/releases/3")
					w.WriteHeader(http.StatusFound)

				case "/repos/some-org/error-repo/releases":
					w.WriteHeader(http.StatusInternalServerError)
					fmt.Fprintln(w, `{"error": "server-error"}`)

				case "/repos/some-org/malformed-json-repo/releases":
					fmt.Fprintln(w, `%%%`)

				case "/repos/some-org/delete-error-repo/releases":
					fmt.Fprintln(w, `[
						{
							"draft": true,
							"id": 3
						}
					]`)

				case "/repos/some-org/delete-error-repo/releases/3":
					w.WriteHeader(http.StatusInternalServerError)
					fmt.Fprintln(w, `{"error": "server-error"}`)

				default:
					t.Fatalf("unknown request: %s", dump)
				}
			}))

			tempDir = t.TempDir()
		})

		context("when a specific version is passed in", func() {
			context("when a matching draft release does NOT exist", func() {
				it("does not change the repo", func() {
					command := exec.Command(
						entrypoint,
						"--endpoint", api.URL,
						"--repo", "some-org/published-repo",
						"--token", "some-github-token",
						"--version", "1.2.3",
					)

					buffer := gbytes.NewBuffer()

					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())

					Eventually(session).Should(gexec.Exit(0), func() string { return fmt.Sprintf("output -> \n%s\n", buffer.Contents()) })

					Expect(requests).To(HaveLen(1))
					Expect(requests[0].Method).To(Equal("GET"))
					Expect(requests[0].URL.Path).To(Equal("/repos/some-org/published-repo/releases"))

					Expect(buffer).To(gbytes.Say(`Fetching latest releases`))
					Expect(buffer).To(gbytes.Say(`  Repository: some-org/published-repo`))
					Expect(buffer).To(gbytes.Say(`No releases matching version 1.2.3 found, exiting.`))
				})
			})

			context("when a matching draft release does exists", func() {
				it("deletes the draft release and outputs its version", func() {
					command := exec.Command(
						entrypoint,
						"--endpoint", api.URL,
						"--repo", "some-org/draft-repo",
						"--token", "some-github-token",
						"--version", "1.2.3",
					)
					command.Env = []string{
						fmt.Sprintf("GITHUB_OUTPUT=%s", filepath.Join(tempDir, "github-output")),
						fmt.Sprintf("GITHUB_STATE=%s", filepath.Join(tempDir, "github-state")),
					}

					buffer := gbytes.NewBuffer()

					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())

					Eventually(session).Should(gexec.Exit(0), func() string { return fmt.Sprintf("output -> \n%s\n", buffer.Contents()) })

					Expect(requests).To(HaveLen(2))
					Expect(requests[0].Method).To(Equal("GET"))
					Expect(requests[0].URL.Path).To(Equal("/repos/some-org/draft-repo/releases"))
					Expect(requests[1].Method).To(Equal("DELETE"))
					Expect(requests[1].URL.Path).To(Equal("/repos/some-org/draft-repo/releases/3"))

					Expect(buffer).To(gbytes.Say(`Fetching latest releases`))
					Expect(buffer).To(gbytes.Say(`  Repository: some-org/draft-repo`))
					Expect(buffer).To(gbytes.Say(`Matching draft version 1.2.3 found`))
					Expect(buffer).To(gbytes.Say(`Latest release is draft, deleting.`))
					Expect(buffer).To(gbytes.Say(`Success`))

					data, err := os.ReadFile(filepath.Join(tempDir, "github-output"))
					Expect(err).NotTo(HaveOccurred())
					outputs := strings.Split(string(data), "\n")
					Expect(outputs).To(ContainElements("current_version=1.2.3"))
				})
			})
		})

		context("when a draft release does NOT exists", func() {
			it("does not change the repo", func() {
				command := exec.Command(
					entrypoint,
					"--endpoint", api.URL,
					"--repo", "some-org/published-repo",
					"--token", "some-github-token",
				)

				buffer := gbytes.NewBuffer()

				session, err := gexec.Start(command, buffer, buffer)
				Expect(err).NotTo(HaveOccurred())

				Eventually(session).Should(gexec.Exit(0), func() string { return fmt.Sprintf("output -> \n%s\n", buffer.Contents()) })

				Expect(requests).To(HaveLen(1))
				Expect(requests[0].Method).To(Equal("GET"))
				Expect(requests[0].URL.Path).To(Equal("/repos/some-org/published-repo/releases"))

				Expect(buffer).To(gbytes.Say(`Fetching latest releases`))
				Expect(buffer).To(gbytes.Say(`  Repository: some-org/published-repo`))
			})
		})

		context("when a draft release does exists", func() {
			it("deletes the draft release and outputs its version", func() {
				command := exec.Command(
					entrypoint,
					"--endpoint", api.URL,
					"--repo", "some-org/draft-repo",
					"--token", "some-github-token",
				)
				command.Env = []string{
					fmt.Sprintf("GITHUB_OUTPUT=%s", filepath.Join(tempDir, "github-output")),
					fmt.Sprintf("GITHUB_STATE=%s", filepath.Join(tempDir, "github-state")),
				}

				buffer := gbytes.NewBuffer()

				session, err := gexec.Start(command, buffer, buffer)
				Expect(err).NotTo(HaveOccurred())

				Eventually(session).Should(gexec.Exit(0), func() string { return fmt.Sprintf("output -> \n%s\n", buffer.Contents()) })

				Expect(requests).To(HaveLen(2))
				Expect(requests[0].Method).To(Equal("GET"))
				Expect(requests[0].URL.Path).To(Equal("/repos/some-org/draft-repo/releases"))
				Expect(requests[1].Method).To(Equal("DELETE"))
				Expect(requests[1].URL.Path).To(Equal("/repos/some-org/draft-repo/releases/2"))

				Expect(buffer).To(gbytes.Say(`Fetching latest releases`))
				Expect(buffer).To(gbytes.Say(`  Repository: some-org/draft-repo`))
				Expect(buffer).To(gbytes.Say(`Latest release is draft, deleting.`))
				Expect(buffer).To(gbytes.Say(`Success`))

				data, err := os.ReadFile(filepath.Join(tempDir, "github-output"))
				Expect(err).NotTo(HaveOccurred())
				outputs := strings.Split(string(data), "\n")
				Expect(outputs).To(ContainElements("current_version=some-version"))
			})
		})

		context("when no releases exist", func() {
			it("exits without erroring", func() {
				command := exec.Command(
					entrypoint,
					"--endpoint", api.URL,
					"--repo", "some-org/empty-repo",
					"--token", "some-github-token",
				)

				buffer := gbytes.NewBuffer()

				session, err := gexec.Start(command, buffer, buffer)
				Expect(err).NotTo(HaveOccurred())

				Eventually(session).Should(gexec.Exit(0), func() string { return fmt.Sprintf("output -> \n%s\n", buffer.Contents()) })

				Expect(requests).To(HaveLen(1))
				Expect(requests[0].Method).To(Equal("GET"))
				Expect(requests[0].URL.Path).To(Equal("/repos/some-org/empty-repo/releases"))

				Expect(buffer).To(gbytes.Say(`Fetching latest releases`))
				Expect(buffer).To(gbytes.Say(`  Repository: some-org/empty-repo`))
				Expect(buffer).To(gbytes.Say(`No releases, exiting.`))
			})
		})

		context("failure cases", func() {
			context("when the --repo flag is missing", func() {
				it("prints an error message and exits non-zero", func() {
					command := exec.Command(
						entrypoint,
						"--endpoint", api.URL,
						"--token", "some-github-token",
					)
					buffer := gbytes.NewBuffer()

					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())

					Eventually(session).Should(gexec.Exit(1), func() string { return fmt.Sprintf("output:\n%s\n", buffer.Contents()) })

					Expect(buffer).To(gbytes.Say(`Error: missing required input "repo"`))
				})
			})

			context("when the --repo flag is missing", func() {
				it("prints an error message and exits non-zero", func() {
					command := exec.Command(
						entrypoint,
						"--endpoint", api.URL,
						"--repo", "some-org/draft-repo",
					)
					buffer := gbytes.NewBuffer()

					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())
					Eventually(session).Should(gexec.Exit(1), func() string { return fmt.Sprintf("output:\n%s\n", buffer.Contents()) })

					Expect(buffer).To(gbytes.Say(`Error: missing required input "token"`))
				})
			})

			context("when the list releases request cannot be created", func() {
				it("prints an error message and exits non-zero", func() {
					command := exec.Command(
						entrypoint,
						"--endpoint", "%%%",
						"--token", "some-github-token",
						"--repo", "redirect-org/loop-repo",
					)
					buffer := gbytes.NewBuffer()

					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())

					Eventually(session).Should(gexec.Exit(1), func() string { return fmt.Sprintf("output:\n%s\n", buffer.Contents()) })

					Expect(buffer).To(gbytes.Say(`Error: parse "%%%/repos/redirect-org/loop-repo/releases": invalid URL escape "%%%"`))
				})
			})

			context("when the list releases request gets stuck in a loop", func() {
				it("prints an error message and exits non-zero", func() {
					command := exec.Command(
						entrypoint,
						"--endpoint", api.URL,
						"--token", "some-github-token",
						"--repo", "redirect-org/loop-repo",
					)
					buffer := gbytes.NewBuffer()

					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())

					Eventually(session).Should(gexec.Exit(1), func() string { return fmt.Sprintf("output:\n%s\n", buffer.Contents()) })

					Expect(buffer).To(gbytes.Say(`Error: Get "/repos/redirect-org/loop-repo/releases": stopped after 10 redirects`))
				})
			})

			context("when the list releases response has an unexpected status", func() {
				it("prints an error message and exits non-zero", func() {
					command := exec.Command(
						entrypoint,
						"--endpoint", api.URL,
						"--token", "some-github-token",
						"--repo", "some-org/error-repo",
					)
					buffer := gbytes.NewBuffer()

					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())

					Eventually(session).Should(gexec.Exit(1), func() string { return fmt.Sprintf("output:\n%s\n", buffer.Contents()) })

					Expect(buffer).To(gbytes.Say(`Error: unexpected response from list releases request`))
					Expect(buffer).To(gbytes.Say(`500 Internal Server Error`))
					Expect(buffer).To(gbytes.Say(`{"error": "server-error"}`))
				})
			})

			context("when the list releases response body is malformed", func() {
				it("prints an error message and exits non-zero", func() {
					command := exec.Command(
						entrypoint,
						"--endpoint", api.URL,
						"--token", "some-github-token",
						"--repo", "some-org/malformed-json-repo",
					)
					buffer := gbytes.NewBuffer()

					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())

					Eventually(session).Should(gexec.Exit(1), func() string { return fmt.Sprintf("output:\n%s\n", buffer.Contents()) })

					Expect(buffer).To(gbytes.Say(`Error: invalid character '%' looking for beginning of value`))
				})
			})

			context("when delete requests is malformed", func() {
				it("prints an error message and exits non-zero", func() {
					command := exec.Command(
						entrypoint,
						"--endpoint", api.URL,
						"--token", "some-github-token",
						"--repo", "some-org/loop-repo",
					)
					buffer := gbytes.NewBuffer()

					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())

					Eventually(session).Should(gexec.Exit(1), func() string { return fmt.Sprintf("output:\n%s\n", buffer.Contents()) })

					Expect(buffer).To(gbytes.Say(`Error: Delete "/repos/some-org/loop-repo/releases/3": stopped after 10 redirects`))
				})
			})

			context("when delete request returns an unexpected response", func() {
				it("prints an error message and exits non-zero", func() {
					command := exec.Command(
						entrypoint,
						"--endpoint", api.URL,
						"--token", "some-github-token",
						"--repo", "some-org/delete-error-repo",
					)
					buffer := gbytes.NewBuffer()

					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())

					Eventually(session).Should(gexec.Exit(1), func() string { return fmt.Sprintf("output:\n%s\n", buffer.Contents()) })

					Expect(buffer).To(gbytes.Say(`Error: unexpected response from delete draft release request`))
					Expect(buffer).To(gbytes.Say(`500 Internal Server Error`))
					Expect(buffer).To(gbytes.Say(`{"error": "server-error"}`))
				})
			})
			context.Focus("when a specific version is passed in", func() {

			})
		})
	})
}
