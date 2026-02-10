package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"time"
)

const JSON_API_URL = "https://ubuntu.com/security/notices.json"

var supportedDistros = []string{
	"noble",
	"jammy",
	"focal",
	"bionic",
}

type USN struct {
	CVEs []struct {
		ID string `json:"id"`
	} `json:"cves"`
	Title           string `json:"title"`
	ID              string `json:"id"`
	ReleasePackages map[string][]struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"release_packages"`
}

type PatchedUsnsInputOutput struct {
	AffectedPackages []string `json:"affected_packages"`
	CVEs             []struct {
		Title string `json:"title"`
		URL   string `json:"url"`
	} `json:"cves"`
	Title string `json:"title"`
	ID    string `json:"id"`
	URL   string `json:"url"`
}

func main() {
	var config struct {
		Distro               string
		LastUSNsJSON         string
		LastUSNsJSONFilepath string
		Output               string
		PackagesJSON         string
		PackagesJSONFilepath string
		APIUrl               string
	}

	flag.StringVar(&config.LastUSNsJSON,
		"last-usns",
		"",
		"JSON array of last known USNs")
	flag.StringVar(&config.LastUSNsJSONFilepath,
		"last-usns-filepath",
		"",
		"Filepath that points to the JSON array of last known USNs")
	flag.StringVar(&config.APIUrl,
		"api-url",
		JSON_API_URL,
		"URL of the Ubuntu security notices JSON API")
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

	flag.Parse()

	if !slices.Contains(supportedDistros, config.Distro) {
		log.Fatalf("--distro flag has to be one of the following values: %v", supportedDistros)
	}

	lastPatchedUSNs := []PatchedUsnsInputOutput{}
	if config.LastUSNsJSON != "" {
		err := json.Unmarshal([]byte(config.LastUSNsJSON), &lastPatchedUSNs)
		if err != nil {
			log.Fatal(err)
		}
	}

	if config.LastUSNsJSONFilepath != "" {

		lastUSNsFilepath, err := os.ReadFile(config.LastUSNsJSONFilepath)
		if err != nil {
			log.Fatal(err)
		}

		err = json.Unmarshal(lastUSNsFilepath, &lastPatchedUSNs)
		if err != nil {
			log.Fatal(err)
		}
	}

	packages := []string{}
	if config.PackagesJSON != "" {
		err := json.Unmarshal([]byte(config.PackagesJSON), &packages)
		if err != nil {
			log.Fatal(err)
		}
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

	newUSNs, err := getNewUSNsFromJSONApi(config.APIUrl, lastPatchedUSNs, config.Distro)
	if err != nil {
		log.Fatal(err)
	}

	filteredUSNs := filterUSNsByPackages(newUSNs, packages, config.Distro)

	transformed := transformUSNsForOutput(filteredUSNs, config.Distro)

	output, err := json.Marshal(transformed)
	if err != nil {
		log.Fatal(err)
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

func filterUSNsByPackages(usns []USN, packages []string, distro string) (filtered []USN) {
	if len(packages) == 0 {
		fmt.Println("No packages specified. Skipping filtering.")
		return usns
	}

	fmt.Println("Filtering USNs by affected packages...")
	for _, usn := range usns {
	matchPkgs:
		for _, affected := range usn.ReleasePackages[distro] {
			for _, pkg := range packages {
				if pkg == affected.Name {
					filtered = append(filtered, usn)
					fmt.Printf("USN '%s' contains affected package '%s'\n", usn.Title, affected.Name)
					break matchPkgs
				}
			}
		}
	}
	return filtered
}

func transformUSNsForOutput(usns []USN, distro string) []PatchedUsnsInputOutput {
	output := []PatchedUsnsInputOutput{}
	for _, usn := range usns {
		var packageNames []string
		for _, pkg := range usn.ReleasePackages[distro] {
			packageNames = append(packageNames, pkg.Name)
		}

		var cves []struct {
			Title string `json:"title"`
			URL   string `json:"url"`
		}
		for _, cve := range usn.CVEs {
			cves = append(cves, struct {
				Title string `json:"title"`
				URL   string `json:"url"`
			}{
				Title: cve.ID,
				URL:   fmt.Sprintf("https://ubuntu.com/security/%s", cve.ID),
			})
		}

		output = append(output, PatchedUsnsInputOutput{
			ID:               usn.ID,
			Title:            fmt.Sprintf("%s: %s", usn.ID, usn.Title),
			CVEs:             cves,
			URL:              fmt.Sprintf("https://ubuntu.com/security/notices/%s", usn.ID),
			AffectedPackages: packageNames,
		})
	}
	return output
}

func getNewUSNsFromJSONApi(jsonApiUrl string, lastPatchedUSNs []PatchedUsnsInputOutput, distro string) ([]USN, error) {
	var allUSNs []USN

	offsets := []int{0, 20}
	for _, offset := range offsets {
		paginatedUrl := fmt.Sprintf("%s?release=%s&limit=%d&offset=%d", jsonApiUrl, distro, 20, offset)
		usns, err := fetchUSNPage(paginatedUrl)
		if err != nil {
			return nil, err
		}
		allUSNs = append(allUSNs, usns...)
	}

	fmt.Println("Looking for new USNs...")
	var newUSNs []USN
	for _, usn := range allUSNs {

		if !isNewUSN(usn.ID, lastPatchedUSNs) {
			continue
		}

		newUSNs = append(newUSNs, USN{
			ID:              usn.ID,
			Title:           usn.Title,
			CVEs:            usn.CVEs,
			ReleasePackages: usn.ReleasePackages,
		})
	}

	if len(newUSNs) > 0 {
		for _, usn := range newUSNs {
			fmt.Printf("New USN found: %s with name %s\n", usn.ID, usn.Title)
		}
	} else {
		fmt.Println("No new USNs found")
	}

	return newUSNs, nil
}

func isNewUSN(id string, lastPatchedUSNs []PatchedUsnsInputOutput) bool {
	for _, lastPatchedUSN := range lastPatchedUSNs {
		if id == lastPatchedUSN.ID {
			return false
		}
	}
	return true
}

const (
	httpTimeout = 90 * time.Second
	maxRetries  = 3
	retryDelay  = 5 * time.Second
)

func fetchUSNPage(url string) ([]USN, error) {

	type USNsResponse struct {
		Notices []USN `json:"notices"`
	}

	client := &http.Client{Timeout: httpTimeout}
	for attempt := range maxRetries {
		if attempt > 0 {
			log.Printf("Retrying request to %s (attempt %d/%d) after %v seconds", url, attempt+1, maxRetries, retryDelay.Seconds())
			time.Sleep(retryDelay)
		}

		resp, err := client.Get(url)
		if err != nil {
			log.Printf("Request to %s failed with: %v", url, err)
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			_, _ = io.Copy(io.Discard, resp.Body)
			log.Printf("API request failed with status: %d", resp.StatusCode)
			continue
		}

		var data USNsResponse
		if err = json.NewDecoder(resp.Body).Decode(&data); err != nil {
			log.Printf("Failed to decode JSON response with: %v", err)
			continue
		}
		return data.Notices, nil
	}
	return nil, fmt.Errorf("failed after %d attempts", maxRetries)
}
