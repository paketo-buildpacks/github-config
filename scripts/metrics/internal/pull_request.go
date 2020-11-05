package internal

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"net/url"
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

func getPullRequestCommits(pullRequest PullRequest, serverURI string) []Commit {
	commitsURL, err := url.Parse(pullRequest.Links.Commits.CommitsURL)
	if err != nil {
		panic(err)
	}

	client := &http.Client{}
	uri, err := url.Parse(serverURI)
	if err != nil {
		panic(err)
	}

	uri.Path = commitsURL.Path

	request, _ := http.NewRequest("GET", uri.String(), nil)
	request.Header.Add("Authorization", fmt.Sprintf("token %s", os.Getenv("PAKETO_GITHUB_TOKEN")))

	response, err := client.Do(request)
	if err != nil {
		panic(err)
	}

	body, _ := ioutil.ReadAll(response.Body)
	pullRequestCommits := []Commit{}

	err = json.Unmarshal(body, &pullRequestCommits)
	if err != nil {
		panic(fmt.Sprintf("failed to unmarshal\n%s\nwith error: %s", string(body), err))
	}
	return pullRequestCommits
}

func GetLastCommit(commits []Commit) Commit {
	if len(commits) == 0 {
		panic("PR has no commits")
	}

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

func calculateMinutesToMerge(pullRequest PullRequest, serverURI string) float64 {
	if pullRequest.MergedAt == "" {
		panic("this pull request was never merged")
	}

	pullRequestCommits := getPullRequestCommits(pullRequest, serverURI)
	lastCommit := GetLastCommit(pullRequestCommits)

	lastCommitTime, err := time.Parse(dateLayout, lastCommit.CommitData.Committer.Date)
	if err != nil {
		panic(err)
	}
	mergedAtTime, _ := time.Parse(dateLayout, pullRequest.MergedAt)

	mergeTime := math.Round(mergedAtTime.Sub(lastCommitTime).Minutes())
	return mergeTime
}
