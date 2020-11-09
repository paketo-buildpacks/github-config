package internal_test

import (
	"testing"

	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"
)

func TestInternal(t *testing.T) {
	suite := spec.New("scripts/time-to-merge/internal", spec.Report(report.Terminal{}))
	suite("PullRequest", testPullRequest)
	suite.Run(t)
}
