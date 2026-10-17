package root

import (
	"bytes"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRootCommands(t *testing.T) {
	for _, name := range []string{"agent", "auth", "avatar", "completion", "config", "image", "speech", "update", "video", "voice"} {
		cmd, _, err := rootCmd.Find([]string{name})
		require.NoError(t, err)
		require.Equal(t, name, cmd.Name())
	}
	cmd, _, err := rootCmd.Find([]string{"agent", "routes"})
	require.NoError(t, err)
	require.Equal(t, "routes", cmd.Name())

	for _, name := range []string{"list", "start", "stop"} {
		_, _, err := rootCmd.Find([]string{"interactive", name})
		require.ErrorContains(t, err, `unknown command "interactive"`)
	}
	require.NotContains(t, strings.ToLower(rootCmd.UsageString()), "interactive")
	require.NotContains(t, strings.ToLower(rootCmd.Long), "interactive")
}

func TestShellCompletionsExcludeLegacyCommands(t *testing.T) {
	legacyCommand := regexp.MustCompile(`(?m)(^|[\s"'_;])interactive([\s"'_:;]|$)`)
	for _, shell := range []string{"bash", "zsh", "fish", "powershell"} {
		t.Run(shell, func(t *testing.T) {
			var output bytes.Buffer
			var err error
			switch shell {
			case "bash":
				err = rootCmd.GenBashCompletionV2(&output, true)
			case "zsh":
				err = rootCmd.GenZshCompletion(&output)
			case "fish":
				err = rootCmd.GenFishCompletion(&output, true)
			case "powershell":
				err = rootCmd.GenPowerShellCompletionWithDesc(&output)
			}
			require.NoError(t, err)
			require.NotEmpty(t, output.String())
			require.False(t, legacyCommand.Match(output.Bytes()), "completion still includes legacy interactive command")
		})
	}
}
