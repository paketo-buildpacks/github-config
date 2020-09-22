package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/paketo-buildpacks/github-config/actions/builder/update/entrypoint/internal"
)

type Options struct {
	BuilderFile    string
	RegistryServer string
}

func main() {
	var options Options

	flag.StringVar(&options.BuilderFile, "builder-file", "", "Path to the builder.toml file")
	flag.StringVar(&options.RegistryServer, "registry-server", "", "Registry server uri (e.g. gcr.io")
	flag.Parse()

	if options.BuilderFile == "" {
		fail(errors.New(`missing required input "builder-file"`))
	}

	if options.RegistryServer == "" {
		fail(errors.New(`missing required input "registry-server"`))
	}

	builder, err := internal.ParseBuilderFile(options.BuilderFile)
	if err != nil {
		fail(err)
	}

	builder.Stack.BuildImage, err = internal.GetLatestBuildImage(builder.Stack.BuildImage, builder.Stack.RunImage)
	if err != nil {
		fail(err)
	}

	err = internal.ValidateRunImageMirrors(builder.Stack.RunImageMirrors)
	if err != nil {
		fail(err)
	}

	builder.Lifecycle, err = internal.GetLatestLifecycle()
	if err != nil {
		fail(err)
	}

	builder.Order, builder.Buildpacks, err = internal.UpdateBuildpacksAndOrder(builder.Order, options.RegistryServer)
	if err != nil {
		fail(err)
	}

	err = internal.OutputBuilder("builder_toml", builder)
	if err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Printf("Error: %s", err)
	os.Exit(1)
}
