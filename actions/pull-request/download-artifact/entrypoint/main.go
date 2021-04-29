package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
)

type Options struct {
	Name      string
	Repo      string
	RunID     string
	GithubAPI string
}

type ArtifactFile struct {
	Artifacts []Artifact `json:"artifacts"`
}

type Artifact struct {
	Name               string `json:"name"`
	ArchiveDownloadURL string `json:"archive_download_url"`
	Size               int    `json:"size_in_bytes"`
}

func main() {
	var options Options

	flag.StringVar(&options.Name, "name", "", "Name of the uploaded artifact")
	flag.StringVar(&options.Repo, "repo", "", "Org and repository that the workflow lives in")
	flag.StringVar(&options.RunID, "runID", "", "ID of the specific workflow that contains the artifact")
	flag.StringVar(&options.GithubAPI, "githubAPI", "", "Github API endpoint to query for the download")
	flag.Parse()

	if options.Name == "" {
		fail(errors.New(`missing required input "name"`))
	}

	if options.Repo == "" {
		fail(errors.New(`missing required input "repo"`))
	}

	if options.RunID == "" {
		fail(errors.New(`missing required input "runID"`))
	}

	fmt.Println("Getting workflow artifacts")
	resp, err := http.Get(
		fmt.Sprintf("%s/repos/%s/actions/runs/%s/artifacts",
			options.GithubAPI,
			options.Repo,
			options.RunID,
		))
	if err != nil {
		fail(fmt.Errorf("failed to get workflow artifacts: %w", err))
	}
	defer resp.Body.Close()

	var artifactFile ArtifactFile
	_ = json.NewDecoder(resp.Body).Decode(&artifactFile)

	if len(artifactFile.Artifacts) == 0 {
		fail(fmt.Errorf("no workflow artifact found"))
	}

	archiveDownloadURL := ""
	zipSize := 0

	for i, artifact := range artifactFile.Artifacts {
		if artifact.Name == options.Name {
			archiveDownloadURL = artifact.ArchiveDownloadURL
			zipSize = artifact.Size
			break
		}
		if i == len(artifactFile.Artifacts)-1 {
			fail(fmt.Errorf("no exact workflow artifact found"))
		}
	}

	fmt.Println("Getting workflow artifact zip file")
	if options.GithubAPI != "https://api.github.com" {
		archiveDownloadURL = options.GithubAPI + archiveDownloadURL
	}

	payloadResp, err := http.Get(archiveDownloadURL)
	if err != nil {
		fail(fmt.Errorf("failed to get artifact zip file: %w", err))
	}
	defer payloadResp.Body.Close()
	payloadBody, _ := io.ReadAll(payloadResp.Body)

	zipReader, err := zip.NewReader(bytes.NewReader(payloadBody), int64(zipSize))
	if err != nil {
		fail(err)
	}

	for _, zipFile := range zipReader.File {
		fmt.Println("Reading file:", zipFile.Name)

		if zipFile.Name == options.Name {
			unzippedFileBytes, err := readZipFile(zipFile)
			if err != nil {
				fail(err)
			}

			if workspace, ok := os.LookupEnv("GITHUB_WORKSPACE"); ok {
				err = os.WriteFile(filepath.Join(workspace, "event.json"), unzippedFileBytes, 0600)
				if err != nil {
					fail(err)
				}
			}
			break
		}
	}
}

func fail(err error) {
	fmt.Printf("Error: %s", err)
	os.Exit(1)
}

func readZipFile(zf *zip.File) ([]byte, error) {
	f, err := zf.Open()
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return ioutil.ReadAll(f)
}
