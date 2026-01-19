package main

import (
	"encoding/json"
	"flag"
	"fmt"
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
	AffectedPackages []UbuntuPackage `json:"affected_packages"`
	CVEs             []CVE           `json:"cves"`
	Title            string          `json:"title"`
	ID               string          `json:"id"`
	URL              string          `json:"url"`
}

type USNsResponse struct {
	Notices []Notice `json:"notices"`
}

type Notice struct {
	ID              string                     `json:"id"`
	Title           string                     `json:"title"`
	Published       string                     `json:"published"`
	CVEs            []CVE                      `json:"cves"`
	ReleasePackages map[string][]UbuntuPackage `json:"release_packages"`
}

// CVE represents the linked vulnerabilities
type CVE struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}

// UbuntuPackage represents the specific package versions fixed
type UbuntuPackage struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type USNOutput struct {
	AffectedPackages []string    `json:"affected_packages"`
	CVEs             []CVEOutput `json:"cves"`
	Title            string      `json:"title"`
	ID               string      `json:"id"`
	URL              string      `json:"url"`
}

type CVEOutput struct {
	Title string `json:"title"`
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

	if config.LastUSNsJSON == "" {
		config.LastUSNsJSON = `[]`
	}

	if config.PackagesJSON == "" {
		config.PackagesJSON = `[]`
	}

	var lastUSNs []USN
	err := json.Unmarshal([]byte(config.LastUSNsJSON), &lastUSNs)
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

	newUSNs, err := getNewUSNsFromJSONApi(config.APIUrl, lastUSNs, config.Distro)
	if err != nil {
		log.Fatal(err)
	}

	filtered := filterUSNsByPackages(newUSNs, packages)

	transformed := transformUSNsForOutput(filtered)

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

func transformUSNsForOutput(usns []USN) []USNOutput {
	output := []USNOutput{}
	for _, usn := range usns {
		var packageNames []string
		for _, pkg := range usn.AffectedPackages {
			packageNames = append(packageNames, pkg.Name)
		}

		var cves []CVEOutput
		for _, cve := range usn.CVEs {
			cves = append(cves, CVEOutput{
				Title: cve.ID,
				URL:   fmt.Sprintf("https://ubuntu.com/security/%s", cve.ID),
			})
		}

		output = append(output, USNOutput{
			ID:               usn.ID,
			Title:            fmt.Sprintf("%s: %s", usn.ID, usn.Title),
			CVEs:             cves,
			URL:              usn.URL,
			AffectedPackages: packageNames,
		})
	}
	return output
}

func getNewUSNsFromJSONApi(jsonApiUrl string, lastUSNs []USN, distro string) ([]USN, error) {
	var allNotices []Notice

	offsets := []int{0, 20}
	for _, offset := range offsets {
		paginatedUrl := fmt.Sprintf("%s?release=%s&limit=%d&offset=%d", jsonApiUrl, distro, 20, offset)
		notices, err := fetchUSNPage(paginatedUrl)
		if err != nil {
			return nil, err
		}
		allNotices = append(allNotices, notices...)
	}

	fmt.Println("Looking for new USNs...")
	var feedUSNs []USN
	for _, item := range allNotices {
		usnId := item.ID

		fmt.Printf("USN ID: %s\n", usnId)
		if !isNewUSN(usnId, lastUSNs) {
			continue
		}
		fmt.Printf("New USN found: %s\n", item.Title)

		usnURL := fmt.Sprintf("https://ubuntu.com/security/notices/%s", usnId)

		feedUSNs = append(feedUSNs, USN{
			ID:               usnId,
			Title:            item.Title,
			CVEs:             item.CVEs,
			URL:              usnURL,
			AffectedPackages: item.ReleasePackages[distro],
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

func fetchUSNPage(url string) ([]Notice, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status: %d", resp.StatusCode)
	}

	var data USNsResponse
	if err = json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	return data.Notices, nil
}
