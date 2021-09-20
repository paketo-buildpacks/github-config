## .NET Core SDK Compatibility Table Update Action

This action updates the .NET Core SDK buildpack.toml file's runtime compatibility table. 

The `entrypoint/main.go` and tests are mostly copied from a previous iteration of this automation that was in the [update-dotnet-sdks-and-compat-table Concourse task](https://github.com/cloudfoundry/buildpacks-ci/tree/master/tasks/cnb/update-dotnet-sdks-and-compat-table). It has been converted into a Github Action.

To run the code locally:

1. `cd github-config/actions/compatibility/entrypoint`
2. Run:
```
go run main.go --buildpack-toml <path to your buildpack.toml> --sdk-version <new-sdk-version to add> --output-dir <directory to output new buildpack.toml>
```
And you may pass an optional `--releases-json-path" flag with a path to a local .NET Core releases.json file that specifies the SDK to runtime compatibilities. If this flag is not passed, the default .NET Core path will be used for the given SDK version line.

3. Check out the new buildpack.toml

In keeping with the logic of the old code, the passed in SDK version must be one of the two newest SDK versions available for the given version line.
