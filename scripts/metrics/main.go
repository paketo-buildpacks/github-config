package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aclements/go-moremath/stats"
)

const dateLayout string = "2006-01-02T15:04:05Z"

var orgs = []string{"paketo-buildpacks", "paketo-community"}

type Repository struct {
	Name  string `json:"name"`
	URL   string `json:"url"`
	Owner struct {
		Login string `json:"login"`
	} `json:"owner"`
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
	start := time.Now()

	if os.Getenv("PAKETO_GITHUB_TOKEN") == "" {
		fmt.Println("Please set PAKETO_GITHUB_TOKEN\nExiting.")
		return
	}

	numWorkers := 2
	if os.Getenv("NUM_WORKERS") != "" {
		numWorkers, _ = strconv.Atoi(os.Getenv("NUM_WORKERS"))
	}

	in := getOrgReposChan(orgs)

	fmt.Printf("Running with %d workers...\nUse NUM_WORKERS to set.\n\n", numWorkers)
	var workers []<-chan float64
	for i := 0; i < numWorkers; i++ {
		workers = append(workers, worker(i, in))
	}

	for time := range merge(workers...) {
		mergeTimes = append(mergeTimes, time)
	}
	mergeTimesSample := stats.Sample{Xs: mergeTimes}
	fmt.Printf("\nMerge Time Stats\nFor %d pull requests\n    Average: %f hours\n    Median %f hours\n    95th Percentile: %f hours\n", len(mergeTimesSample.Xs), (mergeTimesSample.Mean() / 60), (mergeTimesSample.Quantile(0.5) / 60), (mergeTimesSample.Quantile(0.95) / 60))

	duration := time.Since(start)
	fmt.Printf("Execution took %f seconds.\n", duration.Seconds())
}

func worker(id int, input <-chan Repository) chan float64 {
	output := make(chan float64)

	go func() {
		for repo := range input {
			time.Sleep(time.Millisecond * 200)
			getRepoMergeTimes(repo, output)
		}
		close(output)
	}()
	return output
}

func merge(ws ...<-chan float64) chan float64 {
	var wg sync.WaitGroup
	output := make(chan float64)

	getTimes := func(c <-chan float64) {
		for time := range c {
			output <- time
		}
		wg.Done()
	}
	wg.Add(len(ws))
	for _, w := range ws {
		go getTimes(w)
	}
	go func() {
		wg.Wait()
		close(output)
	}()
	return output
}

func getOrgReposChan(orgs []string) chan Repository {
	output := make(chan Repository)
	go func() {
		for _, org := range orgs {
			for _, repo := range getOrgRepos(org) {
				output <- repo
			}
		}
		close(output)
	}()
	return output
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
		panic(string(body))
	}
	return repos
}

func getRepoMergeTimes(repo Repository, output chan float64) {
	pullRequests := getClosedPullRequests(repo)
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
		fmt.Printf("Pull request %s/%s #%d by %s\ntook %f minutes to merge.\n", repo.Owner.Login, repo.Name, pullRequest.Number, pullRequest.User.Login, mergeTime)
		output <- mergeTime
	}
}

func getClosedPullRequests(repo Repository) []PullRequest {
	client := &http.Client{}
	requestURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls?per_page=200&state=closed", repo.Owner.Login, repo.Name)
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
		panic(string(body))
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
