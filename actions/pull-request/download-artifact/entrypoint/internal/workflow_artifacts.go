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
	Workspace string
	Token     string
}

type ArtifactFile struct {
	Artifacts []Artifact `json:"artifacts"`
}

type Artifact struct {
	Name               string `json:"name"`
	ArchiveDownloadURL string `json:"archive_download_url"`
	Size               int    `json:"size_in_bytes"`
}

func GetWorkflowArtifactURL(options Options) (string, int, error) {
	var bearer = "Bearer " + options.Token
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

	fmt.Printf("Getting workflow artifacts from %s\n", url)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("failed making a request to get artifacts: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", 0, fmt.Errorf("failed getting workflow artifacts with status code: %d", resp.StatusCode)
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
