package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupConfigTest(t *testing.T) string {
	t.Helper()
	viper.Reset()
	originalConfigPath := ConfigPath
	t.Cleanup(func() {
		viper.Reset()
		ConfigPath = originalConfigPath
	})
	dir := t.TempDir()
	t.Setenv("MIRAKO_CONFIG_PATH", dir)
	return filepath.Join(dir, DefaultConfigFileName)
}

func TestLoadConfig(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    Config
	}{
		{
			name: "empty config file",
			want: Config{APIURL: "https://mirako.co", DefaultSavePath: "."},
		},
		{
			name: "full config",
			content: `api_token: test-token
api_url: https://test.mirako.co
default_voice: test-voice
default_save_path: /test/path
`,
			want: Config{
				APIToken:        "test-token",
				APIURL:          "https://test.mirako.co",
				DefaultVoice:    "test-voice",
				DefaultSavePath: "/test/path",
			},
		},
		{
			name:    "partial config",
			content: "api_token: test-token\n",
			want:    Config{APIToken: "test-token", APIURL: "https://mirako.co", DefaultSavePath: "."},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := setupConfigTest(t)
			require.NoError(t, os.WriteFile(path, []byte(tt.content), 0600))
			cfg, err := Load()
			require.NoError(t, err)
			assert.Equal(t, tt.want, *cfg)
		})
	}
}

func TestLoadCreatesDefaultConfig(t *testing.T) {
	path := setupConfigTest(t)
	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, Config{APIURL: "https://mirako.co", DefaultSavePath: "."}, *cfg)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.NotContains(t, string(data), "interactive_profiles")
	assert.NotContains(t, string(data), "metis")

	viper.Reset()
	loaded, err := Load()
	require.NoError(t, err)
	assert.Equal(t, cfg, loaded)
}

func TestSaveConfig(t *testing.T) {
	path := setupConfigTest(t)
	ConfigPath = filepath.Dir(path)
	cfg := &Config{
		APIToken:        "test-token",
		APIURL:          "https://test.mirako.co",
		DefaultVoice:    "test-voice",
		DefaultSavePath: "/test/path",
	}
	require.NoError(t, cfg.Save())

	viper.Reset()
	loaded, err := Load()
	require.NoError(t, err)
	assert.Equal(t, cfg, loaded)
}

func TestIsAuthenticated(t *testing.T) {
	for _, tt := range []struct {
		name  string
		token string
		want  bool
	}{
		{name: "with token", token: "test-token", want: true},
		{name: "empty token", want: false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{APIToken: tt.token}
			assert.Equal(t, tt.want, cfg.IsAuthenticated())
		})
	}
}
