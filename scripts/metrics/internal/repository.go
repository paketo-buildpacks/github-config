package internal

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"
)

type Repository struct {
	Name  string `json:"name"`
	URL   string `json:"url"`
	Owner struct {
		Login string `json:"login"`
	} `json:"owner"`
}

func GetOrgRepos(org string, serverURI string) []Repository {
	client := &http.Client{}
	request, _ := http.NewRequest("GET", fmt.Sprintf("%s/orgs/%s/repos?per_page=100", serverURI, org), nil)
	request.Header.Add("Authorization", fmt.Sprintf("token %s", os.Getenv("PAKETO_GITHUB_TOKEN")))

	response, err := client.Do(request)
	if err != nil {
		panic(err)
	}
	body, _ := ioutil.ReadAll(response.Body)

	repos := []Repository{}
	err = json.Unmarshal(body, &repos)
	if err != nil {
		panic(string(body))
	}
	return repos
}

func GetRepoMergeTimes(repo Repository, serverURI string, output chan float64) {
	pullRequests := getClosedPullRequests(repo, serverURI)
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
		mergeTime := calculateMinutesToMerge(pullRequest, serverURI)
		fmt.Printf("Pull request %s/%s #%d by %s\ntook %f minutes to merge.\n", repo.Owner.Login, repo.Name, pullRequest.Number, pullRequest.User.Login, mergeTime)
		output <- mergeTime
	}
}

func getClosedPullRequests(repo Repository, serverURI string) []PullRequest {
	client := &http.Client{}
	requestURL := fmt.Sprintf("%s/repos/%s/%s/pulls?per_page=200&state=closed", serverURI, repo.Owner.Login, repo.Name)
	request, _ := http.NewRequest("GET", requestURL, nil)
	request.Header.Add("Authorization", fmt.Sprintf("token %s", os.Getenv("PAKETO_GITHUB_TOKEN")))

	response, err := client.Do(request)
	if err != nil {
		panic(err)
	}

	body, _ := ioutil.ReadAll(response.Body)
	pullRequests := []PullRequest{}
	err = json.Unmarshal(body, &pullRequests)

	if err != nil {
		panic(fmt.Sprintf("Request: %s\nResponse: %s\n", requestURL, string(body)))
	}

	return pullRequests
}
