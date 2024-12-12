package main

import (
	"bytes"
	"crypto/rand"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"text/template"
)

//go:embed template.md
var tString string

type Package struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	PURL    string `json:"purl"`
}

type ModifiedPackage struct {
	Name            string `json:"name"`
	PreviousVersion string `json:"previousVersion"`
	CurrentVersion  string `json:"currentVersion"`
	PreviousPURL    string `json:"previousPurl"`
	CurrentPURL     string `json:"currentPurl"`
}

type USN struct {
	Title string `json:"title"`
	URL   string `json:"url"`
}

func main() {

	var config struct {
		BuildImage                string
		RunImage                  string
		BuildCveReport            string
		RunCveReport              string
		BuildPackagesAddedJSON    string
		BuildPackagesModifiedJSON string
		BuildPackagesRemovedJSON  string
		RunPackagesAddedJSON      string
		RunPackagesModifiedJSON   string
		RunPackagesRemovedJSON    string
		PatchedJSON               string
		SupportsUsns              string
		ReceiptsShowLimit         string
	}

	flag.StringVar(&config.BuildImage, "build-image", "", "Registry location of stack build image")
	flag.StringVar(&config.RunImage, "run-image", "", "Registry location of stack run image")
	flag.StringVar(&config.BuildCveReport, "build-cve-report", "", "CVE scan report path of build image in markdown format")
	flag.StringVar(&config.RunCveReport, "run-cve-report", "", "CVE scan report path of run image in markdown format")
	flag.StringVar(&config.PatchedJSON, "patched-usns", "", "JSON Array of patched USNs")
	flag.StringVar(&config.SupportsUsns, "supports-usns", "", "Boolean variable to show patched USNs in release notes")
	flag.StringVar(&config.BuildPackagesAddedJSON, "build-added", "", "JSON Array of packages added to build image")
	flag.StringVar(&config.BuildPackagesModifiedJSON, "build-modified", "", "JSON Array of packages modified in build image")
	flag.StringVar(&config.BuildPackagesRemovedJSON, "build-removed", "", "JSON Array of packages removed in build image")
	flag.StringVar(&config.RunPackagesAddedJSON, "run-added", "", "JSON Array of packages added to run image")
	flag.StringVar(&config.RunPackagesModifiedJSON, "run-modified", "", "JSON Array of packages modified in run image")
	flag.StringVar(&config.RunPackagesRemovedJSON, "run-removed", "", "JSON Array of packages removed in run image")
	flag.StringVar(&config.ReceiptsShowLimit, "receipts-show-limit", "", "Integer which defines the limit of whether it should show or not the receipts array of each image")
	flag.Parse()

	absolute, err := filepath.Abs(config.BuildPackagesAddedJSON)
	if err != nil {
		log.Fatalf("Failed to create absolute path for %s", config.BuildPackagesAddedJSON)
	}
	config.BuildPackagesAddedJSON = absolute

	absolute, err = filepath.Abs(config.BuildPackagesModifiedJSON)
	if err != nil {
		log.Fatalf("Failed to create absolute path for %s", config.BuildPackagesModifiedJSON)
	}
	config.BuildPackagesModifiedJSON = absolute

	absolute, err = filepath.Abs(config.BuildPackagesRemovedJSON)
	if err != nil {
		log.Fatalf("Failed to create absolute path for %s", config.BuildPackagesRemovedJSON)
	}
	config.BuildPackagesRemovedJSON = absolute

	absolute, err = filepath.Abs(config.RunPackagesAddedJSON)
	if err != nil {
		log.Fatalf("Failed to create absolute path for %s", config.RunPackagesAddedJSON)
	}
	config.RunPackagesAddedJSON = absolute

	absolute, err = filepath.Abs(config.RunPackagesModifiedJSON)
	if err != nil {
		log.Fatalf("Failed to create absolute path for %s", config.RunPackagesModifiedJSON)
	}
	config.RunPackagesModifiedJSON = absolute

	absolute, err = filepath.Abs(config.RunPackagesRemovedJSON)
	if err != nil {
		log.Fatalf("Failed to create absolute path for %s", config.RunPackagesRemovedJSON)
	}
	config.RunPackagesRemovedJSON = absolute

	var contents struct {
		PatchedArray      []USN
		SupportsUsns      bool
		BuildAdded        []Package
		BuildModified     []ModifiedPackage
		BuildRemoved      []Package
		RunAdded          []Package
		RunModified       []ModifiedPackage
		RunRemoved        []Package
		BuildImage        string
		RunImage          string
		BuildCveReport    string
		RunCveReport      string
		ReceiptsShowLimit int
	}

	err = json.Unmarshal([]byte(fixEmptyArray(config.PatchedJSON)), &contents.PatchedArray)
	if err != nil {
		log.Fatalf("failed unmarshalling patched USNs: %s", err.Error())
	}

	if config.SupportsUsns == "" {
		contents.SupportsUsns = true
	} else {
		contents.SupportsUsns, err = strconv.ParseBool(config.SupportsUsns)
		if err != nil {
			log.Fatalf("failed converting supportsUsns string to boolean: %s", err.Error())
		}
	}

	buildAddedFile, err := os.Open(config.BuildPackagesAddedJSON)
	if err != nil {
		log.Fatal(err)
	}
	defer buildAddedFile.Close()

	err = json.NewDecoder(buildAddedFile).Decode(&contents.BuildAdded)
	if err != nil {
		log.Fatalf("failed unmarshalling build packages added: %s", err.Error())
	}

	buildModifiedFile, err := os.Open(config.BuildPackagesModifiedJSON)
	if err != nil {
		log.Fatal(err)
	}
	defer buildModifiedFile.Close()

	err = json.NewDecoder(buildModifiedFile).Decode(&contents.BuildModified)
	if err != nil {
		log.Fatalf("failed unmarshalling build packages modified: %s", err.Error())
	}

	buildRemovedFile, err := os.Open(config.BuildPackagesRemovedJSON)
	if err != nil {
		log.Fatal(err)
	}
	defer buildRemovedFile.Close()

	err = json.NewDecoder(buildRemovedFile).Decode(&contents.BuildRemoved)
	if err != nil {
		log.Fatalf("failed unmarshalling build packages removed: %s", err.Error())
	}

	runAddedFile, err := os.Open(config.RunPackagesAddedJSON)
	if err != nil {
		log.Fatal(err)
	}
	defer runAddedFile.Close()

	err = json.NewDecoder(runAddedFile).Decode(&contents.RunAdded)
	if err != nil {
		log.Fatalf("failed unmarshalling run packages added: %s", err.Error())
	}

	runModifiedFile, err := os.Open(config.RunPackagesModifiedJSON)
	if err != nil {
		log.Fatal(err)
	}
	defer runModifiedFile.Close()

	err = json.NewDecoder(runModifiedFile).Decode(&contents.RunModified)
	if err != nil {
		log.Fatalf("failed unmarshalling run packages modified: %s", err.Error())
	}

	runRemovedFile, err := os.Open(config.RunPackagesRemovedJSON)
	if err != nil {
		log.Fatal(err)
	}
	defer runRemovedFile.Close()

	err = json.NewDecoder(runRemovedFile).Decode(&contents.RunRemoved)
	if err != nil {
		log.Fatalf("failed unmarshalling run packages removed: %s", err.Error())
	}

	if config.BuildCveReport != "" {
		buildCveReportStr, err := os.ReadFile(config.BuildCveReport)
		if err != nil {
			log.Fatalf("failed reading Build CVE report %s", err.Error())
		}
		contents.BuildCveReport = string(buildCveReportStr)
	}

	if config.RunCveReport != "" {
		runCveReportStr, err := os.ReadFile(config.RunCveReport)
		if err != nil {
			log.Fatalf("failed reading Run CVE report %s", err.Error())
		}
		contents.RunCveReport = string(runCveReportStr)
	}

	contents.BuildImage = config.BuildImage
	contents.RunImage = config.RunImage
	if config.ReceiptsShowLimit == "" {
		config.ReceiptsShowLimit = "2147483647"
	}
	contents.ReceiptsShowLimit, err = strconv.Atoi(config.ReceiptsShowLimit)

	if err != nil {
		log.Fatalf("failed converting receipts show limit string to int: %s", err.Error())
	}

	t, err := template.New("template.md").Parse(tString)
	if err != nil {
		log.Fatalf("failed to create release notes template: %s", err.Error())
	}

	b := bytes.NewBuffer(nil)
	err = t.Execute(b, contents)
	if err != nil {
		log.Fatalf("failed to execute release notes template: %s", err.Error())
	}

	fmt.Println(b.String())

	outputFileName, ok := os.LookupEnv("GITHUB_OUTPUT")
	if !ok {
		log.Fatalf("GITHUB_OUTPUT is not set, see https://docs.github.com/en/actions/using-workflows/workflow-commands-for-github-actions#setting-an-output-parameter")
	}
	file, err := os.OpenFile(outputFileName, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		log.Fatalf("failed to set output: %s", err.Error())
	}
	defer file.Close()
	delimiter := generateDelimiter()
	fmt.Fprintf(file, "release_body<<%s\n%s\n%s\n", delimiter, b.String(), delimiter) // see https://docs.github.com/en/actions/using-workflows/workflow-commands-for-github-actions#multiline-strings
}

func generateDelimiter() string {
	data := make([]byte, 16) // roughly the same entropy as uuid v4 used in https://github.com/actions/toolkit/blob/b36e70495fbee083eb20f600eafa9091d832577d/packages/core/src/file-command.ts#L28
	_, err := rand.Read(data)
	if err != nil {
		log.Fatal("could not generate random delimiter", err)
	}
	return hex.EncodeToString(data)
}

func fixEmptyArray(original string) string {
	if original == "" {
		return `[]`
	}
	return original
}
