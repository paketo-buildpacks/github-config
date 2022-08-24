package main_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/paketo-buildpacks/occam"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

func TestEntrypoint(t *testing.T) {
	var Expect = NewWithT(t).Expect

	SetDefaultEventuallyTimeout(5 * time.Second)

	entrypoint, err := gexec.Build("main.go")
	Expect(err).NotTo(HaveOccurred())

	spec.Run(t, "actions/dependency/update-json", func(t *testing.T, context spec.G, it spec.S) {
		var (
			Expect     = NewWithT(t).Expect
			Eventually = NewWithT(t).Eventually

			source string
		)

		it.Before(func() {
			source, err = occam.Source("testdata")
			Expect(err).NotTo(HaveOccurred())
		})

		it.After(func() {
			Expect(os.RemoveAll(source)).To(Succeed())
		})

		context("given metadata in JSON form with matching version and target", func() {
			it("updates the SHA256 and URI fields for the appropriate JSON entry", func() {
				command := exec.Command(
					entrypoint,
					"--version", "1.2.3",
					"--target", "target-1",
					"--sha256", "target-1.2.3-sha256",
					"--uri", "target-1.2.3-uri",
					"--file", filepath.Join(source, "metadata.json"),
				)

				buffer := gbytes.NewBuffer()
				session, err := gexec.Start(command, buffer, buffer)
				Expect(err).NotTo(HaveOccurred())

				Eventually(session).Should(gexec.Exit(0), func() string { return string(buffer.Contents()) })

				Expect(buffer).To(gbytes.Say("Success! Updated metadata with:"))
				Expect(buffer).To(gbytes.Say(`"sha256": "target-1.2.3-sha256"`))
				Expect(buffer).To(gbytes.Say(`"uri": "target-1.2.3-uri"`))

				actualContents, err := os.ReadFile(filepath.Join(source, "metadata.json"))
				Expect(err).NotTo(HaveOccurred())

				expectedContents, err := os.ReadFile(filepath.Join(source, "expected-metadata.json"))
				Expect(err).NotTo(HaveOccurred())

				Expect(actualContents).To(MatchJSON(expectedContents))

			})
		})

		context("given metadata in JSON form with NO matching version and target", func() {
			it("the metadata.json is unchanged", func() {
				// Get contents before file changes
				expectedContents, err := os.ReadFile(filepath.Join(source, "metadata.json"))
				Expect(err).NotTo(HaveOccurred())

				command := exec.Command(
					entrypoint,
					"--version", "3.4.5",
					"--target", "diff-target",
					"--sha256", "target-1.2.3-sha256",
					"--uri", "target-1.2.3-uri",
					"--file", filepath.Join(source, "metadata.json"),
				)

				buffer := gbytes.NewBuffer()
				session, err := gexec.Start(command, buffer, buffer)
				Expect(err).NotTo(HaveOccurred())

				Eventually(session).Should(gexec.Exit(0), func() string { return string(buffer.Contents()) })
				Expect(buffer).To(gbytes.Say("No change, no matching metadata found. Exiting."))

				actualContents, err := os.ReadFile(filepath.Join(source, "metadata.json"))
				Expect(err).NotTo(HaveOccurred())

				Expect(actualContents).To(MatchJSON(expectedContents))
			})
		})

		context("failure cases", func() {
			context("when the --version flag is missing", func() {
				it("returns an error and exits non-zero", func() {
					command := exec.Command(
						entrypoint,
						"--target", "target-1",
						"--sha256", "target-1.2.3-sha256",
						"--uri", "target-1.2.3-uri",
						"--file", filepath.Join(source, "metadata.json"),
					)

					buffer := gbytes.NewBuffer()
					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())

					Eventually(session).Should(gexec.Exit(1), func() string { return string(buffer.Contents()) })
					Expect(buffer).To(gbytes.Say(`missing required input "version"`))
				})
			})

			context("when the --target flag is missing", func() {
				it("returns an error and exits non-zero", func() {
					command := exec.Command(
						entrypoint,
						"--version", "1.2.3",
						"--sha256", "target-1.2.3-sha256",
						"--uri", "target-1.2.3-uri",
						"--file", filepath.Join(source, "metadata.json"),
					)

					buffer := gbytes.NewBuffer()
					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())

					Eventually(session).Should(gexec.Exit(1), func() string { return string(buffer.Contents()) })
					Expect(buffer).To(gbytes.Say(`missing required input "target"`))
				})
			})

			context("when the --sha256 flag is missing", func() {
				it("returns an error and exits non-zero", func() {
					command := exec.Command(
						entrypoint,
						"--version", "1.2.3",
						"--target", "target-1",
						"--uri", "target-1.2.3-uri",
						"--file", filepath.Join(source, "metadata.json"),
					)

					buffer := gbytes.NewBuffer()
					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())

					Eventually(session).Should(gexec.Exit(1), func() string { return string(buffer.Contents()) })
					Expect(buffer).To(gbytes.Say(`missing required input "SHA256"`))
				})
			})

			context("when the --uri flag is missing", func() {
				it("returns an error and exits non-zero", func() {
					command := exec.Command(
						entrypoint,
						"--version", "1.2.3",
						"--target", "target-1",
						"--sha256", "target-1.2.3-sha256",
						"--file", filepath.Join(source, "metadata.json"),
					)

					buffer := gbytes.NewBuffer()
					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())

					Eventually(session).Should(gexec.Exit(1), func() string { return string(buffer.Contents()) })
					Expect(buffer).To(gbytes.Say(`missing required input "uri"`))
				})
			})

			context("when the --file flag is missing", func() {
				it("returns an error and exits non-zero", func() {
					command := exec.Command(
						entrypoint,
						"--version", "1.2.3",
						"--target", "target-1",
						"--sha256", "target-1.2.3-sha256",
						"--uri", "target-1.2.3-uri",
					)

					buffer := gbytes.NewBuffer()
					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())

					Eventually(session).Should(gexec.Exit(1), func() string { return string(buffer.Contents()) })
					Expect(buffer).To(gbytes.Say(`missing required input "file"`))
				})
			})

			context("when the metadata file cannot be opened", func() {
				it.Before(func() {
					Expect(os.Chmod(filepath.Join(source, "metadata.json"), 0000)).To(Succeed())
				})

				it("returns an error and exits non-zero", func() {
					command := exec.Command(
						entrypoint,
						"--version", "1.2.3",
						"--target", "target-1",
						"--sha256", "target-1.2.3-sha256",
						"--uri", "target-1.2.3-uri",
						"--file", filepath.Join(source, "metadata.json"),
					)

					buffer := gbytes.NewBuffer()
					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())

					Eventually(session).Should(gexec.Exit(1), func() string { return string(buffer.Contents()) })
					Expect(buffer).To(gbytes.Say(`permission denied`))
				})
			})
		})
	})
}
