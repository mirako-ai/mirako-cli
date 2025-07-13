package main

import (
	"os"

	"github.com/mirako-ai/mirako-cli/pkg/cmd/root"
)

func main() {
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}