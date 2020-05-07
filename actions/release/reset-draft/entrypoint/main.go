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
	}

	flag.StringVar(&config.Endpoint, "endpoint", "https://api.github.com", "Specifies endpoint for sending requests")
	flag.StringVar(&config.Repo, "repo", "", "Specifies repo for sending requests")
	flag.StringVar(&config.Token, "token", "", "Github Authorization Token")
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
		ID    int  `json:"id"`
		Draft bool `json:"draft"`
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

	if !releases[0].Draft {
		fmt.Println("Latest release is published, exiting.")
		return
	}

	fmt.Println("Latest release is draft, deleting.")

	req, err = http.NewRequest("DELETE", fmt.Sprintf("%s/repos/%s/releases/%d", config.Endpoint, config.Repo, releases[0].ID), nil)
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

	fmt.Println("Success!")
}

func fail(err error) {
	fmt.Printf("Error: %s", err)
	os.Exit(1)
}
