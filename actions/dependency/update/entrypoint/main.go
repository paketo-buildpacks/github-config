package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type Event struct {
	ClientPayload struct {
		Dependency Dependency `json:"dependency"`
		Strategy   string     `json:"strategy"`
	} `json:"client_payload"`
}

type Dependency struct {
	ID           string   `json:"id"            toml:"id"`
	SHA256       string   `json:"sha256"        toml:"sha256"`
	Source       string   `json:"source"        toml:"source"`
	SourceSHA256 string   `json:"source_sha256" toml:"source_sha256"`
	Stacks       []string `json:"stacks"        toml:"stacks"`
	URI          string   `json:"uri"           toml:"uri"`
	Version      string   `json:"version"       toml:"version"`
}

type Buildpack struct {
	API       string      `toml:"api"`
	Buildpack interface{} `toml:"buildpack"`
	Metadata  struct {
		IncludeFiles []string     `toml:"include_files"`
		Dependencies []Dependency `toml:"dependencies"`
	} `toml:"metadata"`
	Order []struct {
		Group []struct {
			ID       string `toml:"id"`
			Version  string `toml:"version"`
			Optional bool   `toml:"optional,omitempty"`
		} `toml:"group"`
	} `toml:"order"`
}

func main() {
	var workspacePath string
	flag.StringVar(&workspacePath, "workspace-path", "/github/workspace", "path to the workspace")
	flag.Parse()

	event, err := ParseEvent(os.Getenv("GITHUB_EVENT_PATH"))
	if err != nil {
		fail(err)
	}

	fmt.Println("Updating buildpack.toml")

	buildpack, err := ParseBuildpack(filepath.Join(workspacePath, "buildpack.toml"))
	if err != nil {
		fail(err)
	}

	switch event.ClientPayload.Strategy {
	case "replace":
		buildpack = Replace(buildpack, event.ClientPayload.Dependency)
	default:
		fail(fmt.Errorf("unknown update strategy %q", event.ClientPayload.Strategy))
	}

	err = RenderBuildpack(buildpack, filepath.Join(workspacePath, "buildpack.toml"))
	if err != nil {
		fail(err)
	}
}

func ParseEvent(path string) (Event, error) {
	fmt.Println("Parsing dispatch event")

	eventPayload, err := os.Open(os.Getenv("GITHUB_EVENT_PATH"))
	if err != nil {
		return Event{}, fmt.Errorf("Error: failed to read $GITHUB_EVENT_PATH: %w", err)
	}

	var event Event
	err = json.NewDecoder(eventPayload).Decode(&event)
	if err != nil {
		return Event{}, fmt.Errorf("Error: failed to decode $GITHUB_EVENT_PATH: %w", err)
	}

	fmt.Printf("  Dependency: %s\n", event.ClientPayload.Dependency.ID)
	fmt.Printf("  Strategy:   %s\n", event.ClientPayload.Strategy)

	return event, nil
}

func ParseBuildpack(path string) (Buildpack, error) {
	file, err := os.Open(path)
	if err != nil {
		return Buildpack{}, fmt.Errorf("Error: failed to read buildpack.toml: %w", err)
	}
	defer file.Close()

	var buildpack Buildpack
	_, err = toml.DecodeReader(file, &buildpack)
	if err != nil {
		return Buildpack{}, fmt.Errorf("Error: failed to decode buildpack.toml: %w", err)
	}

	return buildpack, nil
}

func RenderBuildpack(buildpack Buildpack, path string) error {
	file, err := os.OpenFile(path, os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to write buildpack.toml: %w", err)
	}
	defer file.Close()

	err = toml.NewEncoder(file).Encode(buildpack)
	if err != nil {
		return fmt.Errorf("failed to write buildpack.toml: %w", err)
	}

	return nil
}

func Replace(buildpack Buildpack, dependency Dependency) Buildpack {
	var replaced bool
	for index, dep := range buildpack.Metadata.Dependencies {
		if dep.ID == dependency.ID {
			buildpack.Metadata.Dependencies[index] = dependency
			replaced = true
		}
	}

	if !replaced {
		buildpack.Metadata.Dependencies = append(buildpack.Metadata.Dependencies, dependency)
	}

	for i, order := range buildpack.Order {
		for j, group := range order.Group {
			if group.ID == dependency.ID {
				buildpack.Order[i].Group[j].Version = dependency.Version
			}
		}
	}

	return buildpack
}

func fail(err error) {
	fmt.Printf("Error: %s", err)
	os.Exit(1)
}
