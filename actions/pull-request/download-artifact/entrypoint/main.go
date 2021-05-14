package main

import (
	"archive/zip"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type Options struct {
	Name      string
	Glob      string
	Repo      string
	RunID     string
	GithubAPI string
	Workspace string
	Token     string
}

func main() {
	var options Options

	flag.StringVar(&options.Name, "name", "", "Name of the uploaded artifact")
	flag.StringVar(&options.Glob, "glob", "*", "Name of the file of interest inside the artifact zip")
	flag.StringVar(&options.Repo, "repo", "", "Org and repository that the workflow lives in")
	flag.StringVar(&options.RunID, "run-id", "", "ID of the specific workflow that contains the artifact")
	flag.StringVar(&options.GithubAPI, "github-api", "https://api.github.com", "Github API endpoint to query for the download")
	flag.StringVar(&options.Workspace, "workspace", "", "Path to the workspace to put artifacts")
	flag.StringVar(&options.Token, "token", "", "Github Access Token used to make the request")
	flag.Parse()

	requiredFlags := map[string]string{
		"--name":      options.Name,
		"--repo":      options.Repo,
		"--run-id":    options.RunID,
		"--workspace": options.Workspace,
		"--token":     options.Token,
	}

	for name, value := range requiredFlags {
		if value == "" {
			fail(fmt.Errorf("missing required flag %s", name))
		}
	}

	url, size, err := GetWorkflowArtifactURL(options.GithubAPI, options.Repo, options.RunID, options.Token, options.Name)
	if err != nil {
		fail(err)
	}

	body, err := GetArtifactZip(url, options.Token)
	if err != nil {
		fail(err)
	}
	defer body.Close()

	err = UnzipPayload(options.Glob, options.Workspace, body, size)
	if err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Printf("Error: %s", err)
	os.Exit(1)
}

func GetWorkflowArtifactURL(api, repo, runID, token, name string) (string, int, error) {
	url := fmt.Sprintf("%s/repos/%s/actions/runs/%s/artifacts", api, repo, runID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", 0, err
	}

	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))

	fmt.Printf("Getting workflow artifacts from %s\n", url)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("failed to list artifacts: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", 0, fmt.Errorf("failed to list artifacts: status code %d", resp.StatusCode)
	}

	var body struct {
		Artifacts []struct {
			Name               string `json:"name"`
			ArchiveDownloadURL string `json:"archive_download_url"`
			SizeInBytes        int    `json:"size_in_bytes"`
		} `json:"artifacts"`
	}
	err = json.NewDecoder(resp.Body).Decode(&body)
	if err != nil {
		return "", 0, fmt.Errorf("failed to parse artifacts response: %s", err)
	}

	for _, artifact := range body.Artifacts {
		if artifact.Name == name {
			return artifact.ArchiveDownloadURL, artifact.SizeInBytes, nil
		}
	}

	return "", 0, fmt.Errorf("failed to find matching artifact")
}

func GetArtifactZip(url, token string) (io.ReadCloser, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))

	fmt.Printf("Downloading zip from %s\n", url)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get artifact zip file: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		return nil, fmt.Errorf("failed to get artifact zip file: status code %d", resp.StatusCode)
	}

	return resp.Body, nil
}

func UnzipPayload(glob, workspace string, reader io.Reader, size int) error {
	buffer, err := os.CreateTemp("", "")
	if err != nil {
		return err
	}
	defer os.Remove(buffer.Name())

	_, err = io.Copy(buffer, reader)
	if err != nil {
		return err
	}

	zr, err := zip.NewReader(buffer, int64(size))
	if err != nil {
		return err
	}

	var matches []string
	for _, file := range zr.File {
		match, err := filepath.Match(glob, file.Name)
		if err != nil {
			return fmt.Errorf("%s: %q", err, glob)
		}

		if match {
			fmt.Println("Unpacking file:", file.Name)
			matches = append(matches, file.Name)

			path := filepath.Join(workspace, file.Name)
			if !strings.HasPrefix(path, filepath.Clean(workspace)+string(os.PathSeparator)) {
				return fmt.Errorf("zipslip: illegal file path: %s", path)
			}

			fd, err := os.Create(path)
			if err != nil {
				return err
			}

			f, err := file.Open()
			if err != nil {
				return err
			}

			_, err = io.CopyN(fd, f, int64(file.UncompressedSize64))
			if err != nil {
				return err
			}

			err = f.Close()
			if err != nil {
				return err
			}

			err = fd.Close()
			if err != nil {
				return err
			}
		}
	}

	if len(matches) == 0 {
		return fmt.Errorf("failed to find any files matching %q in zip", glob)
	}

	return nil
}
