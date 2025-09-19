//go:build windows

package utils

import (
	"fmt"
	"os/exec"
	"runtime"
)

func OpenURLAndForget(url string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	// Currently only windows is supported in this file
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	// start and forget
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("Failed to launch browser")
	}

	return nil
}
