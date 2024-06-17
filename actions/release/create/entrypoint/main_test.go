package main_test

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
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
	entrypoint, err := gexec.Build("github.com/paketo-buildpacks/github-config/actions/release/create/entrypoint")
	Expect(err).NotTo(HaveOccurred())

	spec.Run(t, "create", func(t *testing.T, context spec.G, it spec.S) {
		var (
			Expect     = NewWithT(t).Expect
			Eventually = NewWithT(t).Eventually

			api      *httptest.Server
			requests []*http.Request
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
				case "/repos/some-org/some-repo/releases":
					w.WriteHeader(http.StatusCreated)
					fmt.Fprintf(w, `{
						"id": 1,
						"upload_url": "%s/repos/some-org/some-repo/releases/1/assets{?name,label}"
					}`, api.URL)

				case "/repos/some-org/some-repo/releases/1":
					fmt.Fprintf(w, `{
						"id": 1,
						"upload_url": "%s/repos/some-org/some-repo/releases/1/assets{?name,label}"
					}`, api.URL)

				case "/repos/some-org/some-repo/releases/1/assets":
					w.WriteHeader(http.StatusCreated)

				case "/repos/some-org/some-redirecting-repo/releases":
					w.Header().Set("Location", req.URL.Path)
					w.WriteHeader(http.StatusFound)

				case "/repos/some-org/some-create-error-repo/releases":
					w.WriteHeader(http.StatusInternalServerError)
					fmt.Fprintln(w, `{"error": "message"}`)

				case "/repos/some-org/some-create-malformed-repo/releases":
					w.WriteHeader(http.StatusCreated)
					fmt.Fprintln(w, `%%%`)

				case "/repos/some-org/some-malformed-upload-url-repo/releases":
					w.WriteHeader(http.StatusCreated)
					fmt.Fprintln(w, `{
						"id": 1,
						"upload_url": "%%%"
					}`)

				case "/repos/some-org/some-upload-redirect-repo/releases":
					w.WriteHeader(http.StatusCreated)
					fmt.Fprintf(w, `{
						"id": 1,
						"upload_url": "%s/repos/some-org/some-upload-redirect-repo/releases/1/assets{?name,label}"
					}`, api.URL)

				case "/repos/some-org/some-upload-redirect-repo/releases/1/assets":
					w.Header().Set("Location", req.URL.Path)
					w.WriteHeader(http.StatusFound)

				case "/repos/some-org/some-upload-error-repo/releases":
					w.WriteHeader(http.StatusCreated)
					fmt.Fprintf(w, `{
						"id": 1,
						"upload_url": "%s/repos/some-org/some-upload-error-repo/releases/1/assets{?name,label}"
					}`, api.URL)

				case "/repos/some-org/some-upload-error-repo/releases/1/assets":
					w.WriteHeader(http.StatusInternalServerError)
					fmt.Fprintln(w, `{"error": "message"}`)

				case "/repos/some-org/some-edit-redirect-repo/releases":
					w.WriteHeader(http.StatusCreated)
					fmt.Fprintln(w, `{
						"id": 1
					}`)

				case "/repos/some-org/some-edit-redirect-repo/releases/1":
					w.Header().Set("Location", req.URL.Path)
					w.WriteHeader(http.StatusFound)

				case "/repos/some-org/some-edit-error-repo/releases":
					w.WriteHeader(http.StatusCreated)
					fmt.Fprintln(w, `{
						"id": 1
					}`)

				case "/repos/some-org/some-edit-error-repo/releases/1":
					w.WriteHeader(http.StatusInternalServerError)
					fmt.Fprintln(w, `{"error": "message"}`)

				default:
					t.Fatal(fmt.Sprintf("unknown request: %s", dump))
				}
			}))
		})

		it("creates a release", func() {
			command := exec.Command(
				entrypoint,
				"--endpoint", api.URL,
				"--repo", "some-org/some-repo",
				"--token", "some-github-token",
				"--tag-name", "some-tag",
				"--target-commitish", "some-commitish",
				"--name", "some-name",
				"--body", "some-body",
			)

			buffer := gbytes.NewBuffer()

			session, err := gexec.Start(command, buffer, buffer)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session).Should(gexec.Exit(0), func() string { return fmt.Sprintf("output -> \n%s\n", buffer.Contents()) })

			Expect(requests).To(HaveLen(2))

			Expect(requests[0].Method).To(Equal("POST"))
			Expect(requests[0].URL.Path).To(Equal("/repos/some-org/some-repo/releases"))

			content, err := io.ReadAll(requests[0].Body)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(content)).To(MatchJSON(`{
				"tag_name": "some-tag",
				"target_commitish": "some-commitish",
				"name": "some-name",
				"body": "some-body",
				"draft": true
			}`))

			Expect(requests[1].Method).To(Equal("PATCH"))
			Expect(requests[1].URL.Path).To(Equal("/repos/some-org/some-repo/releases/1"))

			content, err = io.ReadAll(requests[1].Body)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(content)).To(MatchJSON(`{
				"draft": false
			}`))

			Expect(buffer).To(gbytes.Say(`Creating release`))
			Expect(buffer).To(gbytes.Say(`  Repository: some-org/some-repo`))
			Expect(buffer).To(gbytes.Say(`Release is published, exiting.`))
		})

		it("creates a release from a file", func() {
			command := exec.Command(
				entrypoint,
				"--endpoint", api.URL,
				"--repo", "some-org/some-repo",
				"--token", "some-github-token",
				"--tag-name", "some-tag",
				"--target-commitish", "some-commitish",
				"--name", "some-name",
				"--body-filepath", "./testdata/body.txt",
			)

			buffer := gbytes.NewBuffer()

			session, err := gexec.Start(command, buffer, buffer)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session).Should(gexec.Exit(0), func() string { return fmt.Sprintf("output -> \n%s\n", buffer.Contents()) })

			Expect(requests).To(HaveLen(2))

			Expect(requests[0].Method).To(Equal("POST"))
			Expect(requests[0].URL.Path).To(Equal("/repos/some-org/some-repo/releases"))

			content, err := io.ReadAll(requests[0].Body)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(content)).To(MatchJSON(`{
				"tag_name": "some-tag",
				"target_commitish": "some-commitish",
				"name": "some-name",
				"body": "some-body",
				"draft": true
			}`))

			Expect(requests[1].Method).To(Equal("PATCH"))
			Expect(requests[1].URL.Path).To(Equal("/repos/some-org/some-repo/releases/1"))

			content, err = io.ReadAll(requests[1].Body)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(content)).To(MatchJSON(`{
				"draft": false
			}`))

			Expect(buffer).To(gbytes.Say(`Creating release`))
			Expect(buffer).To(gbytes.Say(`  Repository: some-org/some-repo`))
			Expect(buffer).To(gbytes.Say(`Release is published, exiting.`))
		})

		context("when creating a release and both body and body-filepath are defined", func() {

			it("the body flag has precedence", func() {
				command := exec.Command(
					entrypoint,
					"--endpoint", api.URL,
					"--repo", "some-org/some-repo",
					"--token", "some-github-token",
					"--tag-name", "some-tag",
					"--target-commitish", "some-commitish",
					"--name", "some-name",
					"--body", "some-body-from-flag",
					"--body-filepath", "./testdata/body.txt",
				)

				buffer := gbytes.NewBuffer()

				session, err := gexec.Start(command, buffer, buffer)
				Expect(err).NotTo(HaveOccurred())

				Eventually(session).Should(gexec.Exit(0), func() string { return fmt.Sprintf("output -> \n%s\n", buffer.Contents()) })

				Expect(requests).To(HaveLen(2))

				Expect(requests[0].Method).To(Equal("POST"))
				Expect(requests[0].URL.Path).To(Equal("/repos/some-org/some-repo/releases"))

				content, err := io.ReadAll(requests[0].Body)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(content)).To(MatchJSON(`{
				"tag_name": "some-tag",
				"target_commitish": "some-commitish",
				"name": "some-name",
				"body": "some-body-from-flag",
				"draft": true
			}`))

				Expect(requests[1].Method).To(Equal("PATCH"))
				Expect(requests[1].URL.Path).To(Equal("/repos/some-org/some-repo/releases/1"))

				content, err = io.ReadAll(requests[1].Body)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(content)).To(MatchJSON(`{
				"draft": false
			}`))

				Expect(buffer).To(gbytes.Say(`Creating release`))
				Expect(buffer).To(gbytes.Say(`  Repository: some-org/some-repo`))
				Expect(buffer).To(gbytes.Say(`Release is published, exiting.`))
			})
		})

		context("when creating a draft release", func() {
			it("creates a release", func() {
				command := exec.Command(
					entrypoint,
					"--endpoint", api.URL,
					"--repo", "some-org/some-repo",
					"--token", "some-github-token",
					"--tag-name", "some-tag",
					"--target-commitish", "some-commitish",
					"--name", "some-name",
					"--body", "some-body",
					"--draft",
				)

				buffer := gbytes.NewBuffer()

				session, err := gexec.Start(command, buffer, buffer)
				Expect(err).NotTo(HaveOccurred())

				Eventually(session).Should(gexec.Exit(0), func() string { return fmt.Sprintf("output -> \n%s\n", buffer.Contents()) })

				Expect(requests).To(HaveLen(1))

				Expect(requests[0].Method).To(Equal("POST"))
				Expect(requests[0].URL.Path).To(Equal("/repos/some-org/some-repo/releases"))

				content, err := io.ReadAll(requests[0].Body)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(content)).To(MatchJSON(`{
					"tag_name": "some-tag",
					"target_commitish": "some-commitish",
					"name": "some-name",
					"body": "some-body",
					"draft": true
				}`))

				Expect(buffer).To(gbytes.Say(`Creating release`))
				Expect(buffer).To(gbytes.Say(`  Repository: some-org/some-repo`))
				Expect(buffer).To(gbytes.Say(`Release is drafted, exiting.`))
			})
		})

		context("when the body field is omitted", func() {
			it("creates a release without a body", func() {
				command := exec.Command(
					entrypoint,
					"--endpoint", api.URL,
					"--repo", "some-org/some-repo",
					"--token", "some-github-token",
					"--tag-name", "some-tag",
					"--target-commitish", "some-commitish",
					"--name", "some-name",
				)

				buffer := gbytes.NewBuffer()

				session, err := gexec.Start(command, buffer, buffer)
				Expect(err).NotTo(HaveOccurred())

				Eventually(session).Should(gexec.Exit(0), func() string { return fmt.Sprintf("output -> \n%s\n", buffer.Contents()) })

				Expect(requests).To(HaveLen(2))

				Expect(requests[0].Method).To(Equal("POST"))
				Expect(requests[0].URL.Path).To(Equal("/repos/some-org/some-repo/releases"))

				content, err := io.ReadAll(requests[0].Body)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(content)).To(MatchJSON(`{
					"tag_name": "some-tag",
					"target_commitish": "some-commitish",
					"name": "some-name",
					"draft": true
				}`))

				Expect(requests[1].Method).To(Equal("PATCH"))
				Expect(requests[1].URL.Path).To(Equal("/repos/some-org/some-repo/releases/1"))

				content, err = io.ReadAll(requests[1].Body)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(content)).To(MatchJSON(`{
				"draft": false
			}`))

				Expect(buffer).To(gbytes.Say(`Creating release`))
				Expect(buffer).To(gbytes.Say(`  Repository: some-org/some-repo`))
				Expect(buffer).To(gbytes.Say(`Release is published, exiting.`))
			})
		})

		context("when there are assets for the release", func() {
			var tmpDir string

			it.Before(func() {
				var err error
				tmpDir, err = os.MkdirTemp("", "assets")
				Expect(err).NotTo(HaveOccurred())

				err = os.WriteFile(filepath.Join(tmpDir, "some-asset"), []byte("some-contents"), 0644)
				Expect(err).NotTo(HaveOccurred())

				err = os.WriteFile(filepath.Join(tmpDir, "other-asset"), []byte("other-contents"), 0644)
				Expect(err).NotTo(HaveOccurred())
			})

			it.After(func() {
				Expect(os.RemoveAll(tmpDir)).To(Succeed())
			})

			it("creates a release", func() {
				command := exec.Command(
					entrypoint,
					"--endpoint", api.URL,
					"--repo", "some-org/some-repo",
					"--token", "some-github-token",
					"--tag-name", "some-tag",
					"--target-commitish", "some-commitish",
					"--name", "some-name",
					"--body", "some-body",
					"--draft",
					"--assets", fmt.Sprintf(`[
						{
						  "path": "%s",
							"name": "some-asset-name",
							"content_type": "some-content-type"
						},
						{
						  "path": "%s",
							"name": "other-asset-name",
							"content_type": "other-content-type"
						}
					]`, filepath.Join(tmpDir, "some-asset"), filepath.Join(tmpDir, "other-asset")),
				)

				buffer := gbytes.NewBuffer()

				session, err := gexec.Start(command, buffer, buffer)
				Expect(err).NotTo(HaveOccurred())

				Eventually(session).Should(gexec.Exit(0), func() string { return fmt.Sprintf("output -> \n%s\n", buffer.Contents()) })

				Expect(requests).To(HaveLen(3))

				Expect(requests[0].Method).To(Equal("POST"))
				Expect(requests[0].URL.Path).To(Equal("/repos/some-org/some-repo/releases"))

				content, err := io.ReadAll(requests[0].Body)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(content)).To(MatchJSON(`{
					"tag_name": "some-tag",
					"target_commitish": "some-commitish",
					"name": "some-name",
					"body": "some-body",
					"draft": true
				}`))

				Expect(requests[1].Method).To(Equal("POST"))
				Expect(requests[1].URL.Path).To(Equal("/repos/some-org/some-repo/releases/1/assets"))
				Expect(requests[1].URL.Query().Get("name")).To(Equal("some-asset-name"))

				Expect(requests[1].Header.Get("Content-Type")).To(Equal("some-content-type"))
				Expect(requests[1].ContentLength).To(Equal(int64(13)))

				content, err = io.ReadAll(requests[1].Body)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(content)).To(Equal("some-contents"))

				Expect(requests[2].Method).To(Equal("POST"))
				Expect(requests[2].URL.Path).To(Equal("/repos/some-org/some-repo/releases/1/assets"))
				Expect(requests[2].URL.Query().Get("name")).To(Equal("other-asset-name"))

				Expect(requests[2].Header.Get("Content-Type")).To(Equal("other-content-type"))
				Expect(requests[2].ContentLength).To(Equal(int64(14)))

				content, err = io.ReadAll(requests[2].Body)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(content)).To(Equal("other-contents"))

				Expect(buffer).To(gbytes.Say(`Creating release`))
				Expect(buffer).To(gbytes.Say(`  Repository: some-org/some-repo`))
				Expect(buffer).To(gbytes.Say(fmt.Sprintf(`  Uploading asset: %s -> some-asset-name`, filepath.Join(tmpDir, "some-asset"))))
				Expect(buffer).To(gbytes.Say(fmt.Sprintf(`  Uploading asset: %s -> other-asset-name`, filepath.Join(tmpDir, "other-asset"))))
				Expect(buffer).To(gbytes.Say(`Release is drafted, exiting.`))
			})
		})

		context("failure cases", func() {
			context("when the retry time limit is an invalid duration", func() {
				it("prints an error and exits non-zero", func() {
					command := exec.Command(
						entrypoint,
						"--retry-time-limit", "unparsable",
						"--repo", "some-org/some-repo",
						"--endpoint", api.URL,
						"--token", "some-github-token",
						"--tag-name", "some-tag",
						"--target-commitish", "some-commitish",
						"--name", "some-name",
						"--body", "some-body",
					)

					buffer := gbytes.NewBuffer()

					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())

					Eventually(session).Should(gexec.Exit(1), func() string { return fmt.Sprintf("output -> \n%s\n", buffer.Contents()) })

					Expect(buffer).To(gbytes.Say(`Error: time: invalid duration "unparsable"`))
				})

			})

			context("when missing the repo flag", func() {
				it("prints an error and exits non-zero", func() {
					command := exec.Command(
						entrypoint,
						"--endpoint", api.URL,
						"--token", "some-github-token",
						"--tag-name", "some-tag",
						"--target-commitish", "some-commitish",
						"--name", "some-name",
						"--body", "some-body",
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
						"--tag-name", "some-tag",
						"--target-commitish", "some-commitish",
						"--name", "some-name",
						"--body", "some-body",
					)

					buffer := gbytes.NewBuffer()

					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())

					Eventually(session).Should(gexec.Exit(1), func() string { return fmt.Sprintf("output -> \n%s\n", buffer.Contents()) })

					Expect(buffer).To(gbytes.Say(`Error: missing required input "token"`))
				})
			})

			context("when missing the tag-name flag", func() {
				it("prints an error and exits non-zero", func() {
					command := exec.Command(
						entrypoint,
						"--endpoint", api.URL,
						"--repo", "some-org/some-repo",
						"--token", "some-github-token",
						"--target-commitish", "some-commitish",
						"--name", "some-name",
						"--body", "some-body",
					)

					buffer := gbytes.NewBuffer()

					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())

					Eventually(session).Should(gexec.Exit(1), func() string { return fmt.Sprintf("output -> \n%s\n", buffer.Contents()) })

					Expect(buffer).To(gbytes.Say(`Error: missing required input "tag_name"`))
				})
			})

			context("when missing the target-commitish flag", func() {
				it("prints an error and exits non-zero", func() {
					command := exec.Command(
						entrypoint,
						"--endpoint", api.URL,
						"--repo", "some-org/some-repo",
						"--token", "some-github-token",
						"--tag-name", "some-tag",
						"--name", "some-name",
						"--body", "some-body",
					)

					buffer := gbytes.NewBuffer()

					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())

					Eventually(session).Should(gexec.Exit(1), func() string { return fmt.Sprintf("output -> \n%s\n", buffer.Contents()) })

					Expect(buffer).To(gbytes.Say(`Error: missing required input "target_commitish"`))
				})
			})

			context("when missing the name flag", func() {
				it("prints an error and exits non-zero", func() {
					command := exec.Command(
						entrypoint,
						"--endpoint", api.URL,
						"--repo", "some-org/some-repo",
						"--token", "some-github-token",
						"--tag-name", "some-tag",
						"--target-commitish", "some-commitish",
						"--body", "some-body",
					)

					buffer := gbytes.NewBuffer()

					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())

					Eventually(session).Should(gexec.Exit(1), func() string { return fmt.Sprintf("output -> \n%s\n", buffer.Contents()) })

					Expect(buffer).To(gbytes.Say(`Error: missing required input "name"`))
				})
			})

			context("when assets are malformed", func() {
				it("prints an error and exits non-zero", func() {
					command := exec.Command(
						entrypoint,
						"--endpoint", api.URL,
						"--repo", "some-org/some-repo",
						"--token", "some-github-token",
						"--tag-name", "some-tag",
						"--target-commitish", "some-commitish",
						"--name", "some-name",
						"--body", "some-body",
						"--assets", `%%%`,
					)

					buffer := gbytes.NewBuffer()

					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())

					Eventually(session).Should(gexec.Exit(1), func() string { return fmt.Sprintf("output -> \n%s\n", buffer.Contents()) })

					Expect(buffer).To(gbytes.Say(`Error: failed to parse assets: invalid character .*`))
				})
			})

			context("when endpoint is malformed", func() {
				it("prints an error and exits non-zero", func() {
					command := exec.Command(
						entrypoint,
						"--endpoint", "%%%",
						"--repo", "some-org/some-repo",
						"--token", "some-github-token",
						"--tag-name", "some-tag",
						"--target-commitish", "some-commitish",
						"--name", "some-name",
						"--body", "some-body",
					)

					buffer := gbytes.NewBuffer()

					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())

					Eventually(session).Should(gexec.Exit(1), func() string { return fmt.Sprintf("output -> \n%s\n", buffer.Contents()) })

					Expect(buffer).To(gbytes.Say(`Error: failed to create request: .* invalid URL escape .*`))
				})
			})

			context("when the endpoint is infinitely redirecting", func() {
				it("prints an error and exits non-zero", func() {
					command := exec.Command(
						entrypoint,
						"--endpoint", api.URL,
						"--repo", "some-org/some-redirecting-repo",
						"--token", "some-github-token",
						"--tag-name", "some-tag",
						"--target-commitish", "some-commitish",
						"--name", "some-name",
						"--body", "some-body",
					)

					buffer := gbytes.NewBuffer()

					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())

					Eventually(session).Should(gexec.Exit(1), func() string { return fmt.Sprintf("output -> \n%s\n", buffer.Contents()) })

					Expect(buffer).To(gbytes.Say(`Error: failed to complete request: .* stopped after 10 redirects`))
				})
			})

			context("when the create release request errors", func() {
				it("prints an error and exits non-zero", func() {
					command := exec.Command(
						entrypoint,
						"--endpoint", api.URL,
						"--repo", "some-org/some-create-error-repo",
						"--token", "some-github-token",
						"--tag-name", "some-tag",
						"--target-commitish", "some-commitish",
						"--name", "some-name",
						"--body", "some-body",
					)

					buffer := gbytes.NewBuffer()

					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())

					Eventually(session).Should(gexec.Exit(1), func() string { return fmt.Sprintf("output -> \n%s\n", buffer.Contents()) })

					Expect(buffer).To(gbytes.Say(`Error: failed to create release: unexpected response`))
				})
			})

			context("when the create release response is malformed", func() {
				it("prints an error and exits non-zero", func() {
					command := exec.Command(
						entrypoint,
						"--endpoint", api.URL,
						"--repo", "some-org/some-create-malformed-repo",
						"--token", "some-github-token",
						"--tag-name", "some-tag",
						"--target-commitish", "some-commitish",
						"--name", "some-name",
						"--body", "some-body",
					)

					buffer := gbytes.NewBuffer()

					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())

					Eventually(session).Should(gexec.Exit(1), func() string { return fmt.Sprintf("output -> \n%s\n", buffer.Contents()) })

					Expect(buffer).To(gbytes.Say(`Error: failed to parse create release response: invalid character`))
				})
			})

			context("when the upload url is malformed", func() {
				it("prints an error and exits non-zero", func() {
					command := exec.Command(
						entrypoint,
						"--endpoint", api.URL,
						"--repo", "some-org/some-malformed-upload-url-repo",
						"--token", "some-github-token",
						"--tag-name", "some-tag",
						"--target-commitish", "some-commitish",
						"--name", "some-name",
						"--body", "some-body",
						"--assets", `[
							{
								"path": "some-path",
								"name": "some-asset-name",
								"content_type": "some-content-type"
							}
						]`,
					)

					buffer := gbytes.NewBuffer()

					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())

					Eventually(session).Should(gexec.Exit(1), func() string { return fmt.Sprintf("output -> \n%s\n", buffer.Contents()) })

					Expect(buffer).To(gbytes.Say(`Error: failed to parse upload url: .* invalid URL escape .*`))
				})
			})

			context("when the asset cannot be opened", func() {
				it("prints an error and exits non-zero", func() {
					command := exec.Command(
						entrypoint,
						"--retry-time-limit", "1s",
						"--endpoint", api.URL,
						"--repo", "some-org/some-repo",
						"--token", "some-github-token",
						"--tag-name", "some-tag",
						"--target-commitish", "some-commitish",
						"--name", "some-name",
						"--body", "some-body",
						"--assets", `[
							{
								"path": "some-path",
								"name": "some-asset-name",
								"content_type": "some-content-type"
							}
						]`,
					)

					buffer := gbytes.NewBuffer()

					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())

					Eventually(session).Should(gexec.Exit(1), func() string { return fmt.Sprintf("output -> \n%s\n", buffer.Contents()) })

					Expect(buffer).To(gbytes.Say(`Error: failed to open file: .* no such file or directory`))
				})
			})

			context("when the asset cannot be uploaded", func() {
				var tmpDir string

				it.Before(func() {
					var err error
					tmpDir, err = os.MkdirTemp("", "assets")
					Expect(err).NotTo(HaveOccurred())

					err = os.WriteFile(filepath.Join(tmpDir, "some-asset"), []byte("some-contents"), 0644)
					Expect(err).NotTo(HaveOccurred())
				})

				it.After(func() {
					Expect(os.RemoveAll(tmpDir)).To(Succeed())
				})

				it("prints an error and exits non-zero", func() {
					command := exec.Command(
						entrypoint,
						"--retry-time-limit", "1s",
						"--endpoint", api.URL,
						"--repo", "some-org/some-upload-redirect-repo",
						"--token", "some-github-token",
						"--tag-name", "some-tag",
						"--target-commitish", "some-commitish",
						"--name", "some-name",
						"--body", "some-body",
						"--assets", fmt.Sprintf(`[
							{
								"path": "%s",
								"name": "some-asset-name",
								"content_type": "some-content-type"
							}
						]`, filepath.Join(tmpDir, "some-asset")),
					)

					buffer := gbytes.NewBuffer()

					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())

					Eventually(session).Should(gexec.Exit(1), func() string { return fmt.Sprintf("output -> \n%s\n", buffer.Contents()) })

					Expect(buffer).To(gbytes.Say(`Error: failed to complete request: .* stopped after 10 redirects`))
				})
			})

			context("when the asset upload errors", func() {
				var tmpDir string

				it.Before(func() {
					var err error
					tmpDir, err = os.MkdirTemp("", "assets")
					Expect(err).NotTo(HaveOccurred())

					err = os.WriteFile(filepath.Join(tmpDir, "some-asset"), []byte("some-contents"), 0644)
					Expect(err).NotTo(HaveOccurred())
				})

				it.After(func() {
					Expect(os.RemoveAll(tmpDir)).To(Succeed())
				})

				it("prints an error and exits non-zero", func() {
					command := exec.Command(
						entrypoint,
						"--retry-time-limit", "1s",
						"--endpoint", api.URL,
						"--repo", "some-org/some-upload-error-repo",
						"--token", "some-github-token",
						"--tag-name", "some-tag",
						"--target-commitish", "some-commitish",
						"--name", "some-name",
						"--body", "some-body",
						"--assets", fmt.Sprintf(`[
							{
								"path": "%s",
								"name": "some-asset-name",
								"content_type": "some-content-type"
							}
						]`, filepath.Join(tmpDir, "some-asset")),
					)

					buffer := gbytes.NewBuffer()

					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())

					Eventually(session).Should(gexec.Exit(1), func() string { return fmt.Sprintf("output -> \n%s\n", buffer.Contents()) })

					Expect(buffer).To(gbytes.Say(`Error: failed to upload asset: unexpected response`))
				})
			})

			context("when the edit release response is infinitely redirecting", func() {
				it("prints an error and exits non-zero", func() {
					command := exec.Command(
						entrypoint,
						"--endpoint", api.URL,
						"--repo", "some-org/some-edit-redirect-repo",
						"--token", "some-github-token",
						"--tag-name", "some-tag",
						"--target-commitish", "some-commitish",
						"--name", "some-name",
						"--body", "some-body",
					)

					buffer := gbytes.NewBuffer()

					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())

					Eventually(session).Should(gexec.Exit(1), func() string { return fmt.Sprintf("output -> \n%s\n", buffer.Contents()) })

					Expect(buffer).To(gbytes.Say(`Error: failed to complete request: .* stopped after 10 redirects`))
				})
			})

			context("when the edit release errors", func() {
				it("prints an error and exits non-zero", func() {
					command := exec.Command(
						entrypoint,
						"--endpoint", api.URL,
						"--repo", "some-org/some-edit-error-repo",
						"--token", "some-github-token",
						"--tag-name", "some-tag",
						"--target-commitish", "some-commitish",
						"--name", "some-name",
						"--body", "some-body",
					)

					buffer := gbytes.NewBuffer()

					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())

					Eventually(session).Should(gexec.Exit(1), func() string { return fmt.Sprintf("output -> \n%s\n", buffer.Contents()) })

					Expect(buffer).To(gbytes.Say(`Error: failed to edit release: unexpected response`))
				})
			})
		})
	})
}
