package main_test

import (
	"fmt"
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
	entrypoint, err := gexec.Build("github.com/paketo-buildpacks/github-config/actions/stack/get-usns/entrypoint")
	Expect(err).NotTo(HaveOccurred())

	spec.Run(t, "get-usns", func(t *testing.T, context spec.G, it spec.S) {
		var (
			Expect     = NewWithT(t).Expect
			Eventually = NewWithT(t).Eventually

			api            *httptest.Server
			outputFilepath string
		)

		it.Before(func() {
			tempDir := t.TempDir()
			outputFilepath = filepath.Join(tempDir, "output-file")

			api = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				// Handle pagination based on offset query param
				offset := req.URL.Query().Get("offset")

				var filename string
				switch offset {
				case "0":
					filename = "notices0-20.json"
				case "20":
					filename = "notices20-40.json"
				default:
					filename = "notices0-20.json"
				}

				data, err := os.ReadFile(filepath.Join("testdata", filename))
				if err != nil {
					w.WriteHeader(http.StatusNotFound)
					_, _ = w.Write([]byte{})
					return
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(data)
			}))
		})

		it.After(func() {
			api.Close()
		})

		context("previous usns are empty", func() {
			it("outputs the correct patched usns", func() {
				command := exec.Command(
					entrypoint,
					"--api-url", api.URL,
					"--packages", `["avahi", "simgear"]`,
					"--distro", "noble",
					"--output", outputFilepath,
				)

				buffer := gbytes.NewBuffer()

				session, err := gexec.Start(command, buffer, buffer)
				Expect(err).NotTo(HaveOccurred())

				Eventually(session).Should(gexec.Exit(0), func() string { return fmt.Sprintf("output -> \n%s\n", buffer.Contents()) })
				Expect(string(buffer.Contents())).To(ContainSubstring("New USN found:"))

				Expect(outputFilepath).To(BeARegularFile())
				contents, err := os.ReadFile(outputFilepath)
				Expect(err).NotTo(HaveOccurred())

				Expect(string(contents)).To(ContainSubstring(`"id":"USN-7967-1"`))
				Expect(string(contents)).To(ContainSubstring(`"avahi"`))
			})
		})

		context("previous usns are NOT empty", func() {
			it("excludes USNs that are already in previous usns", func() {
				command := exec.Command(
					entrypoint,
					"--api-url", api.URL,
					"--packages", `["avahi", "simgear"]`,
					"--distro", "noble",
					"--last-usns", `[{"id":"USN-7967-1","title":"Avahi vulnerabilities","url":"","affected_packages":[],"cves":[]}]`,
					"--output", outputFilepath,
				)

				buffer := gbytes.NewBuffer()

				session, err := gexec.Start(command, buffer, buffer)
				Expect(err).NotTo(HaveOccurred())

				Eventually(session).Should(gexec.Exit(0), func() string { return fmt.Sprintf("output -> \n%s\n", buffer.Contents()) })

				Expect(outputFilepath).To(BeARegularFile())
				contents, err := os.ReadFile(outputFilepath)
				Expect(err).NotTo(HaveOccurred())

				Expect(string(contents)).ToNot(ContainSubstring(`"id":"USN-7967-1"`))
				Expect(string(contents)).To(ContainSubstring(`"id":"USN-7965-1"`))
			})

			it("returns empty array when all matching USNs are already patched", func() {
				command := exec.Command(
					entrypoint,
					"--api-url", api.URL,
					"--packages", `["avahi"]`,
					"--distro", "noble",
					"--last-usns", `[{"id":"USN-7967-1","title":"Avahi vulnerabilities","url":"","affected_packages":[],"cves":[]}]`,
					"--output", outputFilepath,
				)

				buffer := gbytes.NewBuffer()

				session, err := gexec.Start(command, buffer, buffer)
				Expect(err).NotTo(HaveOccurred())

				Eventually(session).Should(gexec.Exit(0), func() string { return fmt.Sprintf("output -> \n%s\n", buffer.Contents()) })

				Expect(outputFilepath).To(BeARegularFile())
				contents, err := os.ReadFile(outputFilepath)
				Expect(err).NotTo(HaveOccurred())

				Expect(string(contents)).To(Equal("[]"))
			})

			it("outputs USNs matching the expected output file", func() {
				command := exec.Command(
					entrypoint,
					"--api-url", api.URL,
					"--packages-filepath", "testdata/amd64-package-list-jammy-tiny.json",
					"--distro", "jammy",
					"--last-usns-filepath", "testdata/amd64-jammy-tiny-last-patched-usns.json",
					"--output", outputFilepath,
				)

				buffer := gbytes.NewBuffer()

				session, err := gexec.Start(command, buffer, buffer)
				Expect(err).NotTo(HaveOccurred())

				Eventually(session).Should(gexec.Exit(0), func() string { return fmt.Sprintf("output -> \n%s\n", buffer.Contents()) })

				Expect(outputFilepath).To(BeARegularFile())
				contents, err := os.ReadFile(outputFilepath)
				Expect(err).NotTo(HaveOccurred())

				expectedOutput, err := os.ReadFile("testdata/patched-usns_jammy-output.json")
				Expect(err).NotTo(HaveOccurred())

				Expect(string(contents)).To(Equal(string(expectedOutput)))
			})
		})

		context("failure cases", func() {
			context("when the API returns a non-200 status", func() {
				it("prints an error and exits non-zero", func() {
					failingAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
						w.WriteHeader(http.StatusInternalServerError)
					}))
					defer failingAPI.Close()

					command := exec.Command(
						entrypoint,
						"--api-url", failingAPI.URL,
						"--packages", `["avahi"]`,
						"--distro", "noble",
						"--output", outputFilepath,
					)

					buffer := gbytes.NewBuffer()

					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())

					Eventually(session).Should(gexec.Exit(1), func() string { return fmt.Sprintf("output -> \n%s\n", buffer.Contents()) })
					Expect(string(buffer.Contents())).To(ContainSubstring("API request failed with status: 500"))
				})
			})

			context("when the distro flag is invalid", func() {
				it("prints an error and exits non-zero", func() {
					command := exec.Command(
						entrypoint,
						"--api-url", api.URL,
						"--packages", `["avahi"]`,
						"--distro", "invalid-distro",
						"--output", outputFilepath,
					)

					buffer := gbytes.NewBuffer()

					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())

					Eventually(session).Should(gexec.Exit(1), func() string { return fmt.Sprintf("output -> \n%s\n", buffer.Contents()) })
					Expect(string(buffer.Contents())).To(ContainSubstring("--distro flag has to be one of the following values"))
				})
			})
		})
	})
}
