package internal_test

import (
	"testing"

	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"
)

func TestInternal(t *testing.T) {
	suite := spec.New("actions/builder/update/entrypoint/internal", spec.Report(report.Terminal{}))
	suite("Image", testImage)
	suite.Run(t)
}
