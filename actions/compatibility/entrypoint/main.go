package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"

	msemver "github.com/Masterminds/semver"
	"github.com/blang/semver"
	"github.com/cloudfoundry/buildpacks-ci/tasks/cnb/helpers"
	"github.com/mitchellh/mapstructure"
	_ "github.com/pkg/errors"
)

var (
	flags struct {
		buildpackTOML    string
		runtimeVersion   string
		outputDir        string
		sdkVersion       string
		releasesJSONPath string
	}
	channel Channel
)

type RuntimeToSDK struct {
	RuntimeVersion string   `toml:"runtime-version" mapstructure:"runtime-version"`
	SDKs           []string `toml:"sdks"`
}

type Channel struct {
	Releases       []Release `json:"releases"`
	LatestRuntime  string    `json:"latest-runtime"`
	ChannelVersion string    `json:"channel-version"`
}

type Release struct {
	Runtime struct {
		Version string `json:"version"`
	} `json:"runtime,omitempty"`
	Sdk struct {
		Version string `json:"version"`
	} `json:"sdk,omitempty"`
}

func main() {
	var err error

	flag.StringVar(&flags.buildpackTOML, "buildpack-toml", "", "contents of buildpack.toml")
	flag.StringVar(&flags.outputDir, "output-dir", "", "directory to write buildpack.toml to")
	flag.StringVar(&flags.sdkVersion, "sdk-version", "", "version of sdk")
	flag.StringVar(&flags.releasesJSONPath, "releases-json-path", "", "path to dotnet releases.json")
	flag.Parse()

	channel, err = getReleaseChannel()
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

	flags.runtimeVersion, err = getRuntimeVersion()
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

	err = updateCompatibilityTable()
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
}

func updateCompatibilityTable() error {
	buildpackTOML := helpers.BuildpackTOML{}
	if _, err := toml.Decode(flags.buildpackTOML, &buildpackTOML); err != nil {
		return fmt.Errorf("failed to load buildpack toml: %w", err)
	}

	supported, err := checkIfSupportedPatchVersion()
	if err != nil {
		return err
	}

	versionToRemove := flags.sdkVersion
	if supported {
		versionToRemove, err = addSDKToRuntime(buildpackTOML, flags.sdkVersion, flags.runtimeVersion)
		if err != nil {
			return fmt.Errorf("failed to add sdk to runtime mapping: %w", err)
		}
	} else {
		fmt.Println("this runtime patch version is not supported. only the two latest versions are supported")
	}

	if err := removeUnusedSDK(buildpackTOML, versionToRemove); err != nil {
		return fmt.Errorf("failed to removed unused sdk: %w", err)
	}
	if err := buildpackTOML.WriteToFile(filepath.Join(flags.outputDir, "buildpack.toml")); err != nil {
		return fmt.Errorf("failed to update buildpack toml: %w", err)
	}

	return nil
}

func checkIfSupportedPatchVersion() (bool, error) {
	latestRuntime, secondLatestRuntime, err := getLatestTwoSupportedRuntimeVersions()
	if err != nil {
		return false, fmt.Errorf("failed to get two supported versions of runtime: %w", err)
	}

	return flags.runtimeVersion == latestRuntime || flags.runtimeVersion == secondLatestRuntime, nil
}

func getLatestTwoSupportedRuntimeVersions() (string, string, error) {
	latestRuntime := channel.LatestRuntime
	secondLatestRuntime := ""
	for _, release := range channel.Releases {
		if release.Runtime.Version != latestRuntime {
			secondLatestRuntime = release.Runtime.Version
			break
		}
	}
	return latestRuntime, secondLatestRuntime, nil
}

func getRuntimeVersion() (string, error) {
	for _, release := range channel.Releases {
		if release.Sdk.Version == flags.sdkVersion {
			return release.Runtime.Version, nil
		}
	}
	return "", fmt.Errorf("failed to get SDK %s compatible runtime from %s", flags.sdkVersion, flags.releasesJSONPath)
}

func getReleaseChannel() (Channel, error) {
	var releasesJSON []byte
	var err error

	// if the explicit release JSON file isn't passed in, look up the version-specific JSON file from the official release.json URI
	if flags.releasesJSONPath == "" {
		semverSdkVersion, err := msemver.NewVersion(flags.sdkVersion)
		if err != nil {
			return Channel{}, fmt.Errorf("failed to parse SDK version into semantic version: %w", err)
		}

		url := fmt.Sprintf("https://dotnetcli.blob.core.windows.net/dotnet/release-metadata/%d.%d/releases.json", semverSdkVersion.Major(), semverSdkVersion.Minor())
		fmt.Printf("getting releases from %s", url)
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return Channel{}, fmt.Errorf("failed to create request to %s: %w", url, err)
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return Channel{}, fmt.Errorf("failed to reach out to %s: %w", url, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return Channel{}, err
		}

		releasesJSON, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			return Channel{}, fmt.Errorf("failed to read releases.json: %w", err)
		}
	} else {
		releasesJSON, err = ioutil.ReadFile(flags.releasesJSONPath)
		if err != nil {
			return Channel{}, fmt.Errorf("failed to read releases.json: %w", err)
		}
	}

	if err := json.Unmarshal(releasesJSON, &channel); err != nil {
		return Channel{}, fmt.Errorf("failed to unmarshal releases.json: %w", err)
	}
	return channel, nil
}

