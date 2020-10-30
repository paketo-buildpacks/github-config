package main

import (
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/aclements/go-moremath/stats"
	"github.com/paketo-buildpacks/github-config/scripts/metrics/internal"
)

const dateLayout string = "2006-01-02T15:04:05Z"

var orgs = []string{"paketo-buildpacks", "paketo-community"}

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

func worker(id int, input <-chan internal.Repository) chan float64 {
	output := make(chan float64)

	go func() {
		for repo := range input {
			time.Sleep(time.Millisecond * 200)
			internal.GetRepoMergeTimes(repo, output)
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

func getOrgReposChan(orgs []string) chan internal.Repository {
	output := make(chan internal.Repository)
	go func() {
		for _, org := range orgs {
			for _, repo := range internal.GetOrgRepos(org) {
				output <- repo
			}
		}
		close(output)
	}()
	return output
}
