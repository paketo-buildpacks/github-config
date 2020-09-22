package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
)

func fail(err error) {
	fmt.Printf("Error: %s", err)
	os.Exit(1)
}

func main() {
	b := BuilderInfo{}

	flag.StringVar(&b.Stack, "stack", "", "ID of the stack to use for the builder")
	flag.StringVar(&b.BuildImage, "build-image", "", "Build Image")
	flag.StringVar(&b.RunImage, "run-image", "", "Run Image")
	flag.StringVar(&b.RunImageMirrors, "run-image-mirrors", "", "Run Image Mirrors")
	flag.StringVar(&b.StackImageTag, "stack-image-tag", "", "Stack Image Tag")
	flag.StringVar(&b.OrderFile, "order-file", "", "Path to the order.toml file")
	flag.StringVar(&b.RegistryServer, "registry-server", "", "Registrt server uri (e.g. gcr.io")
	flag.Parse()

	if b.Stack == "" {
		fail(errors.New(`missing required input "stack"`))
	}

	if b.BuildImage == "" {
		fail(errors.New(`missing required input "build-image"`))
	}

	if b.RunImage == "" {
		fail(errors.New(`missing required input "run-image"`))
	}

	if b.RunImageMirrors == "" {
		fail(errors.New(`missing required input "run-image-mirrors"`))
	}

	if b.StackImageTag == "" {
		fail(errors.New(`missing required input "stack-image-tag"`))
	}

	if b.OrderFile == "" {
		fail(errors.New(`missing required input "order-file"`))
	}

	if b.RegistryServer == "" {
		fail(errors.New(`missing required input "registry-server"`))
	}

	err := b.setStack()
	if err != nil {
		fail(err)
	}

	order, err := b.readOrderToml()
	if err != nil {
		fail(err)
	}

	err = b.setBuildpacksAndOrder(order)
	if err != nil {
		fail(err)
	}

	err = b.setLifecycle()
	if err != nil {
		fail(err)
	}

	err = b.setTomlOutput("builder_toml")
	if err != nil {
		fail(err)
	}
}
