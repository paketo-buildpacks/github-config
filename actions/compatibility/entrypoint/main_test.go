package main_test

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/cloudfoundry/buildpacks-ci/tasks/cnb/helpers"
	"github.com/mitchellh/mapstructure"
	. "github.com/onsi/gomega"
	. "github.com/paketo-buildpacks/github-config/actions/compatibility/entrypoint"
	"github.com/sclevine/spec"
)

var entrypoint string

func TestSDKCompatibilityTableUpdate(t *testing.T) {
	spec.Run(t, "SDKCompatibilityTableUpdate", func(t *testing.T, context spec.G, it spec.S) {
		var (
			Expect       = NewWithT(t).Expect
			err          error
			outputDir    string
			releasesJSON string
		)

		it.Before(func() {
			RegisterTestingT(t)
			outputDir, err = os.MkdirTemp("", "output")
			Expect(err).NotTo(HaveOccurred())
			releasesJSON = filepath.Join("testdata", "releases.json")
		})

		it.After(func() {
			Expect(os.RemoveAll(outputDir)).To(Succeed())
		})

		context("with empty buildpack.toml", func() {
			it("add version of sdk dependency", func() {
				buildpackTOML := helpers.BuildpackTOML{Metadata: helpers.Metadata{}}
				runTask(buildpackTOML, releasesJSON, "2.1.803", outputDir)

				outputBuildpackToml := decodeBuildpackTOML(outputDir)

				var compatibilityTable []RuntimeToSDK
				err = mapstructure.Decode(outputBuildpackToml.Metadata["runtime-to-sdks"], &compatibilityTable)
				Expect(err).NotTo(HaveOccurred())
				Expect(compatibilityTable).To(Equal([]RuntimeToSDK{
					{
						RuntimeVersion: "2.1.15",
						SDKs:           []string{"2.1.803"},
					},
				}))

			})
		})

		context("with version that doesn't exist in buildpack.toml", func() {
			it("add version of sdk dependency and sorts versions", func() {
				buildpackTOML := helpers.BuildpackTOML{
					Metadata: helpers.Metadata{
						helpers.RuntimeToSDKsKey: []RuntimeToSDK{
							{RuntimeVersion: "2.1.14", SDKs: []string{"2.1.607"}},
						},
					},
				}

				runTask(buildpackTOML, releasesJSON, "2.1.803", outputDir)

				outputBuildpackToml := decodeBuildpackTOML(outputDir)

				var compatibilityTable []RuntimeToSDK
				err = mapstructure.Decode(outputBuildpackToml.Metadata["runtime-to-sdks"], &compatibilityTable)
				Expect(err).NotTo(HaveOccurred())

				Expect(compatibilityTable).To(Equal([]RuntimeToSDK{
					{
						RuntimeVersion: "2.1.14",
						SDKs:           []string{"2.1.607"},
					},
					{
						RuntimeVersion: "2.1.15",
						SDKs:           []string{"2.1.803"},
					},
				}))
			})

			context("the runtime version is not one of the two latest supported versions", func() {
				it("does not add to the compatibility table", func() {
					buildpackTOML := helpers.BuildpackTOML{
						Metadata: helpers.Metadata{
							helpers.DependenciesKey: []helpers.Dependency{
								{ID: "dotnet-sdk", Version: "2.1.801"},
							},
						},
					}

					taskOutput := runTask(buildpackTOML, releasesJSON, "2.1.801", outputDir)

					Expect(taskOutput).To(ContainSubstring("this runtime patch version is not supported. only the two latest versions are supported"))

					outputBuildpackToml := decodeBuildpackTOML(outputDir)

					var dependencies []helpers.Dependency

					err = mapstructure.Decode(outputBuildpackToml.Metadata["dependencies"], &dependencies)
					Expect(err).NotTo(HaveOccurred())
					Expect(dependencies).To(BeEmpty())
				})
			})
		})

		context("runtime version is present in buildpack.toml", func() {
			it("include only one latest version of sdk dependency", func() {
				buildpackTOML := helpers.BuildpackTOML{
					Metadata: helpers.Metadata{
						helpers.DependenciesKey: []helpers.Dependency{
							{ID: "dotnet-sdk", Version: "1.1.801"},
							{ID: "dotnet-sdk", Version: "2.1.606"},
							{ID: "dotnet-sdk", Version: "2.1.607"},
						},
						helpers.RuntimeToSDKsKey: []RuntimeToSDK{
							{RuntimeVersion: "1.1.13", SDKs: []string{"1.1.801"}},
							{RuntimeVersion: "2.1.14", SDKs: []string{"2.1.606"}},
						},
					},
				}

				runTask(buildpackTOML, releasesJSON, "2.1.607", outputDir)

				outputBuildpackToml := decodeBuildpackTOML(outputDir)

				var compatibilityTable []RuntimeToSDK
				err = mapstructure.Decode(outputBuildpackToml.Metadata["runtime-to-sdks"], &compatibilityTable)
				Expect(err).NotTo(HaveOccurred())
				Expect(compatibilityTable).To(Equal(
					[]RuntimeToSDK{
						{
							RuntimeVersion: "1.1.13",
							SDKs:           []string{"1.1.801"},
						},
						{
							RuntimeVersion: "2.1.14",
							SDKs:           []string{"2.1.607"},
						},
					}))

				var dependencies []helpers.Dependency
				err = mapstructure.Decode(outputBuildpackToml.Metadata["dependencies"], &dependencies)
				Expect(err).NotTo(HaveOccurred())

				Expect(dependencies).To(Equal([]helpers.Dependency{
					{ID: "dotnet-sdk", Version: "1.1.801"},
					{ID: "dotnet-sdk", Version: "2.1.607"},
				}))
			})
		})

		context("runtime version is not present in buildpack.toml", func() {
			it("include only two latest versions of runtime dependency", func() {
				buildpackTOML := helpers.BuildpackTOML{
					Metadata: helpers.Metadata{
						helpers.DependenciesKey: []helpers.Dependency{
							{ID: "dotnet-sdk", Version: "2.1.605"},
							{ID: "dotnet-sdk", Version: "2.1.606"},
							{ID: "dotnet-sdk", Version: "2.1.801"},
						},
						helpers.RuntimeToSDKsKey: []RuntimeToSDK{
							{RuntimeVersion: "2.1.13", SDKs: []string{"2.1.605"}},
							{RuntimeVersion: "2.1.14", SDKs: []string{"2.1.606"}},
						},
					},
				}

				runTask(buildpackTOML, releasesJSON, "2.1.803", outputDir)

				outputBuildpackToml := decodeBuildpackTOML(outputDir)

				var compatibilityTable []RuntimeToSDK
				err = mapstructure.Decode(outputBuildpackToml.Metadata["runtime-to-sdks"], &compatibilityTable)
				Expect(err).NotTo(HaveOccurred())

				Expect(compatibilityTable).To(Equal(
					[]RuntimeToSDK{
						{
							RuntimeVersion: "2.1.14",
							SDKs:           []string{"2.1.606"},
						},
						{
							RuntimeVersion: "2.1.15",
							SDKs:           []string{"2.1.803"},
						},
					}))

				var dependencies []helpers.Dependency
				err = mapstructure.Decode(outputBuildpackToml.Metadata["dependencies"], &dependencies)
				Expect(err).NotTo(HaveOccurred())
				Expect(dependencies).To(Equal(
					[]helpers.Dependency{
						{ID: "dotnet-sdk", Version: "2.1.606"},
						{ID: "dotnet-sdk", Version: "2.1.801"},
					}))
			})
		})

		context("dotnet runtime already has latest sdk depedency", func() {
			context("the sdk is the latest version", func() {
				it("does not update or remove from buildpack.toml", func() {
					buildpackTOML := helpers.BuildpackTOML{
						Metadata: helpers.Metadata{
							helpers.DependenciesKey: []helpers.Dependency{
								{ID: "dotnet-sdk", Version: "2.1.607"},
							},
							helpers.RuntimeToSDKsKey: []RuntimeToSDK{
								{RuntimeVersion: "2.1.14", SDKs: []string{"2.1.607"}},
							},
						},
					}

					runTask(buildpackTOML, releasesJSON, "2.1.607", outputDir)

					outputBuildpackToml := decodeBuildpackTOML(outputDir)

					var compatibilityTable []RuntimeToSDK
					err = mapstructure.Decode(outputBuildpackToml.Metadata["runtime-to-sdks"], &compatibilityTable)
					Expect(err).NotTo(HaveOccurred())
					Expect(compatibilityTable).To(Equal(
						[]RuntimeToSDK{
							{
								RuntimeVersion: "2.1.14",
								SDKs:           []string{"2.1.607"},
							},
						}))

					var dependencies []helpers.Dependency
					err = mapstructure.Decode(outputBuildpackToml.Metadata["dependencies"], &dependencies)
					Expect(err).NotTo(HaveOccurred())
					Expect(dependencies).To(Equal(
						[]helpers.Dependency{
							{ID: "dotnet-sdk", Version: "2.1.607"},
						}))
				})
			})
		})

		it("should keep the integrity of the rest of the toml", func() {
			buildpackTOML := helpers.BuildpackTOML{
				API: "0.2",
				Metadata: helpers.Metadata{
					helpers.IncludeFilesKey: []string{"bin/build", "bin/detect", "buildpack.toml", "go.mod", "go.sum"},
					helpers.PrePackageKey:   "./scripts/build.sh",
					helpers.DependenciesKey: []helpers.Dependency{
						{ID: "dotnet-sdk", Version: "2.1.607"},
						{ID: "dotnet-sdk", Version: "2.1.802"},
						{ID: "dotnet-sdk", Version: "2.1.803"},
					},
					helpers.RuntimeToSDKsKey: []RuntimeToSDK{
						{RuntimeVersion: "2.1.14", SDKs: []string{"2.1.607"}},
						{RuntimeVersion: "2.1.15", SDKs: []string{"2.1.802"}},
					},
				},
				Stacks: []helpers.Stack{
					{ID: "org.cloudfoundry.stacks.cflinuxfs3"},
					{ID: "io.buildpacks.stacks.bionic"},
				},
			}

			runTask(buildpackTOML, releasesJSON, "2.1.803", outputDir)

			outputBuildpackToml := decodeBuildpackTOML(outputDir)
			Expect("0.2").To(Equal(outputBuildpackToml.API))
			Expect("./scripts/build.sh").To(Equal(outputBuildpackToml.Metadata[helpers.PrePackageKey]))
			Expect(len(outputBuildpackToml.Stacks)).To(Equal(2))
		})

		context("failure cases", func() {
			context("the sdk version is not in the releases page", func() {
				it("errors out", func() {
					buildpackTOML := helpers.BuildpackTOML{
						Metadata: helpers.Metadata{
							helpers.DependenciesKey: []helpers.Dependency{
								{ID: "dotnet-sdk", Version: "2.1.606"},
								{ID: "dotnet-sdk", Version: "2.1.607"},
							},
							helpers.RuntimeToSDKsKey: []RuntimeToSDK{
								{RuntimeVersion: "2.1.14", SDKs: []string{"2.1.607"}},
							},
						},
					}

					_, err := runTaskError(buildpackTOML, releasesJSON, "2.1.606", outputDir)
					Expect(err).To(HaveOccurred())
				})
			})

		})
	})
}

