package internal_test

import (
	"testing"

	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
	. "github.com/paketo-buildpacks/github-config/scripts/metrics/internal"
)

func testPullRequest(t *testing.T, context spec.G, it spec.S) {
	var Expect = NewWithT(t).Expect

	context("getLastCommit", func() {
		var commits []Commit

		context("the most recent commit is a merge commit", func() {
			it.Before(func() {
				firstCommit := Commit{}
				firstCommit.CommitData.Message = "first-commit-message"
				firstCommit.CommitData.Committer.Date = "2000-10-31T00:00:00Z"

				secondCommit := Commit{}
				secondCommit.CommitData.Message = "second-commit-message"
				secondCommit.CommitData.Committer.Date = "2000-10-31T01:00:00Z"

				mergeCommit := Commit{}
				mergeCommit.CommitData.Message = "Merge branch 'main' into branch 'other-branch'"
				mergeCommit.CommitData.Committer.Date = "2000-10-31T02:00:00Z"
				commits = []Commit{firstCommit, secondCommit, mergeCommit}

			})

			it("returns the latest non-merge commit", func() {
				lastCommit := GetLastCommit(commits)
				Expect(lastCommit.CommitData.Message).To(ContainSubstring("second-commit-message"))
				Expect(lastCommit.CommitData.Committer.Date).To(Equal("2000-10-31T01:00:00Z"))
			})

		})
		context("the most recent commit is NOT a merge commit", func() {
			it.Before(func() {
				firstCommit := Commit{}
				firstCommit.CommitData.Message = "first-commit-message"
				firstCommit.CommitData.Committer.Date = "2000-10-31T00:00:00Z"

				secondCommit := Commit{}
				secondCommit.CommitData.Message = "second-commit-message"
				secondCommit.CommitData.Committer.Date = "2000-10-31T01:00:00Z"

				thirdCommit := Commit{}
				thirdCommit.CommitData.Message = "third-commit-message"
				thirdCommit.CommitData.Committer.Date = "2000-10-31T02:00:00Z"

				commits = []Commit{firstCommit, thirdCommit, secondCommit}
			})

			it("returns the latest non-merge commit", func() {
				lastCommit := GetLastCommit(commits)
				Expect(lastCommit.CommitData.Message).To(ContainSubstring("third-commit-message"))
				Expect(lastCommit.CommitData.Committer.Date).To(Equal("2000-10-31T02:00:00Z"))
			})
		})
	})
}
