package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/paketo-buildpacks/packit/v2/cargo"
)

// Copy of cargo config structure, with the addition of the Target field
type Dependency struct {
	cargo.ConfigMetadataDependency
	Target string `toml:"target"           json:"target,omitempty"`
}

func main() {
	var config struct {
		Version string
		Target  string
		SHA256  string
		URI     string
		File    string
	}

	flag.StringVar(&config.Version, "version", "", "Dependency version")
	flag.StringVar(&config.Target, "target", "", "Dependency target name")
	flag.StringVar(&config.SHA256, "sha256", "", "Dependency SHA256 to add")
	flag.StringVar(&config.URI, "uri", "", "Dependency URI to add")
	flag.StringVar(&config.File, "file", "", "Dependency metadata.json file to modify")
	flag.Parse()

	if config.Version == "" {
		fail(errors.New(`missing required input "version"`))
	}

	if config.Target == "" {
		fail(errors.New(`missing required input "target"`))
	}
	if config.SHA256 == "" {
		fail(errors.New(`missing required input "SHA256"`))
	}
	if config.URI == "" {
		fail(errors.New(`missing required input "uri"`))
	}
	if config.File == "" {
		fail(errors.New(`missing required input "file"`))
	}

	entries := []*Dependency{}
	file, err := os.OpenFile(config.File, os.O_RDWR, os.ModePerm)
	if err != nil {
		fail(err)
	}

	err = json.NewDecoder(file).Decode(&entries)
	if err != nil {
		fail(err)
	}

	// Find the dependency of interest and update the SHA256
	found := false
	for _, dependency := range entries {
		if dependency.Target == config.Target && dependency.Version == config.Version {
			dependency.SHA256 = config.SHA256
			dependency.URI = config.URI
			found = true
		}
	}

	if !found {
		fmt.Println("No change, no matching metadata found. Exiting.")
		os.Exit(0)
	}
	// Clear file and rewrite content
	err = file.Truncate(0)
	if err != nil {
		// untested
		fail(err)
	}
	_, err = file.Seek(0, 0)
	if err != nil {
		// untested
		fail(err)
	}

	// Write it back to the file
	err = json.NewEncoder(file).Encode(entries)
	if err != nil {
		//untested
		fail(err)
	}
	defer file.Close()

	fmt.Println("Success! Updated metadata with:")
	fmt.Printf(`"sha256": "%s"\n`, config.SHA256)
	fmt.Printf(`"uri": "%s"`, config.URI)
}

func fail(err error) {
	fmt.Printf("Error: %s", err)
	os.Exit(1)
}
