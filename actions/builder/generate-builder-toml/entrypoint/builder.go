package main

import (
	"bytes"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

const (
	LifecycleRegistryServer = "index.docker.io"
	LifecycleRepoImage      = "buildpacksio/lifecycle"
)

type buildpack struct {
	ID       string `toml:"id"`
	Version  string `toml:"version"`
	Homepage string `toml:"homepage,omitempty"`
	Optional bool   `toml:"optional,omitempty"`
}

type buildpackConfig struct {
	ImageName string `toml:"image"`
	Version   string `toml:"version"`
}

type orderEntry struct {
	Group []buildpack `toml:"group"`
}

type order []orderEntry

type stackConfig struct {
	ID              string   `toml:"id"`
	BuildImage      string   `toml:"build-image"`
	RunImage        string   `toml:"run-image"`
	RunImageMirrors []string `toml:"run-image-mirrors,omitempty"`
}

type lifecycleConfig struct {
	Version string `toml:"version"`
}

type orderTOML struct {
	Description string `toml:"description,omitempty"`
	Order       order  `toml:"order"`
}

type builderTOML struct {
	Description string            `toml:"description"`
	Buildpacks  []buildpackConfig `toml:"buildpacks"`
	Lifecycle   lifecycleConfig   `toml:"lifecycle"`
	Order       order             `toml:"order"`
	Stack       stackConfig       `toml:"stack"`
}

type BuilderInfo struct {
	Stack           string
	BuildImage      string
	RunImage        string
	RunImageMirrors string
	StackImageTag   string
	OrderFile       string
	RegistryServer  string
	Toml            builderTOML
}

func (b *BuilderInfo) readOrderToml() (orderTOML, error) {
	order := orderTOML{}
	file, err := os.Open(b.OrderFile)

	if err != nil {
		return order, fmt.Errorf("invalid path to order.toml (%s): %w", b.OrderFile, err)
	}

	_, err = toml.DecodeReader(file, &order)
	if err != nil {
		return order, fmt.Errorf("invalid order toml: %w", err)
	}

	return order, nil
}

// Sets the [stack] section.
func (b *BuilderInfo) setStack() error {
	buildImage, err := imageFromFullName(b.BuildImage)
	if err != nil {
		return fmt.Errorf("invalid build image %s: %w", b.BuildImage, err)
	}

	// build-images are versioned in image tags using the syntax: <semver>-<stack-tag>
	// e.g. 0.0.94-full-cnb
	buildImageVersion, err := buildImage.getLatestVersion("-" + b.StackImageTag)
	if err != nil {
		return err
	}

	taggedBuildImage := buildImage.getFullImageName() + ":" + buildImageVersion

	runImage, err := imageFromFullName(b.RunImage)
	if err != nil {
		return fmt.Errorf("invalid run image %s: %w", b.BuildImage, err)
	}

	taggedRunImage := runImage.getFullImageName() + ":" + b.StackImageTag

	runImageMirrors := strings.Split(b.RunImageMirrors, ",")
	for i, mirror := range runImageMirrors {
		mirrorImage, err := imageFromFullName(mirror)
		if err != nil {
			return fmt.Errorf("invalid run-image mirror %s: %w", b.BuildImage, err)
		}

		runImageMirrors[i] = mirrorImage.getFullImageName() + ":" + b.StackImageTag
	}

	b.Toml.Stack = stackConfig{
		ID:              b.Stack,
		BuildImage:      taggedBuildImage,
		RunImage:        taggedRunImage,
		RunImageMirrors: runImageMirrors,
	}

	return nil
}

// Sets the [lifecycle] section.
func (b *BuilderInfo) setLifecycle() error {
	lifecycleImage := Image{
		RegistryServer: LifecycleRegistryServer,
		ImageName:      LifecycleRepoImage,
	}

	lifecycleVersion, err := lifecycleImage.getLatestVersion("")
	if err != nil {
		return err
	}

	b.Toml.Lifecycle = lifecycleConfig{
		Version: lifecycleVersion,
	}

	return nil
}

// Sets the [[buildpacks]] and [[order]] sections.
func (b *BuilderInfo) setBuildpacksAndOrder(order orderTOML) error {
	b.Toml.Description = order.Description
	b.Toml.Order = order.Order

	buildpackMap := map[string]string{}

	for _, entry := range b.Toml.Order {
		for i, buildpack := range entry.Group {
			buildpackImage := Image{
				RegistryServer: b.RegistryServer,
				ImageName:      buildpack.ID,
			}

			buildpackVersion, err := buildpackImage.getLatestVersion("")
			if err != nil {
				return err
			}

			entry.Group[i].Version = buildpackVersion
			buildpackMap[buildpack.ID] = entry.Group[i].Version
		}
	}

	for id, version := range buildpackMap {
		bpc := buildpackConfig{
			ImageName: fmt.Sprintf("%s/%s:%s", b.RegistryServer, id, version),
			Version:   version,
		}
		b.Toml.Buildpacks = append(b.Toml.Buildpacks, bpc)
	}

	// for deterministic output
	sort.Slice(b.Toml.Buildpacks, func(i, j int) bool {
		return b.Toml.Buildpacks[i].ImageName < b.Toml.Buildpacks[j].ImageName
	})

	return nil
}

// Writes the generated builder toml as github action output.
func (b *BuilderInfo) setTomlOutput(variable string) error {
	buf := new(bytes.Buffer)

	err := toml.NewEncoder(buf).Encode(b.Toml)
	if err != nil {
		return fmt.Errorf("unable to create builder toml: %w", err)
	}

	// Convert multiline output to single line output as GitHub "set-output" does
	// not support multiline strings:
	// https://github.community/t5/GitHub-Actions/set-output-Truncates-Multiline-Strings/m-p/38372#M3322
	out := buf.String()
	out = strings.TrimSpace(out)
	out = strings.ReplaceAll(out, "%", "%25")
	out = strings.ReplaceAll(out, "\n", "%0A")
	out = strings.ReplaceAll(out, "\r", "%0D")
	fmt.Printf("::set-output name=%s::%s", variable, out)

	return nil
}
