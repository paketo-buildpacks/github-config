package main

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/Masterminds/semver"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

type Image struct {
	RegistryServer string
	ImageName      string
}

func imageFromFullName(fullImageName string) (image Image, err error) {
	parts := strings.Split(fullImageName, ":")
	// Verify that we aren't confusing a tag for a hostname w/ port
	if len(parts) > 1 && !strings.Contains(parts[len(parts)-1], "/") {
		return Image{}, errors.New("image should not contain tag")
	}

	parts = strings.SplitN(fullImageName, "/", 2)
	if len(parts) != 2 {
		return Image{}, errors.New("image should be in form <registry>/<repo>/<image>")
	}

	return Image{
		RegistryServer: parts[0],
		ImageName:      parts[1],
	}, nil
}

func (i *Image) getFullImageName() string {
	return i.RegistryServer + "/" + i.ImageName
}

// Gets latest version from a registry of an image filtered by tag suffix.
func (i *Image) getLatestVersion(tagSuffix string) (string, error) {
	registry, err := name.NewRegistry(i.RegistryServer)
	if err != nil {
		return "", fmt.Errorf("invalid registry server (%s): %w", i.RegistryServer, err)
	}

	repo, err := name.NewRepository(i.ImageName)
	if err != nil {
		return "", fmt.Errorf("invalid repository name (%s): %w", i.ImageName, err)
	}

	repo.Registry = registry

	tags, err := remote.List(repo)
	if err != nil {
		return "", fmt.Errorf("unable to list images from registry server (%s): %w", i.RegistryServer, err)
	}

	versions := []*semver.Version{}
	semverPattern := regexp.MustCompile(`\d+\.\d+\.\d+`)

	for _, tag := range tags {
		if tagSuffix != "" && !strings.HasSuffix(tag, tagSuffix) {
			continue
		}

		// strip the suffix from tag for semver parsing
		tag = strings.TrimSuffix(tag, tagSuffix)

		if !semverPattern.MatchString(tag) {
			break
		}

		v, err := semver.NewVersion(tag)
		if err != nil {
			return "", fmt.Errorf("invalid semver version (%s): %w", tag, err)
		}

		versions = append(versions, v)
	}

	sort.Sort(semver.Collection(versions))

	return versions[len(versions)-1].String() + tagSuffix, nil
}
