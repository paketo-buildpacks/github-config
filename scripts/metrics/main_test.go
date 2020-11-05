package main_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"

	. "github.com/paketo-buildpacks/occam/matchers"
	"github.com/sclevine/spec"
)

const paketoBuildpacksRepoResponse string = `[{
"name" : "example-repo",
"url" : "example-URL",
"owner" : {
		"login" : "paketo-buildpacks"
	}
}]`

const paketoCommunityRepoResponse string = `[{
"name" : "other-example-repo",
"url" : "example-URL",
"owner" : {
		"login" : "paketo-community"
	}
}]`

const paketoBuildpacksClosedPRsResponse string = `[{
"title" : "pr-1",
"number" : 1,
"merged_at" : "2020-10-31T01:00:00Z",
"created_at" : "some-garbage",
"user" : {
	"login" : "example-contributor"
	},
"_links" : {
	"commits" : {
			"href" : "https://api.server.com/repos/paketo-buildpacks/example-repo/pulls/1/commits"
		}
	}
}]`

const paketoCommunityClosedPRsResponse string = `[{
"title" : "pr-2",
"number" : 2,
"merged_at" : "2020-10-31T01:00:00Z",
"created_at" : "some-garbage",
"user" : {
	"login" : "other-example-contributor"
	},
"_links" : {
	"commits" : {
			"href" : "https://api.server.com/repos/paketo-community/other-example-repo/pulls/2/commits"
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
  "commit": {
    "committer": {
      "name": "other-example-committer",
      "email": "noreply@github.com",
			"date": "2020-10-31T00:45:00Z"
    },
    "message": "other-example-commit-message"
  }
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
					fmt.Fprintln(w, paketoBuildpacksRepoResponse)

				case "/orgs/paketo-community/repos":
					w.WriteHeader(http.StatusOK)
					fmt.Fprintln(w, paketoCommunityRepoResponse)

				case "/repos/paketo-buildpacks/example-repo/pulls":
					w.WriteHeader(http.StatusOK)
					fmt.Fprintln(w, paketoBuildpacksClosedPRsResponse)

				case "/repos/paketo-community/other-example-repo/pulls":
					w.WriteHeader(http.StatusOK)
					fmt.Fprintln(w, paketoCommunityClosedPRsResponse)

				case "/repos/paketo-buildpacks/example-repo/pulls/1/commits":
					w.WriteHeader(http.StatusOK)
					fmt.Fprintln(w, closedPR1CommitsResponse)

				case "/repos/paketo-community/other-example-repo/pulls/2/commits":
					w.WriteHeader(http.StatusOK)
					fmt.Fprintln(w, closedPR2CommitsResponse)
				default:
					t.Fatal(fmt.Sprintf("unknown path: %s", req.URL.Path))
				}
			}))

		})

		it.After(func() {
			mockGithubServer.Close()
		})

		context("given a valid auth token is provided", func() {
			it.Before(func() {
				os.Setenv("PAKETO_GITHUB_TOKEN", "some-token")
			})
			it("correctly calculates median merge time of closed PRs from the past 30 days", func() {
				command := exec.Command(mergeTimeCalculator, "--server", mockGithubServer.URL)
				fmt.Println(mockGithubServerURI)
				buffer := gbytes.NewBuffer()
				session, err := gexec.Start(command, buffer, buffer)

				Expect(err).NotTo(HaveOccurred())
				Eventually(session).Should(gexec.Exit(0), func() string { return string(buffer.Contents()) })

				out := string(buffer.Contents())

				Expect(out).To(ContainLines(
					`Pull request paketo-buildpacks/example-repo #1 by example-contributor`,
					`took 60.000000 minutes to merge.`,
				))

				Expect(out).To(ContainLines(
					`Pull request paketo-community/other-example-repo #2 by other-example-contributor`,
					`took 15.000000 minutes to merge.`,
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
