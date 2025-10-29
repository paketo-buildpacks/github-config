package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"maps"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/mmcdole/gofeed"

	backoff "github.com/cenkalti/backoff/v4"
)

const DEFAULT_GITHUB_USNS_URL = "https://raw.githubusercontent.com/canonical/ubuntu-security-notices/refs/heads/main/usn"
const UBUNTU_CVE_URL = "https://ubuntu.com/security/cve"
const FEED_RSS_URL = "https://ubuntu.com/security/notices/rss.xml"

var supportedDistros = map[string]string{
	"noble":  `24\.04`,
	"jammy":  `22\.04`,
	"focal":  `20\.04`,
	"bionic": `18\.04`,
}

type USN struct {
	AffectedPackages []string `json:"affected_packages"`
	CVEs             []CVE    `json:"cves"`
	Title            string   `json:"title"`
	ID               string   `json:"id"`
	URL              url.URL  `json:"-"`
}

type CVE struct {
	Title string `json:"title"`
	URL   string `json:"url"`
}

type Binary struct {
	Pocket  string `json:"pocket"`
	Version string `json:"version"`
}

type ReleaseData struct {
	Binaries map[string]Binary `json:"binaries"`
}

type usnGithubJsonResponse struct {
	CVEs     []string               `json:"cves"`
	ID       string                 `json:"id"`
	Releases map[string]ReleaseData `json:"releases"`
}

var config struct {
	Distro               string
	LastUSNsJSON         string
	LastUSNsJSONFilepath string
	Output               string
	PackagesJSON         string
	PackagesJSONFilepath string
	RSSURL               string
	RetryTimeLimit       string
	GhUsnUrl             string
}

func main() {

	flag.StringVar(&config.LastUSNsJSON,
		"last-usns",
		"",
		"JSON array of last known USNs")
	flag.StringVar(&config.LastUSNsJSONFilepath,
		"last-usns-filepath",
		"",
		"Filepath that points to the JSON array of last known USNs")
	flag.StringVar(&config.RSSURL,
		"feed-url",
		FEED_RSS_URL,
		"URL of RSS feed")
	flag.StringVar(&config.PackagesJSON,
		"packages",
		"",
		"JSON array of relevant packages")
	flag.StringVar(&config.PackagesJSONFilepath,
		"packages-filepath",
		"",
		"Filepath that points to the JSON array of relevant packages")
	flag.StringVar(&config.Distro,
		"distro",
		"",
		"Name of Ubuntu distro: bionic, focal, jammy, noble")
	flag.StringVar(&config.Output,
		"output",
		"",
		"Path to output JSON file")

	flag.StringVar(&config.RetryTimeLimit, "retry-time-limit", "5m", "How long to retry failures for")

	flag.Parse()

	_, ok := supportedDistros[config.Distro]
	if !ok {
		log.Fatalf("--distro flag has to be one of the following values: %v", slices.Sorted(maps.Keys(supportedDistros)))
	}

	if config.LastUSNsJSON == "" {
		config.LastUSNsJSON = `[]`
	}

	if config.PackagesJSON == "" {
		config.PackagesJSON = `[]`
	}

	if ghUsnBaseUrl, exists := os.LookupEnv("GITHUB_USN_BASE_URL"); exists {
		config.GhUsnUrl = ghUsnBaseUrl
	} else {
		config.GhUsnUrl = DEFAULT_GITHUB_USNS_URL
	}

	retryTimeLimit, err := time.ParseDuration(config.RetryTimeLimit)
	if err != nil {
		log.Fatal(err)
	}

	var lastUSNs []USN
	err = json.Unmarshal([]byte(config.LastUSNsJSON), &lastUSNs)
	if err != nil {
		log.Fatal(err)
	}

	if config.LastUSNsJSONFilepath != "" {

		lastUSNsFilepath, err := os.ReadFile(config.LastUSNsJSONFilepath)
		if err != nil {
			log.Fatal(err)
		}

		err = json.Unmarshal(lastUSNsFilepath, &lastUSNs)
		if err != nil {
			log.Fatal(err)
		}
	}

	var packages []string
	err = json.Unmarshal([]byte(config.PackagesJSON), &packages)
	if err != nil {
		log.Fatal(err)
	}

	if config.PackagesJSONFilepath != "" {

		packagesFilepath, err := os.ReadFile(config.PackagesJSONFilepath)
		if err != nil {
			log.Fatal(err)
		}

		err = json.Unmarshal(packagesFilepath, &packages)
		if err != nil {
			log.Fatal(err)
		}
	}

	newUSNs, err := getNewUSNsFromFeed(config.RSSURL, lastUSNs, retryTimeLimit)
	if err != nil {
		log.Fatal(err)
	}

	filtered := filterUSNsByPackages(newUSNs, packages)

	fmt.Println("Getting CVE metadata for relevant USNs...")

	output, err := json.Marshal(filtered)
	if err != nil {
		log.Fatal(err)
	}

	if len(filtered) == 0 {
		output = []byte(`[]`)
	}

	if config.Output != "" {
		path, err := filepath.Abs(config.Output)
		if err != nil {
			log.Fatal(err)
		}
		err = os.WriteFile(path, output, os.ModePerm)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		outputFileName, ok := os.LookupEnv("GITHUB_OUTPUT")
		if !ok {
			log.Fatal("GITHUB_OUTPUT is not set, see https://docs.github.com/en/actions/using-workflows/workflow-commands-for-github-actions#setting-an-output-parameter")
		}
		file, err := os.OpenFile(outputFileName, os.O_APPEND|os.O_WRONLY, 0)
		if err != nil {
			log.Fatal(err)
		}
		defer file.Close()

		fmt.Fprintf(file, "usns=%s\n", string(output))
	}
}

