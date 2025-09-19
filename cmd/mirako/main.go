package main

import (
	"os"

	"github.com/mirako-ai/mirako-cli/pkg/cmd/root"
)

// These variables will be set by Goreleaser ldflags during build
var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
