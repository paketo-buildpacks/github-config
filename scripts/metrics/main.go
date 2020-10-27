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

var orgs = []string{"paketo-buildpacks", "paketo-community"}

type Repository struct {
	Name string `json:"name"`
	URL  string `json:"url"`
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

	for _, org := range orgs {
		repos := getOrgRepos(org)

		for _, repo := range repos {
			mergeTimes = append(mergeTimes, getRepoMergeTimes(org, repo)...)
		}
	}

	mergeTimesSample := stats.Sample{Xs: mergeTimes}
	fmt.Printf("\nMerge Time Stats\n    Average: %f hours\n    Median %f hours\n    95th Percentile: %f hours\n", (mergeTimesSample.Mean() / 60), (mergeTimesSample.Quantile(0.5) / 60), (mergeTimesSample.Quantile(0.95) / 60))
}

func getOrgRepos(org string) []Repository {
	client := &http.Client{}
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
	return repos
}

func getRepoMergeTimes(org string, repo Repository) []float64 {
	mergeTimes := []float64{}
	pullRequests := getClosedPullRequests(org, repo)
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
		mergeTime := calculateMinutesToMerge(pullRequest)
		fmt.Printf("Pull request %s/%s #%d by %s\ntook %f minutes to merge.\n", org, repo.Name, pullRequest.Number, pullRequest.User.Login, mergeTime)
		mergeTimes = append(mergeTimes, mergeTime)
	}
	return mergeTimes
}

func getClosedPullRequests(org string, repo Repository) []PullRequest {
	client := &http.Client{}
	request, _ := http.NewRequest("GET", fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls?per_page=200&state=closed", org, repo.Name), nil)
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

func calculateMinutesToMerge(pullRequest PullRequest) float64 {
	if pullRequest.MergedAt == "" {
		panic("this pull request was never merged")
	}
	client := &http.Client{}
	request, _ := http.NewRequest("GET", pullRequest.Links.Commits.CommitsURL, nil)
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
	mergedAtTime, _ := time.Parse(dateLayout, pullRequest.MergedAt)

	mergeTime := math.Round(mergedAtTime.Sub(lastCommitTime).Minutes())
	return mergeTime
}
