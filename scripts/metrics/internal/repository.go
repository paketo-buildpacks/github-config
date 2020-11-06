package internal

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type RepositoryContainer struct {
	Repository Repository
	Error      error
}

type Repository struct {
	Name  string `json:"name"`
	URL   string `json:"url"`
	Owner struct {
		Login string `json:"login"`
	} `json:"owner"`
}

func GetOrgRepos(org string, serverURI string) ([]Repository, error) {
	client := &http.Client{}
	uri, err := url.Parse(serverURI)
	if err != nil {
		return nil, fmt.Errorf("failed to parse server URL: %s", err)
	}

	uri.Path = fmt.Sprintf("/orgs/%s/repos", org)
	uri.RawQuery = "per_page=100"

	request, err := http.NewRequest("GET", uri.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create http GET request for repositories: %s", err)
	}

	request.Header.Add("Authorization", fmt.Sprintf("token %s", os.Getenv("PAKETO_GITHUB_TOKEN")))

	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("failed to make http request for repositories: %s", err)
	}
	body, _ := ioutil.ReadAll(response.Body)

	repos := []Repository{}
	err = json.Unmarshal(body, &repos)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal response: %s\n from API endpoint: %s : %s", string(body), uri.String(), err)
	}
	return repos, nil
}

func GetRepoMergeTimes(repo Repository, serverURI string, output chan MergeTimeContainer) {
	pullRequests, err := getClosedPullRequests(repo, serverURI)
	if err != nil {
		output <- MergeTimeContainer{Error: fmt.Errorf("failed to get closed pull requests: %s", err)}
		return
	}
	for _, pullRequest := range pullRequests {
		if pullRequest.MergedAt == "" {
			continue
		}
		mergedAtTime, err := time.Parse(dateLayout, pullRequest.MergedAt)
		if err != nil {
			output <- MergeTimeContainer{Error: fmt.Errorf("failed to parse merge time for a pull request: %s", err)}
			return
		}
		if mergedAtTime.Before(time.Now().Add(-time.Hour * 30 * 24)) {
			continue
		}
		if strings.Contains(pullRequest.User.Login, "bot") {
			continue
		}
		if pullRequest.User.Login == "nebhale" || pullRequest.User.Login == "ekcasey" {
			continue
		}
		if strings.Contains(pullRequest.Title, "rfc") || strings.Contains(pullRequest.Title, "RFC") {
			continue
		}
		mergeTime, err := calculateMinutesToMerge(pullRequest, serverURI)
		if err != nil {
			output <- MergeTimeContainer{Error: fmt.Errorf("failed to compute merge time for a pull request: %s", err)}
			return
		}
		fmt.Printf("Pull request %s/%s #%d by %s\ntook %f minutes to merge.\n", repo.Owner.Login, repo.Name, pullRequest.Number, pullRequest.User.Login, mergeTime)
		output <- MergeTimeContainer{MergeTime: mergeTime}
	}
}

func getClosedPullRequests(repo Repository, serverURI string) ([]PullRequest, error) {
	client := &http.Client{}
	uri, err := url.Parse(serverURI)
	if err != nil {
		return nil, fmt.Errorf("failed to parse server URL: %s", err)
	}
	uri.Path = fmt.Sprintf("/repos/%s/%s/pulls", repo.Owner.Login, repo.Name)
	uri.RawQuery = "per_page=200&state=closed"
	request, err := http.NewRequest("GET", uri.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create http GET request for closed PRs: %s", err)
	}

	request.Header.Add("Authorization", fmt.Sprintf("token %s", os.Getenv("PAKETO_GITHUB_TOKEN")))

	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("failed to make http request for closed PRs: %s", err)
	}

	body, _ := ioutil.ReadAll(response.Body)
	pullRequests := []PullRequest{}
	err = json.Unmarshal(body, &pullRequests)

	if err != nil {
		return nil, fmt.Errorf("could not unmarshal response: %s\n from API endpoint: %s : %s", string(body), uri.String(), err)
	}

	return pullRequests, nil
}
