package internal

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

type Buildpack struct {
	ID       string `toml:"id"`
	Version  string `toml:"version"`
	Homepage string `toml:"homepage,omitempty"`
	Optional bool   `toml:"optional,omitempty"`
}

type ImageConfig struct {
	Image   string `toml:"image"`
	Version string `toml:"version"`
}

type Order struct {
	Group []Buildpack `toml:"group"`
}

type stackConfig struct {
	ID              string   `toml:"id"`
	BuildImage      string   `toml:"build-image"`
	RunImage        string   `toml:"run-image"`
	RunImageMirrors []string `toml:"run-image-mirrors,omitempty"`
}

type Lifecycle struct {
	Version string `toml:"version"`
}

type Builder struct {
	Description string        `toml:"description"`
	Buildpacks  []ImageConfig `toml:"buildpacks"`
	Lifecycle   Lifecycle     `toml:"lifecycle"`
	Order       []Order       `toml:"order"`
	Stack       stackConfig   `toml:"stack"`
}

func ParseBuilderFile(path string) (Builder, error) {
	var builder Builder

	file, err := os.Open(path)
	if err != nil {
		return Builder{}, fmt.Errorf("invalid path to builder.toml (%s): %w", path, err)
	}

	_, err = toml.DecodeReader(file, &builder)
	if err != nil {
		return Builder{}, fmt.Errorf("invalid builder toml: %w", err)
	}

	return builder, nil
}

func ValidateRunImage(ref string) error {
	_, err := NewImageReference(ref)
	if err != nil {
		return fmt.Errorf("invalid run image reference %q: %w", ref, err)
	}

	return nil
}

func GetLatestBuildImage(buildImageRef, runImageRef string) (string, error) {
	runImage, err := NewImageReference(runImageRef)
	if err != nil {
		return "", fmt.Errorf("invalid run image reference %q: %w", runImageRef, err)
	}

	buildImage, err := NewImageReference(buildImageRef)
	if err != nil {
		return "", fmt.Errorf("invalid build image reference %q: %w", buildImageRef, err)
	}

	// build-images are versioned in image tags using the syntax: <semver>-<stack-tag>
	// e.g. 0.0.94-full-cnb
	buildImage.Tag, err = buildImage.LatestVersion("-" + runImage.Tag)
	if err != nil {
		return "", err
	}

	return buildImage.Name(), nil
}

func ValidateRunImageMirrors(mirrors []string) error {
	for _, mirror := range mirrors {
		_, err := NewImageReference(mirror)
		if err != nil {
			return fmt.Errorf("invalid run-image mirror %q: %w", mirror, err)
		}
	}

	return nil
}

func GetLatestLifecycle() (Lifecycle, error) {
	lifecycleImage := ImageReference{
		Domain: LifecycleRegistryServer,
		Path:   LifecycleRepoImage,
	}

	version, err := lifecycleImage.LatestVersion("")
	if err != nil {
		return Lifecycle{}, err
	}

	return Lifecycle{Version: version}, nil
}

func UpdateBuildpacksAndOrder(orders []Order, server string) ([]Order, []ImageConfig, error) {
	buildpackMap := map[string]string{}

	for _, order := range orders {
		for i, buildpack := range order.Group {
			version, err := ImageReference{Domain: server, Path: buildpack.ID}.LatestVersion("")
			if err != nil {
				return nil, nil, err
			}

			order.Group[i].Version = version
			buildpackMap[buildpack.ID] = version
		}
	}

	var buildpacks []ImageConfig

	for id, version := range buildpackMap {
		buildpacks = append(buildpacks, ImageConfig{
			Image:   fmt.Sprintf("%s/%s:%s", server, id, version),
			Version: version,
		})
	}

	// for deterministic output
	sort.Slice(buildpacks, func(i, j int) bool {
		return buildpacks[i].Image < buildpacks[j].Image
	})

	return orders, buildpacks, nil
}

func OutputBuilder(variable string, builder Builder) error {
	buf := new(bytes.Buffer)

	err := toml.NewEncoder(buf).Encode(builder)
	if err != nil {
		return fmt.Errorf("unable to create builder toml: %w", err)
	}

	// Convert multiline output to single line output as GitHub "set-output" does
	// not support multiline strings:
	// https://github.community/t5/GitHub-Actions/set-output-Truncates-Multiline-Strings/m-p/38372#M3322
	out := strings.TrimSpace(buf.String())
	out = strings.ReplaceAll(out, "%", "%25")
	out = strings.ReplaceAll(out, "\n", "%0A")
	out = strings.ReplaceAll(out, "\r", "%0D")
	fmt.Printf("::set-output name=%s::%s", variable, out)

	return nil
}
