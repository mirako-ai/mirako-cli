package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig(t *testing.T) {
	// Create temporary directory for test config
	tempDir, err := os.MkdirTemp("", "mirako-config-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Save original config paths
	originalConfigDir := ConfigDir
	originalConfigFile := ConfigFile
	defer func() {
		ConfigDir = originalConfigDir
		ConfigFile = originalConfigFile
	}()

	// Set test config paths
	ConfigDir = tempDir
	ConfigFile = filepath.Join(tempDir, "config.yml")

	tests := []struct {
		name           string
		configContent  string
		expectedConfig *Config
		expectError    bool
	}{
		{
			name:          "empty config file",
			configContent: ``,
			expectedConfig: &Config{
				APIURL:              "https://mirako.co",
				DefaultModel:        "metis-2.5",
				DefaultVoice:        "",
				DefaultSavePath:     ".",
				InteractiveProfiles: map[string]InteractiveProfile{},
			},
			expectError: false,
		},
		{
			name: "full config with Default profile",
			configContent: `api_token: test-token
api_url: https://test.mirako.co
default_model: test-model
default_voice: test-voice
default_save_path: /test/path
interactive_profiles:
  Default:
    avatar_id: test-avatar-id
    model: test-model
    llm_model: test-llm-model
    voice_profile_id: test-voice-id
    instruction: test instruction
    tools: test-tools
`,
			expectedConfig: &Config{
				APIToken:        "test-token",
				APIURL:          "https://test.mirako.co",
				DefaultModel:    "test-model",
				DefaultVoice:    "test-voice",
				DefaultSavePath: "/test/path",
				InteractiveProfiles: map[string]InteractiveProfile{
					"default": {
						AvatarID:       "test-avatar-id",
						Model:          "test-model",
						LLMModel:       "test-llm-model",
						VoiceProfileID: "test-voice-id",
						Instruction:    "test instruction",
						Tools:          "test-tools",
					},
				},
			},
			expectError: false,
		},
		{
			name: "config with multiple profiles",
			configContent: `api_token: test-token
interactive_profiles:
  Default:
    avatar_id: default-avatar
    model: default-model
    instruction: default instruction
  CustomProfile:
    avatar_id: custom-avatar
    model: custom-model
    instruction: custom instruction
`,
			expectedConfig: &Config{
				APIToken:        "test-token",
				APIURL:          "https://mirako.co",
				DefaultModel:    "metis-2.5",
				DefaultVoice:    "",
				DefaultSavePath: ".",
				InteractiveProfiles: map[string]InteractiveProfile{
					"default": {
						AvatarID:    "default-avatar",
						Model:       "default-model",
						Instruction: "default instruction",
					},
					"customprofile": {
						AvatarID:    "custom-avatar",
						Model:       "custom-model",
						Instruction: "custom instruction",
					},
				},
			},
			expectError: false,
		},
		{
			name: "config with partial profiles",
			configContent: `interactive_profiles:
  Default:
    avatar_id: test-avatar
  MinimalProfile:
    model: minimal-model
`,
			expectedConfig: &Config{
				APIURL:          "https://mirako.co",
				DefaultModel:    "metis-2.5",
				DefaultVoice:    "",
				DefaultSavePath: ".",
				InteractiveProfiles: map[string]InteractiveProfile{
					"default": {
						AvatarID: "test-avatar",
					},
					"minimalprofile": {
						Model: "minimal-model",
					},
				},
			},
			expectError: false,
		},
		{
			name: "config with empty interactive_profiles",
			configContent: `api_token: test-token
interactive_profiles: {}
`,
			expectedConfig: &Config{
				APIToken:            "test-token",
				APIURL:              "https://mirako.co",
				DefaultModel:        "metis-2.5",
				DefaultVoice:        "",
				DefaultSavePath:     ".",
				InteractiveProfiles: map[string]InteractiveProfile{},
			},
			expectError: false,
		},
		{
			name: "config with special characters in profile name",
			configContent: `interactive_profiles:
  "Test-Profile_123":
    avatar_id: special-avatar
    model: special-model
`,
			expectedConfig: &Config{
				APIURL:          "https://mirako.co",
				DefaultModel:    "metis-2.5",
				DefaultVoice:    "",
				DefaultSavePath: ".",
				InteractiveProfiles: map[string]InteractiveProfile{
					"test-profile_123": {
						AvatarID: "special-avatar",
						Model:    "special-model",
					},
				},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset viper for clean state
			viper.Reset()
			
			// Create test config file
			if tt.configContent != "" {
				err := os.WriteFile(ConfigFile, []byte(tt.configContent), 0644)
				require.NoError(t, err)
			}

			cfg, err := Load()
			if tt.expectError {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expectedConfig.APIURL, cfg.APIURL)
			assert.Equal(t, tt.expectedConfig.DefaultModel, cfg.DefaultModel)
			assert.Equal(t, tt.expectedConfig.DefaultVoice, cfg.DefaultVoice)
			assert.Equal(t, tt.expectedConfig.DefaultSavePath, cfg.DefaultSavePath)
			assert.Equal(t, tt.expectedConfig.APIToken, cfg.APIToken)
			assert.Equal(t, len(tt.expectedConfig.InteractiveProfiles), len(cfg.InteractiveProfiles))
			
			// Compare interactive profiles
			for expectedName, expectedProfile := range tt.expectedConfig.InteractiveProfiles {
				actualProfile, exists := cfg.InteractiveProfiles[expectedName]
				assert.True(t, exists, "profile %s should exist", expectedName)
				assert.Equal(t, expectedProfile, actualProfile)
			}
		})
	}
}

func TestLoadConfig_FileNotFound(t *testing.T) {
	// Create temporary directory for test
	tempDir, err := os.MkdirTemp("", "mirako-config-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Save original config paths
	originalConfigDir := ConfigDir
	originalConfigFile := ConfigFile
	defer func() {
		ConfigDir = originalConfigDir
		ConfigFile = originalConfigFile
	}()

	// Set test config paths to non-existent directory
	ConfigDir = filepath.Join(tempDir, "nonexistent")
	ConfigFile = filepath.Join(ConfigDir, "config.yml")

	// Reset viper for clean state
	viper.Reset()

	cfg, err := Load()
	assert.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, "https://mirako.co", cfg.APIURL)
	assert.Equal(t, "metis-2.5", cfg.DefaultModel)
	assert.Equal(t, "", cfg.DefaultVoice)
	assert.Equal(t, ".", cfg.DefaultSavePath)
	assert.Equal(t, map[string]InteractiveProfile{}, cfg.InteractiveProfiles)
}

func TestSaveConfig(t *testing.T) {
	// Create temporary directory for test
	tempDir, err := os.MkdirTemp("", "mirako-config-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Save original config paths
	originalConfigDir := ConfigDir
	originalConfigFile := ConfigFile
	defer func() {
		ConfigDir = originalConfigDir
		ConfigFile = originalConfigFile
	}()

	// Set test config paths
	ConfigDir = tempDir
	ConfigFile = filepath.Join(tempDir, "config.yml")

	cfg := &Config{
		APIToken:        "test-token",
		APIURL:          "https://test.mirako.co",
		DefaultModel:    "test-model",
		DefaultVoice:    "test-voice",
		DefaultSavePath: "/test/path",
		InteractiveProfiles: map[string]InteractiveProfile{
			"default": {
				AvatarID:       "test-avatar",
				Model:          "test-model",
				LLMModel:       "test-llm",
				VoiceProfileID: "test-voice-id",
				Instruction:    "test instruction",
				Tools:          "test-tools",
			},
			"custom": {
				AvatarID: "custom-avatar",
				Model:    "custom-model",
			},
		},
	}

	err = cfg.Save()
	assert.NoError(t, err)

	// Reset viper state and load the saved config
	viper.Reset()
	loadedCfg, err := Load()
	assert.NoError(t, err)

	assert.Equal(t, cfg.APIToken, loadedCfg.APIToken)
	assert.Equal(t, cfg.APIURL, loadedCfg.APIURL)
	assert.Equal(t, cfg.DefaultModel, loadedCfg.DefaultModel)
	assert.Equal(t, cfg.DefaultVoice, loadedCfg.DefaultVoice)
	assert.Equal(t, cfg.DefaultSavePath, loadedCfg.DefaultSavePath)
	
	// Compare interactive profiles individually since maps can't be directly compared
	assert.Equal(t, len(cfg.InteractiveProfiles), len(loadedCfg.InteractiveProfiles))
	for name, expectedProfile := range cfg.InteractiveProfiles {
		actualProfile, exists := loadedCfg.InteractiveProfiles[name]
		assert.True(t, exists, "profile %s should exist", name)
		assert.Equal(t, expectedProfile.AvatarID, actualProfile.AvatarID)
		assert.Equal(t, expectedProfile.Model, actualProfile.Model)
		assert.Equal(t, expectedProfile.LLMModel, actualProfile.LLMModel)
		assert.Equal(t, expectedProfile.VoiceProfileID, actualProfile.VoiceProfileID)
		assert.Equal(t, expectedProfile.Instruction, actualProfile.Instruction)
		assert.Equal(t, expectedProfile.Tools, actualProfile.Tools)
	}
}

func TestIsAuthenticated(t *testing.T) {
	tests := []struct {
		name     string
		token    string
		expected bool
	}{
		{"with token", "test-token", true},
		{"empty token", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{APIToken: tt.token}
			assert.Equal(t, tt.expected, cfg.IsAuthenticated())
		})
	}
}
