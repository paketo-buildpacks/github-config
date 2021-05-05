package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/paketo-buildpacks/github-config/actions/pull-request/download-artifact/entrypoint/internal"
)

func main() {
	var options internal.Options

	flag.StringVar(&options.Name, "name", "", "Name of the uploaded artifact")
	flag.StringVar(&options.Repo, "repo", "", "Org and repository that the workflow lives in")
	flag.StringVar(&options.RunID, "run-id", "", "ID of the specific workflow that contains the artifact")
	flag.StringVar(&options.GithubAPI, "github-api", "", "Github API endpoint to query for the download")
	flag.StringVar(&options.Workspace, "workspace", "", "Path to the workspace to put artifacts")
	flag.Parse()

	if options.Name == "" {
		fail(errors.New(`missing required input "name"`))
	}

	if options.Repo == "" {
		fail(errors.New(`missing required input "repo"`))
	}

	if options.RunID == "" {
		fail(errors.New(`missing required input "run-id"`))
	}

	if options.Workspace == "" {
		fail(errors.New(`missing required input "workspace"`))
	}

	archiveDownloadURL, zipSize, err := internal.GetWorkflowArtifactURL(options, os.Getenv("GITHUB_TOKEN"))
	if err != nil {
		fail(err)
	}

	fmt.Println("Getting workflow artifact zip file")
	if options.GithubAPI != "https://api.github.com" {
		archiveDownloadURL = options.GithubAPI + archiveDownloadURL
	}

	payloadResponseBody, err := internal.GetArtifactZip(archiveDownloadURL, os.Getenv("GITHUB_TOKEN"))
	if err != nil {
		fail(err)
	}

	unzippedFileBytes, err := internal.UnzipPayload(options.Name, payloadResponseBody, zipSize)
	if err != nil {
		fail(err)
	}

	// Write the unzipped contents to a json file in the github.workspace
	err = os.WriteFile(filepath.Join(options.Workspace, "event.json"), unzippedFileBytes, 0600)
	if err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Printf("Error: %s", err)
	os.Exit(1)
}
