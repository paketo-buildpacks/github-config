package internal_test

import (
	"testing"

	"github.com/paketo-buildpacks/github-config/actions/builder/update/entrypoint/internal"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

func testImage(t *testing.T, context spec.G, it spec.S) {
	var Expect = NewWithT(t).Expect

	context("imageFromFullName", func() {
		context("remote image with tag", func() {
			it("image is parsed correctly", func() {
				img, err := internal.NewImageReference("gcr.io/somerepo/someimage:sometag")
				Expect(err).NotTo(HaveOccurred())
				Expect(img.Domain).To(Equal("gcr.io"))
				Expect(img.Path).To(Equal("somerepo/someimage"))
				Expect(img.Tag).To(Equal("sometag"))
			})
		})

		context("remote image with no tag", func() {
			it("image is parsed correctly", func() {
				img, err := internal.NewImageReference("gcr.io/somerepo/someimage")
				Expect(err).NotTo(HaveOccurred())
				Expect(img.Domain).To(Equal("gcr.io"))
				Expect(img.Path).To(Equal("somerepo/someimage"))
				Expect(img.Tag).To(Equal("latest"))
			})
		})

		context("localhost image with tag", func() {
			it("image is parsed correctly", func() {
				img, err := internal.NewImageReference("localhost:8788/somerepo/someimage:sometag")
				Expect(err).NotTo(HaveOccurred())
				Expect(img.Domain).To(Equal("localhost:8788"))
				Expect(img.Path).To(Equal("somerepo/someimage"))
				Expect(img.Tag).To(Equal("sometag"))
			})
		})

		context("localhost image with no tag", func() {
			it("image is parsed correctly", func() {
				img, err := internal.NewImageReference("localhost:8788/somerepo/someimage")
				Expect(err).NotTo(HaveOccurred())
				Expect(img.Domain).To(Equal("localhost:8788"))
				Expect(img.Path).To(Equal("somerepo/someimage"))
				Expect(img.Tag).To(Equal("latest"))
			})
		})

		context("invalid image", func() {
			it("errors with appropriate message", func() {
				_, err := internal.NewImageReference("%%%")
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(ContainSubstring(`failed to parse reference "%%%": invalid reference format`)))
			})
		})
	})
}
