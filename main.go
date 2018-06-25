package main

import (
	"github.com/foxdalas/deploy-checker/pkg/cmd"
)

var AppVersion = "0.0.1"
var AppGitCommit = ""
var AppGitState = ""

func Version() string {
	version := AppVersion
	if len(AppGitCommit) > 0 {
		version += "-"
		version += AppGitCommit[0:8]
	}
	if len(AppGitState) > 0 && AppGitState != "clean" {
		version += "-"
		version += AppGitState
	}
	return version
}

func main() {
	cmd.Run(Version())
}
