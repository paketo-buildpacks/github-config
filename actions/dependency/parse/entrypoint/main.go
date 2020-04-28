package main

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type Outputs struct {
	ID           string
	SHA256       string
	Source       string
	SourceSHA256 string
	Stacks       []string
	URI          string
	Version      string
}

type Buildpack struct {
	Buildpack struct {
		ID      string `toml:"id"`
		Version string `toml:"version"`
	} `toml:"buildpack"`
	Stacks []struct {
		ID string `toml:"id"`
	} `toml:"stacks"`
}

func main() {
	var config struct {
		GithubURI string
	}

	flag.StringVar(&config.GithubURI, "github-uri", "https://github.com", "Specifies Github base URI")
	flag.Parse()

	var (
		outputs Outputs
		err     error
	)

	outputs.URI, outputs.Source, err = ParseReleaseEvent(config.GithubURI)
	if err != nil {
		fail(err)
	}

	var buildpack Buildpack
	outputs.SHA256, outputs.SourceSHA256, buildpack, err = DownloadAssets(outputs.URI, outputs.Source)
	if err != nil {
		fail(err)
	}

	outputs.ID = buildpack.Buildpack.ID
	outputs.Version = buildpack.Buildpack.Version

	stacks := []string{}
	for _, stack := range buildpack.Stacks {
		stacks = append(stacks, stack.ID)
	}
	outputs.Stacks = stacks

	err = SetOutputs(outputs)
	if err != nil {
		fail(err)
	}
}

func ParseReleaseEvent(githubURL string) (string, string, error) {
	fmt.Println("Parsing release event")

	file, err := os.Open(os.Getenv("GITHUB_EVENT_PATH"))
	if err != nil {
		return "", "", fmt.Errorf("failed to read $GITHUB_EVENT_PATH: %w", err)
	}
	var event struct {
		Repository struct {
			FullName string `json:"full_name"`
		} `json:"repository"`
		Release struct {
			Assets []struct {
				BrowserDownloadURL string `json:"browser_download_url"`
			} `json:"assets"`
			Name    string `json:"name"`
			TagName string `json:"tag_name"`
		} `json:"release"`
	}
	err = json.NewDecoder(file).Decode(&event)
	if err != nil {
		return "", "", fmt.Errorf("failed to decode $GITHUB_EVENT_PATH: %w", err)
	}

	fmt.Printf("  Repository: %q\n", event.Repository.FullName)
	fmt.Printf("  Release:    %q\n", event.Release.Name)
	fmt.Printf("  Tag:        %q\n", event.Release.TagName)

	releaseURL := event.Release.Assets[0].BrowserDownloadURL
	sourceURL := fmt.Sprintf("%s/%s/archive/%s.tar.gz", githubURL, event.Repository.FullName, event.Release.TagName)
	return releaseURL, sourceURL, nil
}

func DownloadAssets(releaseURL, sourceURL string) (string, string, Buildpack, error) {
	fmt.Println("Downloading assets")

	fmt.Printf("  Release: %q\n", releaseURL)
	body, err := Download(releaseURL)
	if err != nil {
		return "", "", Buildpack{}, err
	}
	defer body.Close()

	file, err := ioutil.TempFile("", "source")
	if err != nil {
		return "", "", Buildpack{}, err
	}
	defer file.Close()

	_, err = io.Copy(file, body)
	if err != nil {
		return "", "", Buildpack{}, err
	}

	_, err = file.Seek(0, 0)
	if err != nil {
		return "", "", Buildpack{}, err
	}

	sha256, err := Sum(file)
	if err != nil {
		return "", "", Buildpack{}, err
	}

	_, err = file.Seek(0, 0)
	if err != nil {
		return "", "", Buildpack{}, err
	}

	buildpack, err := ParseBuildpackTOML(file)
	if err != nil {
		return "", "", Buildpack{}, err
	}

	fmt.Printf("  Source:  %q\n", sourceURL)
	sourceSHA256, err := DownloadAndSum(sourceURL)
	if err != nil {
		return "", "", Buildpack{}, err
	}

	return sha256, sourceSHA256, buildpack, nil
}

func DownloadAndSum(url string) (string, error) {
	body, err := Download(url)
	if err != nil {
		return "", fmt.Errorf("failed to download asset: %w", err)
	}
	defer body.Close()

	return Sum(body)
}

func Download(url string) (io.ReadCloser, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to download asset: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to download asset: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to download asset: unexpected response %s", resp.Status)
	}

	return resp.Body, nil
}

func Sum(reader io.Reader) (string, error) {
	hash := sha256.New()
	_, err := io.Copy(hash, reader)
	if err != nil {
		return "", fmt.Errorf("failed to calculate asset checksum: %w", err)
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

func ParseBuildpackTOML(reader io.Reader) (Buildpack, error) {
	zr, err := gzip.NewReader(reader)
	if err != nil {
		return Buildpack{}, fmt.Errorf("failed to read asset: %w", err)
	}
	defer zr.Close()

	tr := tar.NewReader(zr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return Buildpack{}, fmt.Errorf("failed to read asset: %w", err)
		}

		if filepath.Base(hdr.Name) == "buildpack.toml" {
			var buildpack Buildpack
			_, err = toml.DecodeReader(tr, &buildpack)
			if err != nil {
				return Buildpack{}, fmt.Errorf("failed to read buildpack.toml: %w", err)
			}

			return buildpack, nil
		}
	}

	return Buildpack{}, nil
}

func SetOutputs(outputs Outputs) error {
	fmt.Println("Setting outputs")

	stacks, err := json.Marshal(outputs.Stacks)
	if err != nil {
		return err
	}

	fmt.Printf("::set-output name=id::%s\n", outputs.ID)
	fmt.Printf("::set-output name=sha256::%s\n", outputs.SHA256)
	fmt.Printf("::set-output name=source::%s\n", outputs.Source)
	fmt.Printf("::set-output name=source_sha256::%s\n", outputs.SourceSHA256)
	fmt.Printf("::set-output name=stacks::%s\n", stacks)
	fmt.Printf("::set-output name=uri::%s\n", outputs.URI)
	fmt.Printf("::set-output name=version::%s\n", outputs.Version)

	return nil
}

func fail(err error) {
	fmt.Printf("Error: %s", err)
	os.Exit(1)
}
