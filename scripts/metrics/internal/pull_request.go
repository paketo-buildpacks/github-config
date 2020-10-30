package internal

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
)

const dateLayout string = "2006-01-02T15:04:05Z"

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

func getPullRequestCommits(pullRequest PullRequest) []Commit {

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
		panic(string(body))
	}
	return pullRequestCommits
}

func GetLastCommit(commits []Commit) Commit {
	sort.Slice(commits, func(i, j int) bool {
		iTime, _ := time.Parse(dateLayout, commits[i].CommitData.Committer.Date)
		jTime, _ := time.Parse(dateLayout, commits[j].CommitData.Committer.Date)
		return iTime.After(jTime)
	})

	for _, commit := range commits {
		if !strings.Contains(commit.CommitData.Message, "Merge branch 'main'") {
			return commit
		}
	}
	panic("no last commit")
}

func calculateMinutesToMerge(pullRequest PullRequest) float64 {
	if pullRequest.MergedAt == "" {
		panic("this pull request was never merged")
	}

	pullRequestCommits := getPullRequestCommits(pullRequest)
	lastCommit := GetLastCommit(pullRequestCommits)

	lastCommitTime, _ := time.Parse(dateLayout, lastCommit.CommitData.Committer.Date)
	mergedAtTime, _ := time.Parse(dateLayout, pullRequest.MergedAt)

	mergeTime := math.Round(mergedAtTime.Sub(lastCommitTime).Minutes())
	return mergeTime
}
