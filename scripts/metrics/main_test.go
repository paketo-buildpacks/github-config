package main_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"

	. "github.com/paketo-buildpacks/occam/matchers"
	"github.com/sclevine/spec"
)

const orgRepoResponse string = `[{
"name" : "example-repo",
"url" : "example-URL",
"owner" : {
		"login" : "paketo-buildpacks"
	}
}]`

const repoClosedPRsResponse string = `[{
"title" : "pr-1",
"number" : 1,
"merged_at" : "2020-10-31T01:00:00Z",
"created_at" : "some-garbage",
"user" : {
	"login" : "paketo-buildpacks"
	},
"_links" : {
	"commits" : {
			"href" : "https://api.github.com/repos/paketo-buildpacks/example-repo/pulls/42/commits"
		}
	}
}]`

const closedPR1CommitsResponse string = `[{
  "commit": {
    "committer": {
      "name": "example-committer",
      "email": "noreply@github.com",
      "date": "2020-10-31T00:00:00Z"
    },
    "message": "example-commit-message"
  }
}]`

const closedPR2CommitsResponse string = `[{
}]`

func TestMergeTimeCalculator(t *testing.T) {
	var Expect = NewWithT(t).Expect

	mergeTimeCalculator, err := gexec.Build("github.com/paketo-buildpacks/github-config/scripts/metrics")
	Expect(err).NotTo(HaveOccurred())

	spec.Run(t, "scripts/metrics/metrics", func(t *testing.T, context spec.G, it spec.S) {
		var (
			Expect     = NewWithT(t).Expect
			Eventually = NewWithT(t).Eventually

			mockGithubServer    *httptest.Server
			mockGithubServerURI string
		)

		it.Before(func() {
			mockGithubServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				if req.Method == http.MethodHead {
					http.Error(w, "NotFound", http.StatusNotFound)
					return
				}

				switch req.URL.Path {
				case "/orgs/paketo-buildpacks/repos":
					w.WriteHeader(http.StatusOK)
					fmt.Fprintln(w, orgRepoResponse)

				case "/orgs/paketo-community/repos":
					w.WriteHeader(http.StatusOK)
					fmt.Fprintln(w, orgRepoResponse)

				case "/repos/paketo-buildpacks/example-repo/pulls":
					w.WriteHeader(http.StatusOK)
					fmt.Fprintln(w, repoClosedPRsResponse)

				case "/repos/paketo-community/example-repo/pulls":
					w.WriteHeader(http.StatusOK)
					fmt.Fprintln(w, repoClosedPRsResponse)

				case "/repos/paketo-community/example-repo/pulls/42/commits":
					w.WriteHeader(http.StatusOK)
					fmt.Fprintln(w, closedPR1CommitsResponse)

				case "/repos/paketo-buildpacks/example-repo/pulls/42/commits":
					w.WriteHeader(http.StatusOK)
					fmt.Fprintln(w, closedPR1CommitsResponse)
				default:
					t.Fatal(fmt.Sprintf("unknown path: %s", req.URL.Path))
				}
			}))

			uri, err := url.Parse(mockGithubServer.URL)
			Expect(err).NotTo(HaveOccurred())
			mockGithubServerURI = uri.Host
		})

		it.After(func() {
			mockGithubServer.Close()
		})

		context.Focus("given a valid auth token is provided", func() {
			it.Before(func() {
				os.Setenv("PAKETO_GITHUB_TOKEN", "some-token")
			})
			it("correctly calculates median merge time of closed PRs from the past 30 days", func() {
				command := exec.Command(mergeTimeCalculator, "--server", mockGithubServerURI)
				buffer := gbytes.NewBuffer()
				session, err := gexec.Start(command, buffer, buffer)

				Expect(err).NotTo(HaveOccurred())
				Eventually(session).Should(gexec.Exit(0), func() string { return string(buffer.Contents()) })

				out := string(buffer.Contents())

				Expect(out).To(ContainLines(
					`"Pull request paketo-buildpacks/example-repo #42 by example-committer`,
					`took 60 minutes to merge.`,
				))
			})
		})

		context("given no auth token has been provided", func() {
			it.Before(func() {
				os.Setenv("PAKETO_GITHUB_TOKEN", "")
			})

			it("exits and says that an auth token is needed", func() {

				command := exec.Command(mergeTimeCalculator)
				fmt.Println(command)
				buffer := gbytes.NewBuffer()
				session, err := gexec.Start(command, buffer, buffer)

				Expect(err).NotTo(HaveOccurred())
				Eventually(session).Should(gexec.Exit(0), func() string { return string(buffer.Contents()) })

				out := string(buffer.Contents())

				Expect(out).To(ContainLines(
					`Please set PAKETO_GITHUB_TOKEN`,
					`Exiting.`,
				))
			})
		})

	})
}
