package utils

import (
	"fmt"
	"os/exec"
	"runtime"
	"syscall"
)

func OpenURLAndForget(url string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "linux":
		cmd = exec.Command("xdg-open", url)
		cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	case "darwin":
		cmd = exec.Command("open", url)
		// detach so it keeps running after this process exits:
		cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	// Currently windows is not supported, but you can add it if needed
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	// start and forget
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("Failed to launch browser")
	}

	return nil
}
