package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/paketo-buildpacks/packit/cargo"
)

type ImplementationBP struct {
	ID      string
	Version string
	URI     string
}

func main() {
	var workspacePath string

	flag.StringVar(&workspacePath, "workspace-path", "/github/workspace", "path to the workspace")
	flag.Parse()

	deps, err := Parse(filepath.Join(workspacePath, "buildpack.toml"))
	if err != nil {
		fail(err)
	}

	output, err := CreateReleaseBody(deps)
	if err != nil {
		fail(err)
	}

	SetOutputs(output)
}

func Parse(buildpackTOMLPath string) ([]ImplementationBP, error) {
	bpParser := cargo.NewBuildpackParser()

	config, err := bpParser.Parse(buildpackTOMLPath)
	if err != nil {
		return []ImplementationBP{}, fmt.Errorf("failed to parse buildpack.toml: %w", err)
	}

	buildpacks := []ImplementationBP{}
	for _, dependency := range config.Metadata.Dependencies {

		buildpacks = append(buildpacks, ImplementationBP{

			ID:      dependency.ID,
			Version: dependency.Version,
			URI:     dependency.URI,
		})
	}

	return buildpacks, nil
}

func CreateReleaseBody(dependencies []ImplementationBP) (string, error) {

	var outputs []string

	outputs = append(outputs, "This buildpack contains the following dependencies")
	for _, dependency := range dependencies {
		var output bytes.Buffer
		tarballPath, err := Download(dependency.URI)
		if err != nil {
			return "", fmt.Errorf("Error: failed to download tarball: %w", err)
		}

		jamCommand := exec.Command("jam", "summarize", "--buildpack", tarballPath)
		jamCommand.Stdout = &output

		err = jamCommand.Run()

		if err != nil {
			return "", fmt.Errorf("Error: failed to run 'jam summarize': %w", err)

		}

		outputs = append(outputs, fmt.Sprintf("#### %s version %s", dependency.ID, dependency.Version))
		outputs = append(outputs, output.String())

	}

	return strings.Join(outputs, "\n"), nil
}

func Download(url string) (string, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to download asset: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to download asset: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to download asset: unexpected response %s", resp.Status)
	}

	file, err := ioutil.TempFile("", "buildpack.tgz")
	if err != nil {
		return "", err
	}

	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return "", err
	}

	return file.Name(), nil
}

func SetOutputs(releaseBody string) {
	fmt.Println("Setting outputs")
	// See
	// https://github.community/t5/GitHub-Actions/set-output-Truncates-Multiline-Strings/m-p/38372#M3322
	releaseBody = strings.ReplaceAll(releaseBody, "%", `%25`)
	releaseBody = strings.ReplaceAll(releaseBody, "\n", `%0A`)
	releaseBody = strings.ReplaceAll(releaseBody, "\r", `%0D`)
	releaseBody = fmt.Sprintf("::set-output name=release_body::%s", releaseBody)
	fmt.Println(releaseBody)
}

func fail(err error) {
	fmt.Printf("Error: %s", err)
	os.Exit(1)
}
