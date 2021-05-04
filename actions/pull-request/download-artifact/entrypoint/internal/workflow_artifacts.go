package internal

import (
	"encoding/json"
	"fmt"
	"net/http"
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

func GetWorkflowArtifactURL(options Options, token string) (string, int, error) {
	var bearer = "Bearer " + token
	var url = fmt.Sprintf("%s/repos/%s/actions/runs/%s/artifacts",
		options.GithubAPI,
		options.Repo,
		options.RunID,
	)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", 0, err
	}
	req.Header.Add("Authorization", bearer)

	fmt.Println("Getting workflow artifacts")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("failed to get workflow artifacts: %w", err)
	}
	defer resp.Body.Close()

	var artifactFile ArtifactFile
	_ = json.NewDecoder(resp.Body).Decode(&artifactFile)

	if len(artifactFile.Artifacts) == 0 {
		return "", 0, fmt.Errorf("no workflow artifact found")
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
			return "", 0, fmt.Errorf("no exact workflow artifact found")
		}
	}
	return archiveDownloadURL, zipSize, err
}
