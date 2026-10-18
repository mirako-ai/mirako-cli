package voice

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCloneVoiceCommand(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "missing-config")
	t.Setenv("MIRAKO_CONFIG_PATH", configPath)
	t.Setenv("MIRAKO_API_TOKEN", "")
	t.Setenv("MIRAKO_API_URL", "://invalid")

	missingAudioDir := filepath.Join(t.TempDir(), "missing-audio")
	missingAnnotations := filepath.Join(t.TempDir(), "missing-annotations.txt")

	tests := []struct {
		name string
		args []string
	}{
		{
			name: "bare command",
		},
		{
			name: "legacy long flags with invalid values",
			args: []string{
				"--name", strings.Repeat("n", 65),
				"--audio-dir", missingAudioDir,
				"--annotations", missingAnnotations,
				"--poll-interval", "-1",
				"--clean-data",
				"--description", strings.Repeat("d", 513),
			},
		},
		{
			name: "legacy shorthand flags with invalid values",
			args: []string{
				"-n", "",
				"-a", missingAudioDir,
				"-t", missingAnnotations,
				"-p", "0",
				"-c=false",
				"-d", strings.Repeat("d", 513),
			},
		},
		{
			name: "optional name",
			args: []string{"--name", "x"},
		},
		{
			name: "optional audio directory",
			args: []string{"--audio-dir", missingAudioDir},
		},
		{
			name: "optional annotations",
			args: []string{"--annotations", missingAnnotations},
		},
		{
			name: "optional polling interval",
			args: []string{"--poll-interval", "not-a-number"},
		},
		{
			name: "invalid shorthand polling interval",
			args: []string{"-p", "not-a-number"},
		},
		{
			name: "optional clean data",
			args: []string{"--clean-data=not-a-boolean"},
		},
		{
			name: "invalid shorthand clean data",
			args: []string{"-c=not-a-boolean"},
		},
		{
			name: "bare shorthand clean data",
			args: []string{"-c"},
		},
		{
			name: "optional description",
			args: []string{"--description", strings.Repeat("d", 513)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			cmd := NewVoiceCmd()
			cmd.SetOut(&stdout)
			cmd.SetErr(&stderr)
			cmd.SetArgs(append([]string{"clone"}, tt.args...))

			require.NoError(t, cmd.Execute())
			assert.Equal(t, "Voice cloning feature is being redesigned and will be available soon\n", stdout.String())
			assert.Empty(t, stderr.String())
			_, err := os.Stat(configPath)
			assert.True(t, os.IsNotExist(err), "clone must not load or create configuration: %v", err)
		})
	}
}

func TestCloneVoiceLegacyFlags(t *testing.T) {
	cmd := newCloneVoiceCmd()
	flags := []struct {
		name      string
		shorthand string
	}{
		{name: "name", shorthand: "n"},
		{name: "audio-dir", shorthand: "a"},
		{name: "annotations", shorthand: "t"},
		{name: "poll-interval", shorthand: "p"},
		{name: "clean-data", shorthand: "c"},
		{name: "description", shorthand: "d"},
	}

	for _, tt := range flags {
		t.Run(tt.name, func(t *testing.T) {
			flag := cmd.Flags().Lookup(tt.name)
			require.NotNil(t, flag)
			assert.Equal(t, tt.shorthand, flag.Shorthand)
			assert.True(t, flag.Hidden)
			assert.Empty(t, flag.Annotations)
		})
	}
}

func TestVoiceCommandHelp(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "voice group",
			args: []string{"--help"},
			want: "Manage premade and custom voice profiles for TTS",
		},
		{
			name: "clone command",
			args: []string{"clone", "--help"},
			want: "Voice cloning feature is being redesigned and will be available soon",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout bytes.Buffer
			cmd := NewVoiceCmd()
			cmd.SetOut(&stdout)
			cmd.SetArgs(tt.args)

			require.NoError(t, cmd.Execute())
			output := stdout.String()
			assert.Contains(t, output, tt.want)
			assert.Contains(t, output, "being redesigned")
			assert.NotContains(t, output, "voice cloning services")
			assert.NotContains(t, output, "Clone a voice from audio samples")
			for _, flag := range []string{"name", "audio-dir", "annotations", "poll-interval", "clean-data", "description"} {
				assert.NotContains(t, output, "--"+flag)
			}
		})
	}
}

func TestCloneVoiceOutputError(t *testing.T) {
	wantErr := errors.New("output unavailable")
	cmd := NewVoiceCmd()
	cmd.SetArgs([]string{"clone"})
	cmd.SetOut(cloneErrorWriter{err: wantErr})
	cmd.SetErr(io.Discard)
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true

	require.ErrorIs(t, cmd.Execute(), wantErr)
}

type cloneErrorWriter struct {
	err error
}

func (w cloneErrorWriter) Write([]byte) (int, error) {
	return 0, w.err
}
