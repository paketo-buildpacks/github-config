package main_test

import (
	"bufio"
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"os/exec"
	"strings"
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
	entrypoint, err = gexec.Build("github.com/paketo-buildpacks/github-config/actions/dispatch/entrypoint")
	Expect(err).NotTo(HaveOccurred())

	spec.Run(t, "dispatch", func(t *testing.T, context spec.G, it spec.S) {
		var (
			Expect     = NewWithT(t).Expect
			Eventually = NewWithT(t).Eventually
		)

		context("when given a release event payload", func() {
			var (
				api      *httptest.Server
				requests []*http.Request
			)

			it.Before(func() {
				requests = []*http.Request{}
				api = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
					dump, _ := httputil.DumpRequest(req, true)
					receivedRequest, _ := http.ReadRequest(bufio.NewReader(bytes.NewBuffer(dump)))

					requests = append(requests, receivedRequest)

					if strings.HasPrefix(req.URL.Path, "/repos") {
						if req.Header.Get("Authorization") != "token some-github-token" {
							w.WriteHeader(http.StatusForbidden)
							return
						}
					}

					switch req.URL.Path {
					case "/repos/some-org/some-repo/dispatches":
						w.WriteHeader(http.StatusNoContent)

					case "/repos/some-org/some-other-repo/dispatches":
						w.WriteHeader(http.StatusNoContent)

					case "/repos/loop-org/loop-repo/dispatches":
						w.Header().Set("Location", "/repos/loop-org/loop-repo/dispatches")
						w.WriteHeader(http.StatusFound)

					case "/repos/fail-org/fail-repo/dispatches":
						w.WriteHeader(http.StatusInternalServerError)
						w.Write([]byte(`{"error": "server-error"}`))

					default:
						t.Fatal(fmt.Sprintf("unknown request: %s", dump))
					}
				}))
			})

			it.After(func() {
				api.Close()
			})

			it("sends a repository_dispatch webhook to a repo", func() {
				command := exec.Command(
					entrypoint,
					"--endpoint", api.URL,
					"--repos", "some-org/some-repo",
					"--token", "some-github-token",
					"--event", "some-event",
					"--payload", `{"key": "value"}`,
				)
				buffer := gbytes.NewBuffer()

				session, err := gexec.Start(command, buffer, buffer)
				Expect(err).NotTo(HaveOccurred())

				Eventually(session).Should(gexec.Exit(0), func() string { return fmt.Sprintf("output:\n%s\n", buffer.Contents()) })

				Expect(buffer).To(gbytes.Say(`Dispatching`))
				Expect(buffer).To(gbytes.Say(`  Repository: some-org/some-repo`))
				Expect(buffer).To(gbytes.Say(`Success!`))

				Expect(requests).To(HaveLen(1))

				dispatchRequest := requests[0]
				Expect(dispatchRequest.Method).To(Equal("POST"))
				Expect(dispatchRequest.URL.Path).To(Equal("/repos/some-org/some-repo/dispatches"))

				body, err := ioutil.ReadAll(dispatchRequest.Body)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(body)).To(MatchJSON(`{
				"event_type": "some-event",
				"client_payload": {
					"key": "value"
				}
			}`))
			})

			context("when there are multiple target repos", func() {
				it("sends a repository_dispatch webhook to all target repos", func() {
					command := exec.Command(
						entrypoint,
						"--endpoint", api.URL,
						"--repos", "some-org/some-repo,  some-org/some-other-repo",
						"--token", "some-github-token",
						"--event", "some-event",
						"--payload", `{"key": "value"}`,
					)
					buffer := gbytes.NewBuffer()

					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())

					Eventually(session).Should(gexec.Exit(0), func() string { return fmt.Sprintf("output:\n%s\n", buffer.Contents()) })

					Expect(buffer).To(gbytes.Say(`Dispatching`))

					Expect(buffer).To(gbytes.Say(`  Repository: some-org/some-repo`))
					Expect(buffer).To(gbytes.Say(`Success!`))

					Expect(buffer).To(gbytes.Say(`  Repository: some-org/some-other-repo`))
					Expect(buffer).To(gbytes.Say(`Success!`))

					Expect(requests).To(HaveLen(2))

					dispatchRequest := requests[0]
					Expect(dispatchRequest.Method).To(Equal("POST"))
					Expect(dispatchRequest.URL.Path).To(Equal("/repos/some-org/some-repo/dispatches"))

					dispatchRequest = requests[1]
					Expect(dispatchRequest.Method).To(Equal("POST"))
					Expect(dispatchRequest.URL.Path).To(Equal("/repos/some-org/some-other-repo/dispatches"))

					body, err := ioutil.ReadAll(dispatchRequest.Body)
					Expect(err).NotTo(HaveOccurred())
					Expect(string(body)).To(MatchJSON(`{
				"event_type": "some-event",
				"client_payload": {
					"key": "value"
				}
			}`))
				})
			})

			context("failure cases", func() {
				context("when the --event flag is missing", func() {
					it("prints an error message and exits non-zero", func() {
						command := exec.Command(
							entrypoint,
							"--endpoint", api.URL,
							"--repos", "some-org/some-repo",
							"--token", "some-github-token",
							"--payload", "{}",
						)
						buffer := gbytes.NewBuffer()

						session, err := gexec.Start(command, buffer, buffer)
						Expect(err).NotTo(HaveOccurred())

						Eventually(session).Should(gexec.Exit(1), func() string { return fmt.Sprintf("output:\n%s\n", buffer.Contents()) })

						Expect(buffer).To(gbytes.Say(`Error: missing required input "event"`))
					})
				})

				context("when the --payload flag is missing", func() {
					it("prints an error message and exits non-zero", func() {
						command := exec.Command(
							entrypoint,
							"--endpoint", api.URL,
							"--repos", "some-org/some-repo",
							"--token", "some-github-token",
							"--event", "some-event",
						)
						buffer := gbytes.NewBuffer()

						session, err := gexec.Start(command, buffer, buffer)
						Expect(err).NotTo(HaveOccurred())

						Eventually(session).Should(gexec.Exit(1), func() string { return fmt.Sprintf("output:\n%s\n", buffer.Contents()) })

						Expect(buffer).To(gbytes.Say(`Error: missing required input "payload"`))
					})
				})

				context("when the --repo flag is missing", func() {
					it("prints an error message and exits non-zero", func() {
						command := exec.Command(
							entrypoint,
							"--endpoint", api.URL,
							"--token", "some-github-token",
							"--event", "some-event",
							"--payload", "{}",
						)
						buffer := gbytes.NewBuffer()

						session, err := gexec.Start(command, buffer, buffer)
						Expect(err).NotTo(HaveOccurred())

						Eventually(session).Should(gexec.Exit(1), func() string { return fmt.Sprintf("output:\n%s\n", buffer.Contents()) })

						Expect(buffer).To(gbytes.Say(`Error: missing required input "repo"`))
					})
				})

				context("when the --token flag is missing", func() {
					it("prints an error message and exits non-zero", func() {
						command := exec.Command(
							entrypoint,
							"--endpoint", api.URL,
							"--repos", "some-org/some-repo",
							"--event", "some-event",
							"--payload", "{}",
						)
						buffer := gbytes.NewBuffer()

						session, err := gexec.Start(command, buffer, buffer)
						Expect(err).NotTo(HaveOccurred())

						Eventually(session).Should(gexec.Exit(1), func() string { return fmt.Sprintf("output:\n%s\n", buffer.Contents()) })

						Expect(buffer).To(gbytes.Say(`Error: missing required input "token"`))
					})
				})

				context("when the dispatch request cannot be created", func() {
					it("prints an error message and exits non-zero", func() {
						command := exec.Command(
							entrypoint,
							"--endpoint", "%%%",
							"--repos", "some-org/some-repo",
							"--token", "some-github-token",
							"--event", "some-event",
							"--payload", "{}",
						)
						buffer := gbytes.NewBuffer()

						session, err := gexec.Start(command, buffer, buffer)
						Expect(err).NotTo(HaveOccurred())

						Eventually(session).Should(gexec.Exit(1), func() string { return fmt.Sprintf("output:\n%s\n", buffer.Contents()) })

						Expect(buffer).To(gbytes.Say(`Dispatching`))
						Expect(buffer).To(gbytes.Say(`Error: failed to create dispatch request`))
						Expect(buffer).To(gbytes.Say(`invalid URL escape`))
					})
				})

				context("when the dispatch request cannot be completed", func() {
					it("prints an error message and exits non-zero", func() {
						command := exec.Command(
							entrypoint,
							"--endpoint", api.URL,
							"--repos", "loop-org/loop-repo",
							"--token", "some-github-token",
							"--event", "some-event",
							"--payload", "{}",
						)
						buffer := gbytes.NewBuffer()

						session, err := gexec.Start(command, buffer, buffer)
						Expect(err).NotTo(HaveOccurred())

						Eventually(session).Should(gexec.Exit(1), func() string { return fmt.Sprintf("output:\n%s\n", buffer.Contents()) })

						Expect(buffer).To(gbytes.Say(`Dispatching`))
						Expect(buffer).To(gbytes.Say(`Error: failed to complete dispatch request`))
						Expect(buffer).To(gbytes.Say(`stopped after 10 redirects`))
					})
				})

				context("when the dispatch request response is not success", func() {
					it("prints an error message and exits non-zero", func() {
						command := exec.Command(
							entrypoint,
							"--endpoint", api.URL,
							"--repos", "fail-org/fail-repo",
							"--token", "some-github-token",
							"--event", "some-event",
							"--payload", "{}",
						)
						buffer := gbytes.NewBuffer()

						session, err := gexec.Start(command, buffer, buffer)
						Expect(err).NotTo(HaveOccurred())

						Eventually(session).Should(gexec.Exit(1), func() string { return fmt.Sprintf("output:\n%s\n", buffer.Contents()) })

						Expect(buffer).To(gbytes.Say(`Dispatching`))
						Expect(buffer).To(gbytes.Say(`Error: unexpected response from dispatch request`))
						Expect(buffer).To(gbytes.Say(`500 Internal Server Error`))
						Expect(buffer).To(gbytes.Say(`{"error": "server-error"}`))
					})
				})
			})
		})
	}, spec.Report(report.Terminal{}), spec.Parallel())
}
