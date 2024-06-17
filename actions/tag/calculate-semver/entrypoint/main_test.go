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

	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/sclevine/spec"
)

func TestEntrypoint(t *testing.T) {
	var Expect = NewWithT(t).Expect

	SetDefaultEventuallyTimeout(5 * time.Second)

	var err error
	entrypoint, err := gexec.Build("github.com/paketo-buildpacks/github-config/actions/tag/calculate-semver-tag/entrypoint")
	Expect(err).NotTo(HaveOccurred())

	spec.Run(t, "calculate-semver", func(t *testing.T, context spec.G, it spec.S) {
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

				switch req.URL.Path {
				case
					"/repos/some-org/some-broken-commits-repo",
					"/repos/some-org/some-broken-pulls-repo",
					"/repos/some-org/some-broken-release-repo",
					"/repos/some-org/some-major-repo",
					"/repos/some-org/some-malformed-commits-repo",
					"/repos/some-org/some-malformed-pulls-repo",
					"/repos/some-org/some-malformed-release-repo",
					"/repos/some-org/some-many-label-repo",
					"/repos/some-org/some-minor-repo",
					"/repos/some-org/some-no-label-repo",
					"/repos/some-org/some-no-new-commits-repo",
					"/repos/some-org/some-non-semver-release-repo",
					"/repos/some-org/some-unreleased-repo",
					"/repos/some-org/some-patch-repo",
					"/repos/some-org/some-draft-release-repo":
					w.WriteHeader(http.StatusOK)
					fmt.Fprintln(w, `{
						"message": "Repo exists"
					}`)

				case "/repos/some-org/some-fake-repo":
					w.WriteHeader(http.StatusNotFound)
					fmt.Fprintln(w, `{ "message": "Not found" }`)

				case
					"/repos/some-org/some-broken-commits-repo/releases",
					"/repos/some-org/some-broken-pulls-repo/releases",
					"/repos/some-org/some-major-repo/releases",
					"/repos/some-org/some-malformed-commits-repo/releases",
					"/repos/some-org/some-malformed-pulls-repo/releases",
					"/repos/some-org/some-many-label-repo/releases",
					"/repos/some-org/some-minor-repo/releases",
					"/repos/some-org/some-no-label-repo/releases",
					"/repos/some-org/some-no-new-commits-repo/releases",
					"/repos/some-org/some-patch-repo/releases":
					w.WriteHeader(http.StatusOK)
					fmt.Fprintln(w, `[
						{
							"tag_name": "v1.3.0",
							"draft": true
						},
						{
							"tag_name": "v1.2.3",
							"draft": false
						},
						{
							"tag_name": "v1.1.2",
							"draft": false
						}
					]`)

				case "/repos/some-org/some-unreleased-repo/releases":
					w.WriteHeader(http.StatusNotFound)
					fmt.Fprintln(w, `{ "message": "Not found" }`)

				case "/repos/some-org/some-draft-release-repo/releases":
					w.WriteHeader(http.StatusOK)
					fmt.Fprintln(w, `[
						{
							"tag_name": "v1.3.0",
							"draft": true
						},
						{
							"tag_name": "v1.2.3",
							"draft": true
						}
					]`)

				case "/repos/some-org/some-non-semver-release-repo/releases":
					w.WriteHeader(http.StatusOK)
					fmt.Fprintln(w, `[{ "tag_name": "sentimental_version", "draft": false }]`)

				case
					"/repos/some-org/some-major-repo/compare/v1.2.3...some-ref-name",
					"/repos/some-org/some-minor-repo/compare/v1.2.3...some-ref-name",
					"/repos/some-org/some-patch-repo/compare/v1.2.3...some-ref-name",
					"/repos/some-org/some-patch-repo/compare/v1.1.2...some-ref-name":
					w.WriteHeader(http.StatusOK)
					fmt.Fprintln(w, `{ "commits" : [{ "sha" : "abcdef"}, { "sha" : "ghijklm" }, { "sha" : "openstill" }]}`)

				case
					"/repos/some-org/some-broken-pulls-repo/compare/v1.2.3...some-ref-name",
					"/repos/some-org/some-malformed-pulls-repo/compare/v1.2.3...some-ref-name",
					"/repos/some-org/some-many-label-repo/compare/v1.2.3...some-ref-name",
					"/repos/some-org/some-no-label-repo/compare/v1.2.3...some-ref-name":
					w.WriteHeader(http.StatusOK)
					fmt.Fprintln(w, `{ "commits" : [{ "sha" : "abcdef"}] }`)

				case "/repos/some-org/some-no-new-commits-repo/compare/v1.2.3...some-ref-name":
					w.WriteHeader(http.StatusOK)
					fmt.Fprintln(w, `{ "commits" : []}`)

				case "/repos/some-org/some-patch-repo/commits/abcdef/pulls":
					w.WriteHeader(http.StatusOK)
					fmt.Fprintln(w, `[
					{ "number" : 1,
						"labels" : [
						{ "name" : "semver:patch"},
						{ "name" : "otherLabel" }
						],
						"merged_at" : "some-time"
					}]`)

				case "/repos/some-org/some-patch-repo/commits/ghijklm/pulls":
					w.WriteHeader(http.StatusOK)
					fmt.Fprintln(w, `[
					{ "number" : 1,
						"labels" : [
						{ "name" : "semver:patch"},
						{ "name" : "otherLabel" }
						],
						"merged_at" : "some-time"
					}]`)

				case "/repos/some-org/some-patch-repo/commits/openstill/pulls":
					w.WriteHeader(http.StatusOK)
					fmt.Fprintln(w, `[
					{ "number" : 3,
						"labels" : [
						{ "name" : "otherLabel" }
						]
					}]`)

				case "/repos/some-org/some-minor-repo/commits/abcdef/pulls":
					w.WriteHeader(http.StatusOK)
					fmt.Fprintln(w, `[
					{ "number" : 1,
						"labels" : [
						{ "name" : "semver:patch"},
						{ "name" : "otherLabel" }
						],
						"merged_at" : "some-time"
					}]`)

				case "/repos/some-org/some-minor-repo/commits/ghijklm/pulls":
					w.WriteHeader(http.StatusOK)
					fmt.Fprintln(w, `[
					{ "number" : 3,
						"labels" : [
						{ "name" : "semver:minor"},
						{ "name" : "otherLabel" }
						],
						"merged_at" : "some-time"
					}]`)

				case "/repos/some-org/some-minor-repo/commits/openstill/pulls":
					w.WriteHeader(http.StatusOK)
					fmt.Fprintln(w, `[
					{ "number" : 3,
						"labels" : [
						{ "name" : "otherLabel" }
						]
					}]`)

				case "/repos/some-org/some-major-repo/commits/abcdef/pulls":
					w.WriteHeader(http.StatusOK)
					fmt.Fprintln(w, `[
					{ "number" : 1,
						"labels" : [
						{ "name" : "semver:major"},
						{ "name" : "otherLabel" }
						],
						"merged_at" : "some-time"
					}]`)

				case "/repos/some-org/some-major-repo/commits/ghijklm/pulls":
					w.WriteHeader(http.StatusOK)
					fmt.Fprintln(w, `[
					{ "number" : 2,
						"labels" : [
						{ "name" : "semver:minor"},
						{ "name" : "otherLabel" }
						],
						"merged_at" : "some-time"
					}]`)

				case "/repos/some-org/some-major-repo/commits/openstill/pulls":
					w.WriteHeader(http.StatusOK)
					fmt.Fprintln(w, `[
					{ "number" : 3,
						"labels" : [
						{ "name" : "otherLabel" }
						]
					}]`)

				case "/repos/some-org/some-no-label-repo/commits/abcdef/pulls":
					w.WriteHeader(http.StatusOK)
					fmt.Fprintln(w, `[
					{ "number" : 1,
						"labels" : [
						{ "name" : "otherLabel" }
						],
						"merged_at" : "some-time"
					}]`)

				case "/repos/some-org/some-many-label-repo/commits/abcdef/pulls":
					w.WriteHeader(http.StatusOK)
					fmt.Fprintln(w, `[
					{ "number" : 1,
						"labels" : [
						{ "name" : "semver:minor" },
						{ "name" : "semver:major" }
						],
						"merged_at" : "some-time"
					}]`)

				case
					"/repos/some-org/some-broken-commits-repo/compare/v1.2.3...some-ref-name",
					"/repos/some-org/some-broken-pulls-repo/commits/abcdef/pulls",
					"/repos/some-org/some-broken-release-repo/releases":
					w.WriteHeader(http.StatusInternalServerError)
					fmt.Fprintln(w, `{ "message": "Internal Error" }`)

				case
					"/repos/some-org/some-malformed-commits-repo/compare/v1.2.3...some-ref-name",
					"/repos/some-org/some-malformed-pulls-repo/commits/abcdef/pulls",
					"/repos/some-org/some-malformed-release-repo/releases":
					w.WriteHeader(http.StatusOK)
					fmt.Fprintln(w, `{ "message": "Malformed JSON }`)

				default:
					t.Fatalf("unknown request: %s", dump)
				}
			}))

			tempDir = t.TempDir()
		})

		outputContains := func(elements ...string) {
			data, err := os.ReadFile(filepath.Join(tempDir, "github-output"))
			Expect(err).NotTo(HaveOccurred())
			outputs := strings.Split(string(data), "\n")
			Expect(outputs).To(ContainElements(elements))
		}

		context("all PRs since the last release have semver:patch", func() {
			it("increments the patch from the previous version", func() {
				command := exec.Command(
					entrypoint,
					"--endpoint", api.URL,
					"--repo", "some-org/some-patch-repo",
					"--token", "some-github-token",
					"--ref-name", "some-ref-name",
				)
				command.Env = []string{
					fmt.Sprintf("GITHUB_OUTPUT=%s", filepath.Join(tempDir, "github-output")),
					fmt.Sprintf("GITHUB_STATE=%s", filepath.Join(tempDir, "github-state")),
				}

				buffer := gbytes.NewBuffer()

				session, err := gexec.Start(command, buffer, buffer)
				Expect(err).NotTo(HaveOccurred())

				Eventually(session).Should(gexec.Exit(0), func() string { return fmt.Sprintf("output -> \n%s\n", buffer.Contents()) })

				Expect(requests).To(HaveLen(6))

				Expect(requests[0].URL.Path).To(Equal("/repos/some-org/some-patch-repo"))
				Expect(requests[1].URL.Path).To(Equal("/repos/some-org/some-patch-repo/releases"))
				Expect(requests[2].URL.Path).To(Equal("/repos/some-org/some-patch-repo/compare/v1.2.3...some-ref-name"))
				Expect(requests[3].URL.Path).To(Equal("/repos/some-org/some-patch-repo/commits/abcdef/pulls"))
				Expect(requests[4].URL.Path).To(Equal("/repos/some-org/some-patch-repo/commits/ghijklm/pulls"))
				Expect(requests[5].URL.Path).To(Equal("/repos/some-org/some-patch-repo/commits/openstill/pulls"))

				outputContains(`tag=1.2.4`)
			})
		})

		context("some PR since last release has semver:minor", func() {
			it("increments the minor from the previous version", func() {
				command := exec.Command(
					entrypoint,
					"--endpoint", api.URL,
					"--repo", "some-org/some-minor-repo",
					"--token", "some-github-token",
					"--ref-name", "some-ref-name",
				)
				command.Env = []string{
					fmt.Sprintf("GITHUB_OUTPUT=%s", filepath.Join(tempDir, "github-output")),
					fmt.Sprintf("GITHUB_STATE=%s", filepath.Join(tempDir, "github-state")),
				}

				buffer := gbytes.NewBuffer()

				session, err := gexec.Start(command, buffer, buffer)
				Expect(err).NotTo(HaveOccurred())

				Eventually(session).Should(gexec.Exit(0), func() string { return fmt.Sprintf("output -> \n%s\n", buffer.Contents()) })

				Expect(requests).To(HaveLen(6))

				Expect(requests[0].URL.Path).To(Equal("/repos/some-org/some-minor-repo"))
				Expect(requests[1].URL.Path).To(Equal("/repos/some-org/some-minor-repo/releases"))
				Expect(requests[2].URL.Path).To(Equal("/repos/some-org/some-minor-repo/compare/v1.2.3...some-ref-name"))
				Expect(requests[3].URL.Path).To(Equal("/repos/some-org/some-minor-repo/commits/abcdef/pulls"))
				Expect(requests[4].URL.Path).To(Equal("/repos/some-org/some-minor-repo/commits/ghijklm/pulls"))
				Expect(requests[5].URL.Path).To(Equal("/repos/some-org/some-minor-repo/commits/openstill/pulls"))

				outputContains(`tag=1.3.0`)
			})
		})

		context("some PR since last release has semver:major", func() {
			it("increments the major from the previous version", func() {
				command := exec.Command(
					entrypoint,
					"--endpoint", api.URL,
					"--repo", "some-org/some-major-repo",
					"--token", "some-github-token",
					"--ref-name", "some-ref-name",
				)
				command.Env = []string{
					fmt.Sprintf("GITHUB_OUTPUT=%s", filepath.Join(tempDir, "github-output")),
					fmt.Sprintf("GITHUB_STATE=%s", filepath.Join(tempDir, "github-state")),
				}

				buffer := gbytes.NewBuffer()

				session, err := gexec.Start(command, buffer, buffer)
				Expect(err).NotTo(HaveOccurred())

				Eventually(session).Should(gexec.Exit(0), func() string { return fmt.Sprintf("output -> \n%s\n", buffer.Contents()) })

				Expect(requests).To(HaveLen(6))

				Expect(requests[0].URL.Path).To(Equal("/repos/some-org/some-major-repo"))
				Expect(requests[1].URL.Path).To(Equal("/repos/some-org/some-major-repo/releases"))
				Expect(requests[2].URL.Path).To(Equal("/repos/some-org/some-major-repo/compare/v1.2.3...some-ref-name"))
				Expect(requests[3].URL.Path).To(Equal("/repos/some-org/some-major-repo/commits/abcdef/pulls"))
				Expect(requests[4].URL.Path).To(Equal("/repos/some-org/some-major-repo/commits/ghijklm/pulls"))
				Expect(requests[5].URL.Path).To(Equal("/repos/some-org/some-major-repo/commits/openstill/pulls"))

				outputContains(`tag=2.0.0`)
			})
		})

		context("a repo has no releases", func() {
			it("returns v0.0.1 and doesn't look at PRs", func() {
				command := exec.Command(
					entrypoint,
					"--endpoint", api.URL,
					"--repo", "some-org/some-unreleased-repo",
					"--token", "some-github-token",
					"--ref-name", "some-ref-name",
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

				Expect(requests[0].URL.Path).To(Equal("/repos/some-org/some-unreleased-repo"))
				Expect(requests[1].URL.Path).To(Equal("/repos/some-org/some-unreleased-repo/releases"))

				outputContains(`tag=0.0.1`)
			})
		})

		context("when there are no published semver versioned releases", func() {
			it("prints an warning and sets output to v0.0.1", func() {
				command := exec.Command(
					entrypoint,
					"--endpoint", api.URL,
					"--repo", "some-org/some-non-semver-release-repo",
					"--token", "some-github-token",
					"--ref-name", "some-ref-name",
				)
				command.Env = []string{
					fmt.Sprintf("GITHUB_OUTPUT=%s", filepath.Join(tempDir, "github-output")),
					fmt.Sprintf("GITHUB_STATE=%s", filepath.Join(tempDir, "github-state")),
				}

				buffer := gbytes.NewBuffer()

				session, err := gexec.Start(command, buffer, buffer)
				Expect(err).NotTo(HaveOccurred())

				Eventually(session).Should(gexec.Exit(0), func() string { return fmt.Sprintf("output -> \n%s\n", buffer.Contents()) })
				Expect(buffer).To(gbytes.Say(`No semantically versioned published releases found`))

				Expect(requests).To(HaveLen(2))

				Expect(requests[0].URL.Path).To(Equal("/repos/some-org/some-non-semver-release-repo"))
				Expect(requests[1].URL.Path).To(Equal("/repos/some-org/some-non-semver-release-repo/releases"))
				outputContains(`tag=0.0.1`)
			})
		})

		context("there have been no PRs since the last release", func() {
			it("increments the patch of the previous version", func() {
				command := exec.Command(
					entrypoint,
					"--endpoint", api.URL,
					"--repo", "some-org/some-no-new-commits-repo",
					"--token", "some-github-token",
					"--ref-name", "some-ref-name",
				)
				command.Env = []string{
					fmt.Sprintf("GITHUB_OUTPUT=%s", filepath.Join(tempDir, "github-output")),
					fmt.Sprintf("GITHUB_STATE=%s", filepath.Join(tempDir, "github-state")),
				}

				buffer := gbytes.NewBuffer()

				session, err := gexec.Start(command, buffer, buffer)
				Expect(err).NotTo(HaveOccurred())

				Eventually(session).Should(gexec.Exit(0), func() string { return fmt.Sprintf("output -> \n%s\n", buffer.Contents()) })

				Expect(requests).To(HaveLen(3))

				Expect(requests[0].URL.Path).To(Equal("/repos/some-org/some-no-new-commits-repo"))
				Expect(requests[1].URL.Path).To(Equal("/repos/some-org/some-no-new-commits-repo/releases"))
				Expect(requests[2].URL.Path).To(Equal("/repos/some-org/some-no-new-commits-repo/compare/v1.2.3...some-ref-name"))

				outputContains("tag=1.2.4")
			})
		})

		context("a latest version to base calculations off of is passed in", func() {
			it("the version passed in is used instead of the latest release", func() {
				command := exec.Command(
					entrypoint,
					"--endpoint", api.URL,
					"--repo", "some-org/some-patch-repo",
					"--token", "some-github-token",
					"--ref-name", "some-ref-name",
					"--latest-version", "v1.1.2",
				)
				command.Env = []string{
					fmt.Sprintf("GITHUB_OUTPUT=%s", filepath.Join(tempDir, "github-output")),
					fmt.Sprintf("GITHUB_STATE=%s", filepath.Join(tempDir, "github-state")),
				}

				buffer := gbytes.NewBuffer()

				session, err := gexec.Start(command, buffer, buffer)
				Expect(err).NotTo(HaveOccurred())

				Eventually(session).Should(gexec.Exit(0), func() string { return fmt.Sprintf("output -> \n%s\n", buffer.Contents()) })

				Expect(requests).To(HaveLen(6))

				Expect(requests[0].URL.Path).To(Equal("/repos/some-org/some-patch-repo"))
				Expect(requests[1].URL.Path).To(Equal("/repos/some-org/some-patch-repo/releases"))
				Expect(requests[2].URL.Path).To(Equal("/repos/some-org/some-patch-repo/compare/v1.1.2...some-ref-name"))
				Expect(requests[3].URL.Path).To(Equal("/repos/some-org/some-patch-repo/commits/abcdef/pulls"))
				Expect(requests[4].URL.Path).To(Equal("/repos/some-org/some-patch-repo/commits/ghijklm/pulls"))
				Expect(requests[5].URL.Path).To(Equal("/repos/some-org/some-patch-repo/commits/openstill/pulls"))

				outputContains(`tag=1.1.3`)
			})

			context("there are no prior published releases on the repository", func() {
				it("the version passed in is used as the output version", func() {
					command := exec.Command(
						entrypoint,
						"--endpoint", api.URL,
						"--repo", "some-org/some-draft-release-repo",
						"--token", "some-github-token",
						"--ref-name", "some-ref-name",
						"--latest-version", "v1.2.3",
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

					Expect(requests[0].URL.Path).To(Equal("/repos/some-org/some-draft-release-repo"))
					Expect(requests[1].URL.Path).To(Equal("/repos/some-org/some-draft-release-repo/releases"))
					Expect(buffer).To(gbytes.Say("First release in the new version line, using `latest-version` as output"))
					outputContains(`tag=1.2.3`)
				})
			})

			context("the version passed in the first release in a new version line", func() {
				it("the version passed in is used as the output version", func() {
					command := exec.Command(
						entrypoint,
						"--endpoint", api.URL,
						"--repo", "some-org/some-patch-repo",
						"--token", "some-github-token",
						"--ref-name", "some-ref-name",
						"--latest-version", "v1.3.0",
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

					Expect(requests[0].URL.Path).To(Equal("/repos/some-org/some-patch-repo"))
					Expect(requests[1].URL.Path).To(Equal("/repos/some-org/some-patch-repo/releases"))
					Expect(buffer).To(gbytes.Say("First release in the new version line, using `latest-version` as output"))
					outputContains(`tag=1.3.0`)
				})
			})
		})

		context("failure cases", func() {
			context("when missing the repo flag", func() {
				it("prints an error and exits non-zero", func() {
					command := exec.Command(
						entrypoint,
						"--endpoint", api.URL,
						"--token", "some-github-token",
						"--ref-name", "some-ref-name",
					)

					buffer := gbytes.NewBuffer()

					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())

					Eventually(session).Should(gexec.Exit(1), func() string { return fmt.Sprintf("output -> \n%s\n", buffer.Contents()) })

					Expect(buffer).To(gbytes.Say(`Error: missing required input "repo"`))
				})
			})

			context("when missing the token flag", func() {
				it("prints an error and exits non-zero", func() {
					command := exec.Command(
						entrypoint,
						"--endpoint", api.URL,
						"--repo", "some-org/some-repo",
						"--ref-name", "some-ref-name",
					)

					buffer := gbytes.NewBuffer()

					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())

					Eventually(session).Should(gexec.Exit(1), func() string { return fmt.Sprintf("output -> \n%s\n", buffer.Contents()) })

					Expect(buffer).To(gbytes.Say(`Error: missing required input "token"`))
				})
			})

			context("when missing the ref-name flag", func() {
				it("prints an error and exits non-zero", func() {
					command := exec.Command(
						entrypoint,
						"--endpoint", api.URL,
						"--repo", "some-org/some-repo",
						"--token", "some-github-token",
					)

					buffer := gbytes.NewBuffer()

					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())

					Eventually(session).Should(gexec.Exit(1), func() string { return fmt.Sprintf("output -> \n%s\n", buffer.Contents()) })

					Expect(buffer).To(gbytes.Say(`Error: missing required input "ref-name"`))
				})
			})

			context("when endpoint is malformed", func() {
				it("prints an error and exits non-zero", func() {
					command := exec.Command(
						entrypoint,
						"--endpoint", "%%%",
						"--repo", "some-org/some-repo",
						"--token", "some-github-token",
						"--ref-name", "some-ref-name",
					)

					buffer := gbytes.NewBuffer()

					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())

					Eventually(session).Should(gexec.Exit(1), func() string { return fmt.Sprintf("output -> \n%s\n", buffer.Contents()) })

					Expect(buffer).To(gbytes.Say(`.* invalid URL escape .*`))
				})
			})

			context("when the repo doesn't exist", func() {
				it("prints an error and exits non-zero", func() {
					command := exec.Command(
						entrypoint,
						"--endpoint", api.URL,
						"--repo", "some-org/some-fake-repo",
						"--token", "some-github-token",
						"--ref-name", "some-ref-name",
					)

					buffer := gbytes.NewBuffer()

					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())

					Eventually(session).Should(gexec.Exit(1), func() string { return fmt.Sprintf("output -> \n%s\n", buffer.Contents()) })

					Expect(buffer).To(gbytes.Say(`failed to get repo`))
				})
			})

			context("when an error occurs getting the latest release", func() {
				it("prints an error and exits non-zero", func() {
					command := exec.Command(
						entrypoint,
						"--endpoint", api.URL,
						"--repo", "some-org/some-broken-release-repo",
						"--token", "some-github-token",
						"--ref-name", "some-ref-name",
					)

					buffer := gbytes.NewBuffer()

					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())

					Eventually(session).Should(gexec.Exit(1), func() string { return fmt.Sprintf("output -> \n%s\n", buffer.Contents()) })

					Expect(buffer).To(gbytes.Say(`failed to get latest release: unexpected response`))
				})
			})

			context("when the release response can't be decoded", func() {
				it("prints an error and exits non-zero", func() {
					command := exec.Command(
						entrypoint,
						"--endpoint", api.URL,
						"--repo", "some-org/some-malformed-release-repo",
						"--token", "some-github-token",
						"--ref-name", "some-ref-name",
					)

					buffer := gbytes.NewBuffer()

					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())

					Eventually(session).Should(gexec.Exit(1), func() string { return fmt.Sprintf("output -> \n%s\n", buffer.Contents()) })

					Expect(buffer).To(gbytes.Say(`failed to decode releases:`))
				})
			})

			context("a latest version to base calculations off of is passed in", func() {
				context("latest version is not a semantic version", func() {
					it("returns an error", func() {
						command := exec.Command(
							entrypoint,
							"--endpoint", api.URL,
							"--repo", "some-org/some-patch-repo",
							"--token", "some-github-token",
							"--ref-name", "some-ref-name",
							"--latest-version", "bad-version",
						)

						buffer := gbytes.NewBuffer()

						session, err := gexec.Start(command, buffer, buffer)
						Expect(err).NotTo(HaveOccurred())

						Eventually(session).Should(gexec.Exit(1), func() string { return fmt.Sprintf("output -> \n%s\n", buffer.Contents()) })

						Expect(buffer).To(gbytes.Say(`--latest-version is not a well-formed semantic version`))
					})
				})
			})

			context("when there is an error fetching commits since the last release", func() {
				it("prints an error and exits non-zero", func() {
					command := exec.Command(
						entrypoint,
						"--endpoint", api.URL,
						"--repo", "some-org/some-broken-commits-repo",
						"--token", "some-github-token",
						"--ref-name", "some-ref-name",
					)

					buffer := gbytes.NewBuffer()

					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())

					Eventually(session).Should(gexec.Exit(1), func() string { return fmt.Sprintf("output -> \n%s\n", buffer.Contents()) })

					Expect(buffer).To(gbytes.Say(`failed to get commits since last release: unexpected response`))
				})
			})

			context("when the commits response can't be decoded", func() {
				it("prints an error and exits non-zero", func() {
					command := exec.Command(
						entrypoint,
						"--endpoint", api.URL,
						"--repo", "some-org/some-malformed-commits-repo",
						"--token", "some-github-token",
						"--ref-name", "some-ref-name",
					)

					buffer := gbytes.NewBuffer()

					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())

					Eventually(session).Should(gexec.Exit(1), func() string { return fmt.Sprintf("output -> \n%s\n", buffer.Contents()) })

					Expect(buffer).To(gbytes.Say(`failed to parse commits since release response:`))
				})
			})

			context("when there is an error fetching PRs for a commit", func() {
				it("prints an error and exits non-zero", func() {
					command := exec.Command(
						entrypoint,
						"--endpoint", api.URL,
						"--repo", "some-org/some-broken-pulls-repo",
						"--token", "some-github-token",
						"--ref-name", "some-ref-name",
					)

					buffer := gbytes.NewBuffer()

					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())

					Eventually(session).Should(gexec.Exit(1), func() string { return fmt.Sprintf("output -> \n%s\n", buffer.Contents()) })

					Expect(buffer).To(gbytes.Say(`failed to get pull requests for commit: unexpected response`))
				})
			})

			context("when the PRs response can't be decoded", func() {
				it("prints an error and exits non-zero", func() {
					command := exec.Command(
						entrypoint,
						"--endpoint", api.URL,
						"--repo", "some-org/some-malformed-pulls-repo",
						"--token", "some-github-token",
						"--ref-name", "some-ref-name",
					)

					buffer := gbytes.NewBuffer()

					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())

					Eventually(session).Should(gexec.Exit(1), func() string { return fmt.Sprintf("output -> \n%s\n", buffer.Contents()) })

					Expect(buffer).To(gbytes.Say(`failed to parse commit PRs response:`))
				})
			})

			context("when a PR has no semver label", func() {
				it("prints an error and exits non-zero", func() {
					command := exec.Command(
						entrypoint,
						"--endpoint", api.URL,
						"--repo", "some-org/some-no-label-repo",
						"--token", "some-github-token",
						"--ref-name", "some-ref-name",
					)

					buffer := gbytes.NewBuffer()

					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())

					Eventually(session).Should(gexec.Exit(1), func() string { return fmt.Sprintf("output -> \n%s\n", buffer.Contents()) })

					Expect(buffer).To(gbytes.Say(`PR 1 has no semver label`))
				})
			})

			context("when a PR has multiple semver labels", func() {
				it("prints an error and exits non-zero", func() {
					command := exec.Command(
						entrypoint,
						"--endpoint", api.URL,
						"--repo", "some-org/some-many-label-repo",
						"--token", "some-github-token",
						"--ref-name", "some-ref-name",
					)

					buffer := gbytes.NewBuffer()

					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())

					Eventually(session).Should(gexec.Exit(1), func() string { return fmt.Sprintf("output -> \n%s\n", buffer.Contents()) })

					Expect(buffer).To(gbytes.Say(`PR 1 has multiple semver labels`))
				})
			})
		})
	})
}