func decodeBuildpackTOML(outputDir string) helpers.BuildpackTOML {
	var buildpackTOML helpers.BuildpackTOML
	_, err := toml.DecodeFile(filepath.Join(outputDir, "buildpack.toml"), &buildpackTOML)
	Expect(err).NotTo(HaveOccurred())
	return buildpackTOML
}

func runTask(buildpackTOML helpers.BuildpackTOML, releasesJSON, sdkVersion, outputDir string) string {
	buildpackTOMLContents := setupOutputDirectory(outputDir, buildpackTOML)

	taskCmd := exec.Command(
		"go", "run", "main.go",
		"--buildpack-toml", buildpackTOMLContents,
		"--releases-json-path", releasesJSON,
		"--sdk-version", sdkVersion,
		"--output-dir", outputDir,
	)
	taskCmd.Env = append(taskCmd.Env, "HOME="+os.Getenv("HOME"), "PATH="+os.Getenv("PATH"))

	taskOutput, err := taskCmd.CombinedOutput()
	Expect(err).NotTo(HaveOccurred())
	return string(taskOutput)
}

func runTaskError(buildpackTOML helpers.BuildpackTOML, releasesJSON, sdkVersion, outputDir string) (string, error) {
	buildpackTOMLContents := setupOutputDirectory(outputDir, buildpackTOML)

	taskCmd := exec.Command(
		"go", "run", "main.go",
		"--buildpack-toml", buildpackTOMLContents,
		"--releases-json-path", releasesJSON,
		"--sdk-version", sdkVersion,
		"--output-dir", outputDir,
	)
	taskCmd.Env = append(taskCmd.Env, "HOME="+os.Getenv("HOME"), "PATH="+os.Getenv("PATH"))

	taskOutput, err := taskCmd.CombinedOutput()
	return string(taskOutput), err
}

func setupOutputDirectory(outputDir string, buildpackTOML helpers.BuildpackTOML) string {
	Expect(os.RemoveAll(outputDir)).To(Succeed())
	Expect(os.Mkdir(outputDir, 0755)).To(Succeed())
	Expect(buildpackTOML.WriteToFile(filepath.Join(outputDir, "buildpack.toml"))).To(Succeed())

	buildpackTOMLContents, err := ioutil.ReadFile(filepath.Join(outputDir, "buildpack.toml"))
	Expect(err).NotTo(HaveOccurred())
	return string(buildpackTOMLContents)
}