func removeUnsupportedRuntime(inputs []RuntimeToSDK) (string, []RuntimeToSDK, error) {

	latestRuntime, secondLatestRuntime, err := getLatestTwoSupportedRuntimeVersions()
	if err != nil {
		return "", nil, fmt.Errorf("failed to get two supported versions of runtime: %w", err)
	}

	channelVersion := channel.ChannelVersion
	var inputWithoutUnsupportedRuntime []RuntimeToSDK
	var sdkVersionToRemove string

	for _, runtimeToSDK := range inputs {
		if isSupportedRuntime(runtimeToSDK, channelVersion, latestRuntime, secondLatestRuntime) {
			inputWithoutUnsupportedRuntime = append(inputWithoutUnsupportedRuntime, runtimeToSDK)
		} else {
			sdkVersionToRemove = runtimeToSDK.SDKs[0]
		}

	}

	return sdkVersionToRemove, inputWithoutUnsupportedRuntime, nil
}

func isSupportedRuntime(runtimeToSDK RuntimeToSDK, channelVersion string, latestRuntime string, secondLatestRuntime string) bool {
	return !(strings.Contains(runtimeToSDK.RuntimeVersion, channelVersion) &&
		runtimeToSDK.RuntimeVersion != latestRuntime &&
		runtimeToSDK.RuntimeVersion != secondLatestRuntime)
}

func addSDKToRuntime(buildpackTOML helpers.BuildpackTOML, sdkVersion, runtimeVersion string) (string, error) {
	var versionToRemove string
	var inputs []RuntimeToSDK

	err := mapstructure.Decode(buildpackTOML.Metadata[helpers.RuntimeToSDKsKey], &inputs)
	if err != nil {
		return "cannot decode runtime to sdk keys from buildpacks TOML", err
	}

	versionToRemove, inputs, err = removeUnsupportedRuntime(inputs)
	if err != nil {
		return "cannot remove unsupported runtime from buildpacks TOML", err
	}

	runtimeExists := false
	for _, runtimeToSDK := range inputs {
		if runtimeToSDK.RuntimeVersion == runtimeVersion {
			var updatedSDK string
			updatedSDK, versionToRemove, err = checkSDK(sdkVersion, runtimeToSDK.SDKs[0])
			if err != nil {
				return "", err
			}
			runtimeToSDK.SDKs[0] = updatedSDK
			runtimeExists = true
			break
		}
	}

	if !runtimeExists {
		inputs = append(inputs, RuntimeToSDK{
			RuntimeVersion: runtimeVersion,
			SDKs:           []string{sdkVersion},
		})
	}
	sort.Slice(inputs, func(i, j int) bool {
		firstRuntime := semver.MustParse(inputs[i].RuntimeVersion)
		secondRuntime := semver.MustParse(inputs[j].RuntimeVersion)
		return firstRuntime.LT(secondRuntime)
	})

	buildpackTOML.Metadata[helpers.RuntimeToSDKsKey] = inputs
	return versionToRemove, nil
}

func checkSDK(callingSDK, existingSDK string) (string, string, error) {
	var versionToRemove string
	updatedSDK := existingSDK

	currentSdkVersion, err := semver.New(existingSDK)
	if err != nil {
		return "", "", err
	}
	newSdkVersion, err := semver.New(callingSDK)
	if err != nil {
		return "", "", err
	}

	if newSdkVersion.GT(*currentSdkVersion) {
		versionToRemove = existingSDK
		updatedSDK = callingSDK
	} else if newSdkVersion.LT(*currentSdkVersion) {
		versionToRemove = callingSDK
	}
	return updatedSDK, versionToRemove, nil
}

func removeUnusedSDK(buildpackTOML helpers.BuildpackTOML, sdkVersion string) error {
	var dependencies []helpers.Dependency
	err := mapstructure.Decode(buildpackTOML.Metadata[helpers.DependenciesKey], &dependencies)
	if err != nil {
		return err
	}
	for i, dependency := range dependencies {
		if dependency.Version == sdkVersion {
			dependencies = append(dependencies[:i], dependencies[i+1:]...)
			break
		}
	}
	buildpackTOML.Metadata[helpers.DependenciesKey] = dependencies
	return nil
}
