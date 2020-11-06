package main

import (
	"flag"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/aclements/go-moremath/stats"
	"github.com/paketo-buildpacks/github-config/scripts/metrics/internal"
)

var orgs = []string{"paketo-buildpacks", "paketo-community"}

func main() {
	var mergeTimes []float64
	var githubServer string
	var numWorkers int
	start := time.Now()

	flag.StringVar(&githubServer, "server", "https://api.github.com", "base URL for the github API")
	flag.IntVar(&numWorkers, "workers", 1, "number of concurrent workers to use")
	flag.Parse()

	if os.Getenv("PAKETO_GITHUB_TOKEN") == "" {
		fmt.Println("Please set PAKETO_GITHUB_TOKEN")
		os.Exit(1)
	}

	in := getOrgReposChan(orgs, githubServer)

	fmt.Printf("Running with %d workers...\nUse --workers to set.\n\n", numWorkers)
	var workers []<-chan internal.MergeTimeContainer
	for i := 0; i < numWorkers; i++ {
		workers = append(workers, worker(i, githubServer, in))
	}

	for timeContainer := range merge(workers...) {
		if err := timeContainer.Error; err != nil {
			fmt.Printf("failed to calculate merge times: %s\n", err)
			os.Exit(1)
		}
		mergeTimes = append(mergeTimes, timeContainer.MergeTime)
	}
	mergeTimesSample := stats.Sample{Xs: mergeTimes}
	fmt.Printf("\nMerge Time Stats\nFor %d pull requests\n    Average: %f hours\n    Median %f hours\n    95th Percentile: %f hours\n",
		len(mergeTimesSample.Xs),
		(mergeTimesSample.Mean() / 60),
		(mergeTimesSample.Quantile(0.5) / 60),
		(mergeTimesSample.Quantile(0.95) / 60))

	duration := time.Since(start)
	fmt.Printf("Execution took %f seconds.\n", duration.Seconds())
}

func worker(id int, serverURI string, input <-chan internal.RepositoryContainer) chan internal.MergeTimeContainer {
	output := make(chan internal.MergeTimeContainer)

	go func() {
		for repoContainer := range input {
			if repoContainer.Error != nil {
				output <- internal.MergeTimeContainer{Error: repoContainer.Error}
				close(output)
				return
			}
			internal.GetRepoMergeTimes(repoContainer.Repository, serverURI, output)
		}
		close(output)
	}()
	return output
}

func merge(ws ...<-chan internal.MergeTimeContainer) chan internal.MergeTimeContainer {
	var wg sync.WaitGroup
	output := make(chan internal.MergeTimeContainer)

	getTimes := func(c <-chan internal.MergeTimeContainer) {
		for timeContainer := range c {
			output <- timeContainer
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

func getOrgReposChan(orgs []string, serverURI string) chan internal.RepositoryContainer {
	output := make(chan internal.RepositoryContainer)
	go func() {
		for _, org := range orgs {
			repos, err := internal.GetOrgRepos(org, serverURI)
			if err != nil {
				output <- internal.RepositoryContainer{Error: fmt.Errorf("failed to get repositories: %s", err)}
			}
			for _, repo := range repos {
				repoContainer := internal.RepositoryContainer{
					Repository: repo,
					Error:      nil,
				}
				output <- repoContainer
			}
		}
		close(output)
	}()
	return output
}
