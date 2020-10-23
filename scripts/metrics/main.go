package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
)

type Repository struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

func (r *Repository) GetClosedPullRequests(org string) []PullRequest {

	client := &http.Client{}
	request, _ := http.NewRequest("GET", fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls?per_page=2&state=closed", org, r.Name), nil)
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
	MergedAt  string          `json:"merged_at,omitempty"`
	CreatedAt string          `json:"created_at"`
	User      PullRequestUser `json:"user"`
	Links     struct {
		Commits struct {
			CommitsURL string `json:"href"`
		} `json:"commits"`
	} `json:"_links"`
}

func (p *PullRequest) CalculateMinutesToMerge() (int64, error) {
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

	// TODO: sort commits by date the the later commits at the beginning
	// sort.....
	var lastCommit Commit
	for _, commit := range pullRequestCommits {
		if !strings.Contains(commit.CommitData.Message, "Merge branch 'main'") {
			lastCommit = commit
			break
		}
	}
	fmt.Printf("Commit with message %s at %s.\n", lastCommit.CommitData.Message, lastCommit.CommitData.Committer.Date)
	return 10, nil
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
	var mergeTimes []int64
	orgs := []string{"paketo-buildpacks", "paketo-community"}
	client := &http.Client{}

	for _, org := range orgs[0:1] {

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

		for _, repo := range repos[0:10] {
			pullRequests := repo.GetClosedPullRequests(org)

			for _, pullRequest := range pullRequests {
				if pullRequest.MergedAt == "" {
					continue
				}
				mergeTime, _ := pullRequest.CalculateMinutesToMerge()
				mergeTimes = append(mergeTimes, mergeTime)
			}

		}
	}
	fmt.Println(mergeTimes)

}
