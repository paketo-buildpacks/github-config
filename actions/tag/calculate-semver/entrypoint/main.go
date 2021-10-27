package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"net/http/httputil"
	"os"

	"golang.org/x/oauth2"

	"github.com/Masterminds/semver"
)

type Commit struct {
	SHA string `json:"sha"`
}

type Config struct {
	Endpoint string
	Repo     string
	Token    string
}

type Label struct {
	Name string `json:"name"`
}

type PullRequest struct {
	Number int     `json:"number"`
	Labels []Label `json:"labels"`
}

const (
	PATCH int = 0
	MINOR int = 1
	MAJOR int = 2
)

func main() {
	var config Config

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

	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: config.Token},
	)
	ghClient := oauth2.NewClient(ctx, ts)

	// Validate that the repo exists
	uri := fmt.Sprintf("%s/repos/%s", config.Endpoint, config.Repo)
	resp, err := ghClient.Get(uri)
	if err != nil {
		fail(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		dump, _ := httputil.DumpResponse(resp, true)
		fail(fmt.Errorf("failed to get repo: unexpected response: %s", dump))
	}

	prevVersion, err := getLatestVersion(ghClient, config)
	if err != nil {
		fail(err)
	}

	// there are no releases on the repo
	if prevVersion == nil {
		fmt.Println("::set-output name=tag::0.0.1")
		os.Exit(0)
	}

	PRsWithSizes, err := getPRsSinceLastRelease(ghClient, config, prevVersion)
	if err != nil {
		fail(err)
	}

	largestChange := PATCH
	for _, v := range PRsWithSizes {
		if v > largestChange {
			largestChange = v
		}
	}

	next := calculateNextSemver(*prevVersion, largestChange)
	fmt.Printf("::set-output name=tag::%s", next.String())
}

func fail(err error) {
	fmt.Printf("Error: %s", err)
	os.Exit(1)
}

func getLatestVersion(client *http.Client, config Config) (*semver.Version, error) {
	uri := fmt.Sprintf("%s/repos/%s/releases/latest", config.Endpoint, config.Repo)
	resp, err := client.Get(uri)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// The repo has no releases
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		dump, _ := httputil.DumpResponse(resp, true)
		return nil, fmt.Errorf("failed to get latest release: unexpected response: %s", dump)
	}

	var latestRelease struct {
		TagName string `json:"tag_name"`
	}
	err = json.NewDecoder(resp.Body).Decode(&latestRelease)
	if err != nil {
		return nil, fmt.Errorf("failed to decode latest release: %w", err)
	}

	prevVersion, err := semver.NewVersion(latestRelease.TagName)
	if err != nil {
		return nil, fmt.Errorf("latest release tag '%s' isn't semver versioned: %w", latestRelease.TagName, err)
	}
	return prevVersion, nil
}

func getPRsSinceLastRelease(client *http.Client, config Config, previous *semver.Version) (map[int]int, error) {
	PRsWithSizes := map[int]int{}
	uri := fmt.Sprintf("%s/repos/%s/compare/%s...main", config.Endpoint, config.Repo, previous.Original())
	resp, err := client.Get(uri)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		dump, _ := httputil.DumpResponse(resp, true)
		return nil, fmt.Errorf("failed to get commits since last release: unexpected response: %s", dump)
	}

	var comparison struct {
		Commits []Commit `json:"commits"`
	}
	err = json.NewDecoder(resp.Body).Decode(&comparison)
	if err != nil {
		return nil, fmt.Errorf("failed to parse commits since release response: %w", err)
	}

	for _, commit := range comparison.Commits {
		uri = fmt.Sprintf("%s/repos/%s/commits/%s/pulls", config.Endpoint, config.Repo, commit.SHA)
		resp, err = client.Get(uri)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			dump, _ := httputil.DumpResponse(resp, true)
			return nil, fmt.Errorf("failed to get pull requests for commit: unexpected response: %s", dump)
		}

		var commitPRs []PullRequest
		err = json.NewDecoder(resp.Body).Decode(&commitPRs)
		if err != nil {
			return nil, fmt.Errorf("failed to parse commit PRs response: %w", err)
		}

		for _, pr := range commitPRs {
			for _, label := range pr.Labels {
				if _, ok := PRsWithSizes[pr.Number]; ok && isSemverLabel(label.Name) {
					return nil, fmt.Errorf("PR %d has multiple semver labels", pr.Number)
				}
				switch label.Name {
				case "semver:patch":
					PRsWithSizes[pr.Number] = PATCH
				case "semver:minor":
					PRsWithSizes[pr.Number] = MINOR
				case "semver:major":
					PRsWithSizes[pr.Number] = MAJOR
				default:
					continue
				}
			}
			if _, ok := PRsWithSizes[pr.Number]; !ok {
				return nil, fmt.Errorf("PR %d has no semver label", pr.Number)
			}
		}
	}
	return PRsWithSizes, nil
}

func isSemverLabel(label string) bool {
	return label == "semver:patch" || label == "semver:minor" || label == "semver:major"
}

func calculateNextSemver(previous semver.Version, largestChange int) semver.Version {
	switch largestChange {
	case 0:
		return previous.IncPatch()
	case 1:
		return previous.IncMinor()
	case 2:
		return previous.IncMajor()
	default:
		fail(fmt.Errorf("input change size doesn't correspond to patch/minor/major: %d", largestChange))
	}
	return semver.Version{}
}
