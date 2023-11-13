package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"net/http/httputil"
	"os"
)

func main() {
	var config struct {
		Endpoint string
		Repo     string
		Token    string
		Version  string
	}

	flag.StringVar(&config.Endpoint, "endpoint", "https://api.github.com", "Specifies endpoint for sending requests")
	flag.StringVar(&config.Repo, "repo", "", "Specifies repo for sending requests")
	flag.StringVar(&config.Token, "token", "", "Github Authorization Token")
	flag.StringVar(&config.Version, "version", "", "Optional specific release version to reset")
	flag.Parse()

	if config.Repo == "" {
		fail(errors.New(`missing required input "repo"`))
	}

	if config.Token == "" {
		fail(errors.New(`missing required input "token"`))
	}

	req, err := http.NewRequest("GET", fmt.Sprintf("%s/repos/%s/releases", config.Endpoint, config.Repo), nil)
	if err != nil {
		fail(err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("token %s", config.Token))

	fmt.Println(`Fetching latest releases`)
	fmt.Printf("  Repository: %s\n", config.Repo)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fail(err)
	}

	if resp.StatusCode != http.StatusOK {
		dump, _ := httputil.DumpResponse(resp, true)
		fail(fmt.Errorf("unexpected response from list releases request: %s", dump))
	}

	var releases []struct {
		ID      int    `json:"id"`
		Draft   bool   `json:"draft"`
		TagName string `json:"tag_name"`
	}
	err = json.NewDecoder(resp.Body).Decode(&releases)
	if err != nil {
		fail(err)
	}

	err = resp.Body.Close()
	if err != nil {
		fail(err)
	}

	if len(releases) == 0 {
		fmt.Println("No releases, exiting.")
		return
	}

	releaseToDelete := releases[0]

	// If version is passed in, look for matching draft
	if config.Version != "" {
		found := false
		for _, r := range releases {
			if r.TagName == config.Version && r.Draft {
				fmt.Printf("Matching draft version %s found\n", config.Version)

				releaseToDelete = r
				found = true
				break
			}
		}

		if !found {
			fmt.Printf("No releases matching version %s found, exiting.\n", config.Version)
			return
		}
	} else if !releases[0].Draft {
		fmt.Println("Latest release is published, exiting.")
		return
	}

	fmt.Println("Latest release is draft, deleting.")

	req, err = http.NewRequest("DELETE", fmt.Sprintf("%s/repos/%s/releases/%d", config.Endpoint, config.Repo, releaseToDelete.ID), nil)
	if err != nil {
		fail(err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("token %s", config.Token))

	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		fail(err)
	}

	if resp.StatusCode != http.StatusNoContent {
		dump, _ := httputil.DumpResponse(resp, true)
		fail(fmt.Errorf("unexpected response from delete draft release request: %s", dump))
	}

	outputFileName, ok := os.LookupEnv("GITHUB_OUTPUT")
	if !ok {
		fail(errors.New("GITHUB_OUTPUT is not set, see https://docs.github.com/en/actions/using-workflows/workflow-commands-for-github-actions#setting-an-output-parameter"))
	}
	file, err := os.OpenFile(outputFileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		fail(err)
	}
	defer file.Close()
	fmt.Fprintf(file, "current_version=%s\n", releaseToDelete.TagName)

	fmt.Println("Success!")
}

func fail(err error) {
	fmt.Printf("Error: %s", err)
	os.Exit(1)
}
