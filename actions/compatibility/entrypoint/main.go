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

	"github.com/paketo-buildpacks/packit/cargo"

	"github.com/blang/semver"
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
	RuntimeVersion string   `toml:"runtime-version" json:"runtime-version"`
	SDKs           []string `toml:"sdks" json:"sdks"`
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

	flag.StringVar(&flags.buildpackTOML, "buildpack-toml", "", "path to input buildpack.toml file")
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
	buildpackTOML := cargo.Config{}

	file, err := os.Open(flags.buildpackTOML)
	if err != nil {
		return fmt.Errorf("failed to open buildpack.toml file: %w", err)
	}

	defer file.Close()
	if err = cargo.DecodeConfig(file, &buildpackTOML); err != nil {
		return fmt.Errorf("failed to load buildpack toml: %w", err)
	}
	supported, err := checkIfSupportedPatchVersion()
	if err != nil {
		return err
	}

	versionToRemove := flags.sdkVersion
	if supported {
		versionToRemove, buildpackTOML, err = addSDKToRuntime(buildpackTOML, flags.sdkVersion, flags.runtimeVersion)
		if err != nil {
			return fmt.Errorf("failed to add sdk to runtime mapping: %w", err)
		}
	} else {
		fmt.Println("this runtime patch version is not supported. only the two latest versions are supported")
	}

	buildpackTOML, err = removeUnusedSDK(buildpackTOML, versionToRemove)
	if err != nil {
		return fmt.Errorf("failed to removed unused sdk: %w", err)
	}

	buildpackTOMLFile, err := os.Create(filepath.Join(flags.outputDir, "buildpack.toml"))
	if err != nil {
		return fmt.Errorf("failed to open buildpack.toml at: %s", flags.outputDir)
	}
	defer buildpackTOMLFile.Close()

	if err := cargo.EncodeConfig(buildpackTOMLFile, buildpackTOML); err != nil {
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
		semverSdkVersion, err := semver.New(flags.sdkVersion)
		if err != nil {
			return Channel{}, fmt.Errorf("failed to parse SDK version into semantic version: %w", err)
		}

		url := fmt.Sprintf("https://dotnetcli.blob.core.windows.net/dotnet/release-metadata/%d.%d/releases.json", semverSdkVersion.Major, semverSdkVersion.Minor)
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

func addSDKToRuntime(buildpackTOML cargo.Config, sdkVersion, runtimeVersion string) (string, cargo.Config, error) {
	var versionToRemove string
	var inputs []RuntimeToSDK

	if buildpackTOML.Metadata.Unstructured != nil {
		err := json.Unmarshal([]byte(buildpackTOML.Metadata.Unstructured["runtime-to-sdks"].(json.RawMessage)), &inputs)
		if err != nil {
			return "cannot decode runtime to sdk keys from buildpacks TOML", buildpackTOML, err
		}
	}

	versionToRemove, inputs, err := removeUnsupportedRuntime(inputs)
	if err != nil {
		return "cannot remove unsupported runtime from buildpacks TOML", buildpackTOML, err
	}

	runtimeExists := false
	for _, runtimeToSDK := range inputs {
		if runtimeToSDK.RuntimeVersion == runtimeVersion {
			var updatedSDK string
			updatedSDK, versionToRemove, err = checkSDK(sdkVersion, runtimeToSDK.SDKs[0])
			if err != nil {
				return "", buildpackTOML, err
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

	buildpackTOML.Metadata.Unstructured = make(map[string]interface{})
	buildpackTOML.Metadata.Unstructured["runtime-to-sdks"] = inputs

	return versionToRemove, buildpackTOML, nil
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

func removeUnusedSDK(buildpackTOML cargo.Config, sdkVersion string) (cargo.Config, error) {
	var dependencies []cargo.ConfigMetadataDependency
	err := mapstructure.Decode(buildpackTOML.Metadata.Dependencies, &dependencies)
	if err != nil {
		return buildpackTOML, err
	}
	for i, dependency := range dependencies {
		if dependency.Version == sdkVersion {
			dependencies = append(dependencies[:i], dependencies[i+1:]...)
			break
		}
	}
	buildpackTOML.Metadata.Dependencies = dependencies
	return buildpackTOML, nil
}
