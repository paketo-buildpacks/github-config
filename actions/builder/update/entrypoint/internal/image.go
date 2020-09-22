package internal

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/Masterminds/semver"
	"github.com/docker/distribution/reference"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

type ImageReference struct {
	Domain string
	Path   string
	Tag    string
}

func NewImageReference(ref string) (ImageReference, error) {
	named, err := reference.ParseNormalizedNamed(ref)
	if err != nil {
		return ImageReference{}, fmt.Errorf("failed to parse reference %q: %w", ref, err)
	}

	tag := "latest"
	if tagged, ok := named.(reference.Tagged); ok {
		tag = tagged.Tag()
	}

	return ImageReference{
		Domain: reference.Domain(named),
		Path:   reference.Path(named),
		Tag:    tag,
	}, nil
}

func (r ImageReference) Name() string {
	return r.Domain + "/" + r.Path + ":" + r.Tag
}

// Gets latest version from a registry of an image filtered by tag suffix.
func (r ImageReference) LatestVersion(tagSuffix string) (string, error) {
	registry, err := name.NewRegistry(r.Domain)
	if err != nil {
		return "", fmt.Errorf("invalid registry domain (%s): %w", r.Domain, err)
	}

	repo, err := name.NewRepository(r.Path)
	if err != nil {
		return "", fmt.Errorf("invalid repository path (%s): %w", r.Path, err)
	}

	repo.Registry = registry

	tags, err := remote.List(repo)
	if err != nil {
		return "", fmt.Errorf("unable to list image (%s) from domain (%s): %w", r.Path, r.Domain, err)
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