func filterUSNsByPackages(usns []USN, packages []string) (filtered []USN) {
	if len(packages) == 0 {
		fmt.Println("No packages specified. Skipping filtering.")
		return usns
	}

	fmt.Println("Filtering USNs by affected packages...")
	for _, usn := range usns {
	matchPkgs:
		for _, affected := range usn.AffectedPackages {
			for _, pkg := range packages {
				if pkg == affected {
					filtered = append(filtered, usn)
					fmt.Printf("USN '%s' contains affected package '%s'\n", usn.Title, affected)
					break matchPkgs
				}
			}
		}
	}
	return filtered
}

func getNewUSNsFromFeed(rssURL string, lastUSNs []USN, retryTimeLimit time.Duration) ([]USN, error) {
	fp := gofeed.NewParser()

	var feed *gofeed.Feed
	var err error

	exponentialBackoff := backoff.NewExponentialBackOff()
	exponentialBackoff.MaxElapsedTime = retryTimeLimit
	err = backoff.RetryNotify(func() error {
		feed, err = fp.ParseURL(rssURL)
		if err == nil {
			return nil
		}
		var httpError gofeed.HTTPError
		if errors.As(err, &httpError) {
			return fmt.Errorf("error parsing rss feed: %w", err)
		}
		return &backoff.PermanentError{Err: err}
	},
		exponentialBackoff,
		func(err error, t time.Duration) {
			log.Println(err)
			log.Printf("Retrying in %s\n", t)
		},
	)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Looking for new USNs...")
	var feedUSNs []USN
	for _, item := range feed.Items {
		// regex extracts 'USN-5464-1' from 'USN-5464-1: e2fsprogs vulnerability'
		re := regexp.MustCompile(`USN\-\d+\-\d+`)
		usnId := re.FindString(item.Title)

		if !isNewUSN(usnId, lastUSNs) {
			continue
		}
		fmt.Printf("New USN found: %s\n", item.Title)

		usnURL, err := url.Parse(item.Link)
		if err != nil {
			return nil, fmt.Errorf("error parsing URL of USN %s: %w", item.Title, err)
		}
		usnNumId := strings.TrimPrefix(usnId, "USN-")
		usnGithubURL := fmt.Sprintf("%s/%s", config.GhUsnUrl, usnNumId+".json")

		var usnBody []byte
		var code int
		var parsedUSNBody usnGithubJsonResponse

		err = backoff.RetryNotify(func() error {
			usnBody, code, err = get(usnGithubURL)
			if err != nil {
				return fmt.Errorf("error getting USN: %w", err)
			}
			if code != http.StatusOK {
				return fmt.Errorf("unexpected status code getting USN: %d", code)
			}
			err = json.Unmarshal(usnBody, &parsedUSNBody)
			if err != nil {
				return fmt.Errorf("error unmarshalling USN body: %w", err)
			}
			return nil
		},
			backoff.WithMaxRetries(backoff.NewExponentialBackOff(), 3),
			func(err error, t time.Duration) {
				fmt.Println(err)
				fmt.Printf("Retrying in %s seconds\n", t)
			},
		)

		var CVEs []CVE
		for _, cve := range parsedUSNBody.CVEs {
			CVE := CVE{
				Title: cve,
				URL:   path.Join(UBUNTU_CVE_URL, cve),
			}
			CVEs = append(CVEs, CVE)
		}

		feedUSNs = append(feedUSNs, USN{
			ID:               usnId,
			Title:            item.Title,
			CVEs:             CVEs,
			URL:              *usnURL,
			AffectedPackages: getAffectedPackages(parsedUSNBody, config.Distro),
		})
	}

	return feedUSNs, nil
}

func isNewUSN(id string, lastUSNs []USN) bool {
	for _, lastUSN := range lastUSNs {
		if id == lastUSN.ID {
			return false
		}
	}
	return true
}

func getAffectedPackages(usnBody usnGithubJsonResponse, distro string) []string {

	packages := make([]string, 0)

	for packageName := range usnBody.Releases[distro].Binaries {
		packages = append(packages, packageName)
	}

	return packages
}

func get(url string) ([]byte, int, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, 0, err
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, err
	}

	return body, resp.StatusCode, nil
}
