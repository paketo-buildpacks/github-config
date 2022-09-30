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
	entrypoint, err := gexec.Build("github.com/paketo-buildpacks/github-config/actions/release/download-asset/entrypoint")
	Expect(err).NotTo(HaveOccurred())

	spec.Run(t, "download-asset", func(t *testing.T, context spec.G, it spec.S) {
		var (
			Expect     = NewWithT(t).Expect
			Eventually = NewWithT(t).Eventually

			api      *httptest.Server
			requests []*http.Request

			outputFilepath string
		)

		it.Before(func() {
			tempDir := t.TempDir()
			outputFilepath = filepath.Join(tempDir, "output-file")

			api = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				dump, _ := httputil.DumpRequest(req, true)
				receivedRequest, _ := http.ReadRequest(bufio.NewReader(bytes.NewBuffer(dump)))

				requests = append(requests, receivedRequest)

				if req.Header.Get("Authorization") != "token some-github-token" {
					w.WriteHeader(http.StatusForbidden)
					return
				}

				switch req.URL.Path {

				case "/some-valid-asset":
					w.WriteHeader(http.StatusOK)
					fmt.Fprintf(w, "some-asset-contents")

				case "/some-redirecting-url":
					w.Header().Set("Location", req.URL.Path)
					w.WriteHeader(http.StatusFound)

				case "/some-error":
					w.WriteHeader(http.StatusInternalServerError)
					fmt.Fprintln(w, `{"error": "message"}`)

				default:
					t.Fatalf("unknown request: %s", dump)
				}
			}))
		})

		it("downloads a github asset", func() {
			command := exec.Command(
				entrypoint,
				"--url", fmt.Sprintf("%s/some-valid-asset", api.URL),
				"--token", "some-github-token",
				"--output", outputFilepath,
			)

			buffer := gbytes.NewBuffer()

			session, err := gexec.Start(command, buffer, buffer)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session).Should(gexec.Exit(0), func() string { return fmt.Sprintf("output -> \n%s\n", buffer.Contents()) })

			Expect(requests).To(HaveLen(1))

			Expect(requests[0].Method).To(Equal("GET"))
			Expect(requests[0].URL.Path).To(Equal("/some-valid-asset"))
			Expect(requests[0].Header["Accept"]).To(Equal([]string{"application/octet-stream"}))

			Expect(outputFilepath).To(BeARegularFile())
			contents, err := os.ReadFile(outputFilepath)
			Expect(err).NotTo(HaveOccurred())
			Expect(contents).To(Equal([]byte("some-asset-contents")))

			Expect(buffer).To(gbytes.Say(`Downloading asset:`))
			Expect(buffer).To(gbytes.Say(fmt.Sprintf(`%s/some-valid-asset -> %s`, api.URL, outputFilepath)))
			Expect(buffer).To(gbytes.Say(`Download complete`))
		})

		context("failure cases", func() {
			context("when the retry time limit is an invalid duration", func() {
				it("prints an error and exits non-zero", func() {
					command := exec.Command(
						entrypoint,
						"--retry-time-limit", "unparsable",
						"--output", "some-output",
						"--url", "some-url",
						"--token", "some-github-token",
					)

					buffer := gbytes.NewBuffer()

					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())

					Eventually(session).Should(gexec.Exit(1), func() string { return fmt.Sprintf("output -> \n%s\n", buffer.Contents()) })

					Expect(buffer).To(gbytes.Say(`Error: time: invalid duration "unparsable"`))
				})

			})

			context("when missing the output flag", func() {
				it("prints an error and exits non-zero", func() {
					command := exec.Command(
						entrypoint,
						"--token", "some-github-token",
						"--url", "some-url",
					)

					buffer := gbytes.NewBuffer()

					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())

					Eventually(session).Should(gexec.Exit(1), func() string { return fmt.Sprintf("output -> \n%s\n", buffer.Contents()) })

					Expect(buffer).To(gbytes.Say(`Error: missing required input "output"`))
				})
			})

			context("when missing the url flag", func() {
				it("prints an error and exits non-zero", func() {
					command := exec.Command(
						entrypoint,
						"--token", "some-github-token",
						"--output", "some-output",
					)

					buffer := gbytes.NewBuffer()

					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())

					Eventually(session).Should(gexec.Exit(1), func() string { return fmt.Sprintf("output -> \n%s\n", buffer.Contents()) })

					Expect(buffer).To(gbytes.Say(`Error: missing required input "url"`))
				})
			})

			context("when missing the token flag", func() {
				it("prints an error and exits non-zero", func() {
					command := exec.Command(
						entrypoint,
						"--url", "some-url",
						"--output", "some-output",
					)

					buffer := gbytes.NewBuffer()

					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())

					Eventually(session).Should(gexec.Exit(1), func() string { return fmt.Sprintf("output -> \n%s\n", buffer.Contents()) })

					Expect(buffer).To(gbytes.Say(`Error: missing required input "token"`))
				})
			})

			context("when the url is infinitely redirecting", func() {
				it("prints an error and exits non-zero", func() {
					command := exec.Command(
						entrypoint,
						"--retry-time-limit", "1s",
						"--url", fmt.Sprintf("%s/some-redirecting-url", api.URL),
						"--token", "some-github-token",
						"--output", "some-output",
					)

					buffer := gbytes.NewBuffer()

					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())

					Eventually(session).Should(gexec.Exit(1), func() string { return fmt.Sprintf("output -> \n%s\n", buffer.Contents()) })

					Expect(buffer).To(gbytes.Say(`Error: failed to complete request: .* stopped after 10 redirects`))
				})
			})

			context("when the URL is malformed", func() {
				it("prints an error and exits non-zero", func() {
					command := exec.Command(
						entrypoint,
						"--retry-time-limit", "1s",
						"--url", "%%%",
						"--token", "some-github-token",
						"--output", "some-output",
					)

					buffer := gbytes.NewBuffer()

					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())

					Eventually(session).Should(gexec.Exit(1), func() string { return fmt.Sprintf("output -> \n%s\n", buffer.Contents()) })

					Expect(buffer).To(gbytes.Say(`Error: failed to create request: parse "%%%": invalid URL escape "%%%"`))
				})
			})

			context("when the download request errors", func() {
				it("prints an error and exits non-zero", func() {
					command := exec.Command(
						entrypoint,
						"--retry-time-limit", "1s",
						"--url", fmt.Sprintf("%s/some-error", api.URL),
						"--token", "some-github-token",
						"--output", "some-output",
					)

					buffer := gbytes.NewBuffer()

					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())

					Eventually(session).Should(gexec.Exit(1), func() string { return fmt.Sprintf("output -> \n%s\n", buffer.Contents()) })

					Expect(buffer).To(gbytes.Say(`Error: failed to download asset: unexpected status`))
				})
			})

			context("when creating the output file fails", func() {
				it("prints an error and exits non-zero", func() {
					command := exec.Command(
						entrypoint,
						"--retry-time-limit", "1s",
						"--url", fmt.Sprintf("%s/some-valid-asset", api.URL),
						"--token", "some-github-token",
						"--output", "/invalid/path/to/file",
					)

					buffer := gbytes.NewBuffer()

					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())

					Eventually(session).Should(gexec.Exit(1), func() string { return fmt.Sprintf("output -> \n%s\n", buffer.Contents()) })

					Expect(buffer).To(gbytes.Say(`Error: failed to create output file: open /invalid/path/to/file: no such file or directory`))
				})
			})
		})
	})
}
