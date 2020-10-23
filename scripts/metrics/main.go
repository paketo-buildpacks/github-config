package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/aclements/go-moremath/stats"
)

const dateLayout string = "2006-01-02T15:04:05Z"

type Repository struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

func (r *Repository) GetClosedPullRequests(org string) []PullRequest {

	client := &http.Client{}
	request, _ := http.NewRequest("GET", fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls?per_page=200&state=closed", org, r.Name), nil)
	request.Header.Add("Authorization", fmt.Sprintf("token %s", os.Getenv("PAKETO_GITHUB_TOKEN")))

	response, err := client.Do(request)
	if err != nil {
		panic(err)
	}

	body, _ := ioutil.ReadAll(response.Body)
	pullRequests := []PullRequest{}
	err = json.Unmarshal(body, &pullRequests)
	if err != nil {
		panic(err)
	}
	return pullRequests
}

type PullRequestUser struct {
	Login string `json:"login"`
}

type PullRequest struct {
	Title     string          `json:"title"`
	Number    int64           `json:"number"`
	MergedAt  string          `json:"merged_at,omitempty"`
	CreatedAt string          `json:"created_at"`
	User      PullRequestUser `json:"user"`
	Links     struct {
		Commits struct {
			CommitsURL string `json:"href"`
		} `json:"commits"`
	} `json:"_links"`
}

func (p *PullRequest) CalculateMinutesToMerge() (float64, error) {
	if p.MergedAt == "" {
		panic("this pull request was never merged")
	}
	client := &http.Client{}
	request, _ := http.NewRequest("GET", p.Links.Commits.CommitsURL, nil)
	request.Header.Add("Authorization", fmt.Sprintf("token %s", os.Getenv("PAKETO_GITHUB_TOKEN")))

	response, err := client.Do(request)
	if err != nil {
		panic(err)
	}

	body, _ := ioutil.ReadAll(response.Body)
	pullRequestCommits := []Commit{}
	err = json.Unmarshal(body, &pullRequestCommits)
	if err != nil {
		panic(err)
	}

	sort.Slice(pullRequestCommits, func(i, j int) bool {
		iTime, _ := time.Parse(dateLayout, pullRequestCommits[i].CommitData.Committer.Date)
		jTime, _ := time.Parse(dateLayout, pullRequestCommits[j].CommitData.Committer.Date)
		return iTime.After(jTime)
	})

	var lastCommit Commit
	for _, commit := range pullRequestCommits {
		if !strings.Contains(commit.CommitData.Message, "Merge branch 'main'") {
			lastCommit = commit
			break
		}
	}
	lastCommitTime, _ := time.Parse(dateLayout, lastCommit.CommitData.Committer.Date)
	mergedAtTime, _ := time.Parse(dateLayout, p.MergedAt)
	return math.Round(mergedAtTime.Sub(lastCommitTime).Minutes()), nil
}

type Commit struct {
	CommitData struct {
		Message   string `json:"message"`
		Committer struct {
			Name string `json:"name"`
			Date string `json:"date"`
		} `json:"committer"`
	} `json:"commit"`
}

func main() {
	var mergeTimes []float64
	orgs := []string{"paketo-buildpacks", "paketo-community"}
	client := &http.Client{}

	for _, org := range orgs {

		request, _ := http.NewRequest("GET", fmt.Sprintf("https://api.github.com/orgs/%s/repos?per_page=100", org), nil)
		request.Header.Add("Authorization", fmt.Sprintf("token %s", os.Getenv("PAKETO_GITHUB_TOKEN")))

		response, err := client.Do(request)
		if err != nil {
			panic(err)
		}
		body, _ := ioutil.ReadAll(response.Body)

		repos := []Repository{}
		err = json.Unmarshal(body, &repos)
		if err != nil {
			panic(err)
		}

		for _, repo := range repos {
			pullRequests := repo.GetClosedPullRequests(org)

			for _, pullRequest := range pullRequests {
				if pullRequest.MergedAt == "" {
					continue
				}
				mergedAtTime, _ := time.Parse(dateLayout, pullRequest.MergedAt)
				if mergedAtTime.Before(time.Now().Add(-time.Hour * 30 * 24)) {
					continue
				}
				if strings.Contains(pullRequest.User.Login, "bot") {
					continue
				}
				if strings.Contains(pullRequest.User.Login, "nebhale") {
					continue
				}
				if strings.Contains(pullRequest.Title, "rfc") {
					continue
				}
				mergeTime, _ := pullRequest.CalculateMinutesToMerge()
				mergeTimes = append(mergeTimes, mergeTime)
				fmt.Printf("Pull request by %s\nOrg: %s   Repo: %s   ID: %d\ntook %f minutes to merge.\n", pullRequest.User.Login, org, repo.Name, pullRequest.Number, mergeTime)
			}
		}
	}
	mergeTimesSample := stats.Sample{Xs: mergeTimes}
	fmt.Printf("Merge Times (in minutes) \n\nAverage: %f\nMedian %f\n95th Percentile: %f\n", mergeTimesSample.Mean(), mergeTimesSample.Quantile(0.5), mergeTimesSample.Quantile(0.95))

}
