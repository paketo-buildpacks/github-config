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
	"path"
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

				requestedFile := path.Base(req.URL.Path)

				data, err := os.ReadFile(filepath.Join("testdata", requestedFile))
				if err != nil {
					t.Fatalf("failed to read fixture: %v", err)
				}
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(data)

			}))
		})

		context("previous usns are empty", func() {
			it.Before(func() {
				os.Setenv("GITHUB_USN_BASE_URL", api.URL)
			})

			it("outputs the correct patched usns", func() {
				command := exec.Command(
					entrypoint,
					"--packages", "[\"squid\", \"fetchmail\"]",
					"--distro", "noble",
					"--feed-url", fmt.Sprintf("%s/rss_feed_1.xml", api.URL),
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

				Expect(contents).To(ContainSubstring(`"affected_packages":["squid"]`))
				Expect(contents).To(ContainSubstring(`"affected_packages":["fetchmail"]`))
			})

		})

		context("previous usns are NOT empty", func() {
			it.Before(func() {
				os.Setenv("GITHUB_USN_BASE_URL", api.URL)
			})

			it("and the fetchmail package is on previous usns", func() {
				command := exec.Command(
					entrypoint,
					"--packages", "[\"squid\", \"fetchmail\"]",
					"--distro", "noble",
					"--feed-url", fmt.Sprintf("%s/rss_feed_1.xml", api.URL),
					"--last-usns-filepath", "testdata/previous_patched_usns_fetchmail.json",
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

				Expect(contents).To(ContainSubstring(`"affected_packages":["squid"]`))
				Expect(contents).ToNot(ContainSubstring(`"affected_packages":["fetchmail"]`))
			})

			it("fetchmail and squid package is on previous usns", func() {
				command := exec.Command(
					entrypoint,
					"--packages", "[\"squid\", \"fetchmail\"]",
					"--distro", "noble",
					"--feed-url", fmt.Sprintf("%s/rss_feed_1.xml", api.URL),
					"--last-usns-filepath", "testdata/previous_patched_usns_fetchmail_squid.json",
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

				Expect(contents).ToNot(ContainSubstring(`"affected_packages":["squid"]`))
				Expect(contents).ToNot(ContainSubstring(`"affected_packages":["fetchmail"]`))
				Expect(string(contents)).To(Equal("[]"))
			})

			it("fetchmail and squid package is on previous usns with different order", func() {
				command := exec.Command(
					entrypoint,
					"--packages", "[\"squid\", \"fetchmail\"]",
					"--distro", "noble",
					"--feed-url", fmt.Sprintf("%s/rss_feed_1.xml", api.URL),
					"--last-usns-filepath", "testdata/previous_patched_usns_squid_fetchmail.json",
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

				Expect(contents).ToNot(ContainSubstring(`"affected_packages":["squid"]`))
				Expect(contents).ToNot(ContainSubstring(`"affected_packages":["fetchmail"]`))
				Expect(string(contents)).To(Equal("[]"))
			})

		})
	})
}
