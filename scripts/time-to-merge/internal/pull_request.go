package internal

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"
)

const dateLayout string = "2006-01-02T15:04:05Z"

type MergeTimeContainer struct {
	MergeTime float64
	Error     error
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

func getPullRequestCommits(pullRequest PullRequest, serverURI string) ([]Commit, error) {
	commitsURL, err := url.Parse(pullRequest.Links.Commits.CommitsURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse commits URL: %s", err)
	}

	client := &http.Client{}
	uri, err := url.Parse(serverURI)
	if err != nil {
		return nil, fmt.Errorf("failed to parse server URL: %s", err)
	}

	uri.Path = commitsURL.Path

	request, err := http.NewRequest("GET", uri.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create http GET request for commits: %s", err)
	}
	request.Header.Add("Authorization", fmt.Sprintf("token %s", os.Getenv("PAKETO_GITHUB_TOKEN")))

	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("failed to make http GET request for commits: %s", err)
	}

	body, _ := io.ReadAll(response.Body)
	pullRequestCommits := []Commit{}

	err = json.Unmarshal(body, &pullRequestCommits)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal %s\n from API endpoint %s: %s", string(body), uri.String(), err)
	}
	return pullRequestCommits, nil
}

func GetLastCommit(commits []Commit) (Commit, error) {
	if len(commits) == 0 {
		return Commit{}, fmt.Errorf("PR has no commits")
	}

	sort.Slice(commits, func(i, j int) bool {
		iTime, _ := time.Parse(time.RFC3339, commits[i].CommitData.Committer.Date)
		jTime, _ := time.Parse(time.RFC3339, commits[j].CommitData.Committer.Date)
		return iTime.After(jTime)
	})

	for _, commit := range commits {
		if !strings.Contains(commit.CommitData.Message, "Merge branch 'main'") {
			return commit, nil
		}
	}
	return Commit{}, fmt.Errorf("PR has no last commit")
}

func calculateMinutesToMerge(pullRequest PullRequest, serverURI string) (float64, error) {
	if pullRequest.MergedAt == "" {
		return -1, fmt.Errorf("this pull request was never merged")
	}

	pullRequestCommits, err := getPullRequestCommits(pullRequest, serverURI)
	if err != nil {
		return -1, fmt.Errorf("could not get commits from closed pull request: %s", err)
	}
	lastCommit, err := GetLastCommit(pullRequestCommits)
	if err != nil {
		return -1, fmt.Errorf("failed to get last commit from PR: %s", err)
	}

	lastCommitTime, err := time.Parse(time.RFC3339, lastCommit.CommitData.Committer.Date)
	if err != nil {
		return -1, fmt.Errorf("could not parse PR last commit time %s: %s", lastCommitTime, err)
	}

	mergedAtTime, err := time.Parse(time.RFC3339, pullRequest.MergedAt)
	if err != nil {
		return -1, fmt.Errorf("could not parse PR merge time %s: %s", mergedAtTime, err)
	}

	mergeTime := math.Round(mergedAtTime.Sub(lastCommitTime).Minutes())
	return mergeTime, nil
}
