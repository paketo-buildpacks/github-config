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
		BuildPackagesAddedJSON    string
		BuildPackagesModifiedJSON string
		BuildPackagesRemovedJSON  string
		RunPackagesAddedJSON      string
		RunPackagesModifiedJSON   string
		RunPackagesRemovedJSON    string
		PatchedJSON               string
	}

	flag.StringVar(&config.BuildImage, "build-image", "", "Registry location of stack build image")
	flag.StringVar(&config.RunImage, "run-image", "", "Registry location of stack run image")
	flag.StringVar(&config.PatchedJSON, "patched-usns", "", "JSON Array of patched USNs")
	flag.StringVar(&config.BuildPackagesAddedJSON, "build-added", "", "JSON Array of packages added to build image")
	flag.StringVar(&config.BuildPackagesModifiedJSON, "build-modified", "", "JSON Array of packages modified in build image")
	flag.StringVar(&config.BuildPackagesRemovedJSON, "build-removed", "", "JSON Array of packages removed in build image")
	flag.StringVar(&config.RunPackagesAddedJSON, "run-added", "", "JSON Array of packages added to run image")
	flag.StringVar(&config.RunPackagesModifiedJSON, "run-modified", "", "JSON Array of packages modified in run image")
	flag.StringVar(&config.RunPackagesRemovedJSON, "run-removed", "", "JSON Array of packages removed in run image")
	flag.Parse()

	var contents struct {
		PatchedArray  []USN
		BuildAdded    []Package
		BuildModified []ModifiedPackage
		BuildRemoved  []Package
		RunAdded      []Package
		RunModified   []ModifiedPackage
		RunRemoved    []Package
		BuildImage    string
		RunImage      string
	}

	err := json.Unmarshal([]byte(fixEmptyArray(config.PatchedJSON)), &contents.PatchedArray)
	if err != nil {
		log.Fatalf("failed unmarshalling patched USNs: %s", err.Error())
	}

	err = json.Unmarshal([]byte(fixEmptyArray(config.BuildPackagesAddedJSON)), &contents.BuildAdded)
	if err != nil {
		log.Fatalf("failed unmarshalling build packages added: %s", err.Error())
	}

	err = json.Unmarshal([]byte(fixEmptyArray(config.BuildPackagesModifiedJSON)), &contents.BuildModified)
	if err != nil {
		log.Fatalf("failed unmarshalling build packages modified: %s", err.Error())
	}

	err = json.Unmarshal([]byte(fixEmptyArray(config.BuildPackagesRemovedJSON)), &contents.BuildRemoved)
	if err != nil {
		log.Fatalf("failed unmarshalling build packages removed: %s", err.Error())
	}

	err = json.Unmarshal([]byte(fixEmptyArray(config.RunPackagesAddedJSON)), &contents.RunAdded)
	if err != nil {
		log.Fatalf("failed unmarshalling run packages added: %s", err.Error())
	}

	err = json.Unmarshal([]byte(fixEmptyArray(config.RunPackagesModifiedJSON)), &contents.RunModified)
	if err != nil {
		log.Fatalf("failed unmarshalling run packages modified: %s", err.Error())
	}

	err = json.Unmarshal([]byte(fixEmptyArray(config.RunPackagesRemovedJSON)), &contents.RunRemoved)
	if err != nil {
		log.Fatalf("failed unmarshalling run packages removed: %s", err.Error())
	}

	contents.BuildImage = config.BuildImage
	contents.RunImage = config.RunImage

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
