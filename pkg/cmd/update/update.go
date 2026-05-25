package update

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/mirako-ai/mirako-cli/internal/updater"
	"github.com/spf13/cobra"
)

const updateTimeout = 5 * time.Minute

// NewUpdateCmd creates the update command.
func NewUpdateCmd(currentVersion func() string) *cobra.Command {
	var checkOnly bool

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update Mirako CLI to the latest version",
		Long:  "Check GitHub releases and update the current Mirako CLI executable to the latest version.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			version := currentVersion()
			ctx, cancel := context.WithTimeout(cmd.Context(), updateTimeout)
			defer cancel()

			if checkOnly {
				result, err := updater.CheckForUpdate(ctx, version, updater.Options{})
				if err != nil {
					return friendlyUpdateError(err)
				}
				if !result.Newer {
					fmt.Fprintf(cmd.OutOrStdout(), "Mirako CLI is up to date: %s\n", result.CurrentVersion)
					return nil
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Update available: %s (current: %s)\n", result.LatestVersion, result.CurrentVersion)
				fmt.Fprintln(cmd.OutOrStdout(), "Run: mirako update")
				return nil
			}

			executablePath, err := os.Executable()
			if err != nil {
				return fmt.Errorf("failed to determine current executable path: %w", err)
			}

			result, err := updater.InstallLatest(ctx, version, updater.Options{ExecutablePath: executablePath}, cmd.OutOrStdout())
			if err != nil {
				return friendlyUpdateError(err)
			}
			if !result.Newer {
				fmt.Fprintf(cmd.OutOrStdout(), "Mirako CLI is already up to date: %s\n", result.CurrentVersion)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&checkOnly, "check", false, "Only check whether an update is available")

	return cmd
}

func friendlyUpdateError(err error) error {
	if errors.Is(err, updater.ErrDevelopmentBuild) {
		return fmt.Errorf("this is a development build and cannot be updated automatically")
	}
	if errors.Is(err, updater.ErrHomebrewManaged) {
		return fmt.Errorf("this Mirako CLI installation appears to be managed by Homebrew\nUpdate with: brew upgrade mirako-ai/tap/mirako")
	}
	return err
}
