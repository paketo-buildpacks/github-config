package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type Package struct {
	Name         string `json:"name"`
	Version      string `json:"version"`
	Architecture string `json:"architecture"`
}

type ModifiedPackage struct {
	Name            string `json:"name"`
	PreviousVersion string `json:"previousVersion"`
	CurrentVersion  string `json:"currentVersion"`
}

func main() {
	var config struct {
		CurrentPath  string
		PreviousPath string
	}

	flag.StringVar(&config.PreviousPath,
		"previous",
		"",
		"Path to previous package receipt")

	flag.StringVar(&config.CurrentPath,
		"current",
		"",
		"Path to current package receipt")

	flag.Parse()

	if config.CurrentPath == "" || config.PreviousPath == "" {
		log.Fatal("Must provide current and previous paths")
	}

	absolute, err := filepath.Abs(config.CurrentPath)
	if err != nil {
		log.Fatalf("Failed to create absolute path for %s", config.CurrentPath)
	}
	config.CurrentPath = absolute

	absolute, err = filepath.Abs(config.PreviousPath)
	if err != nil {
		log.Fatalf("Failed to create absolute path for %s", config.PreviousPath)
	}
	config.PreviousPath = absolute

	previous, err := parsePackagesFromFile(config.PreviousPath)
	if err != nil {
		log.Fatal(err)
	}
	current, err := parsePackagesFromFile(config.CurrentPath)
	if err != nil {
		log.Fatal(err)
	}

	var added, removed []Package
	var modified []ModifiedPackage
	for prevName, prevPackage := range previous {
		if _, ok := current[prevName]; !ok {
			// package in previous but not in current
			removed = append(removed, prevPackage)
			continue
		}
		// package appears in both previous and current
		curPackage := current[prevName]
		if prevPackage.Version != curPackage.Version || prevPackage.Architecture != curPackage.Architecture {
			// package metadata has changed
			modified = append(modified, ModifiedPackage{
				Name:            curPackage.Name,
				PreviousVersion: prevPackage.Version,
				CurrentVersion:  curPackage.Version,
			})
		}
	}

	for curName, curPackage := range current {
		if _, ok := previous[curName]; !ok {
			// package appears in current, not in previous
			added = append(added, curPackage)
		}
	}

	addedJSON, err := json.Marshal(added)
	if err != nil {
		log.Fatal(err)
	}
	if len(added) == 0 {
		addedJSON = []byte(`[]`)
	}

	removedJSON, err := json.Marshal(removed)
	if err != nil {
		log.Fatal(err)
	}
	if len(removed) == 0 {
		removedJSON = []byte(`[]`)
	}

	modifiedJSON, err := json.Marshal(modified)
	if err != nil {
		log.Fatal(err)
	}
	if len(modified) == 0 {
		modifiedJSON = []byte(`[]`)
	}

	fmt.Println("Added packages:")
	for _, pkg := range added {
		fmt.Println(pkg.Name, pkg.Version)
	}
	fmt.Println("Removed packages:")
	for _, pkg := range removed {
		fmt.Println(pkg.Name, pkg.Version)
	}
	fmt.Println("Modified packages:")
	for _, pkg := range modified {
		fmt.Printf("%s %s => %s\n", pkg.Name, pkg.PreviousVersion, pkg.CurrentVersion)
	}

	fmt.Printf("::set-output name=added::%s\n", string(addedJSON))
	fmt.Printf("::set-output name=removed::%s\n", string(removedJSON))
	fmt.Printf("::set-output name=modified::%s\n", string(modifiedJSON))
}

func parsePackagesFromFile(path string) (map[string]Package, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("Failed to open %s: %w", path, err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	re := regexp.MustCompile(`\S+\s+`)
	packages := make(map[string]Package)

	for scanner.Scan() {
		if !strings.HasPrefix(scanner.Text(), "ii") {
			continue
		}
		matches := re.FindAllString(scanner.Text(), -1)
		if len(matches) < 4 {
			fmt.Println(scanner.Text())
			return nil, fmt.Errorf("failed to parse line in %s", path)
		}
		name := strings.Split(strings.TrimSpace(matches[1]), ":")[0]
		packages[name] = Package{
			Name:         name,
			Version:      strings.TrimSpace(matches[2]),
			Architecture: strings.TrimSpace(matches[3]),
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return packages, nil
}
