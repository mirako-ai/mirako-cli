package interactive

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mirako-ai/mirako-cli/internal/client"
	"github.com/mirako-ai/mirako-cli/internal/config"
	"github.com/mirako-ai/mirako-cli/internal/errors"
	"github.com/mirako-ai/mirako-cli/pkg/cmd/util"
	"github.com/mirako-ai/mirako-cli/pkg/ui"
	"github.com/mirako-ai/mirako-cli/pkg/utils"
	"github.com/mirako-ai/mirako-go/api"
	"github.com/spf13/cobra"
	"os"
)

func NewInteractiveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "interactive",
		Short: "Manage interactive sessions",
		Long:  `Start, stop, and manage interactive AI sessions`,
	}

	cmd.AddCommand(newListCmd())
	cmd.AddCommand(newStartCmd())
	cmd.AddCommand(newStopCmd())

	return cmd
}

func newListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List active sessions",
		Long:  `List all active interactive sessions`,
		RunE:  runList,
	}

	cmd.Flags().BoolP("json", "j", false, "Output in JSON format")

	return cmd
}

func runList(cmd *cobra.Command, args []string) error {
	cfg, err := util.GetConfig(cmd)
	if err != nil {
		return err
	}

	client, err := client.New(cfg)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	resp, err := client.ListSessions(context.Background())
	if err != nil {
		if apiErr, ok := errors.IsAPIError(err); ok {
			return fmt.Errorf("%s", apiErr.GetUserFriendlyMessage())
		}
		return fmt.Errorf("failed to list sessions: %w", err)
	}

	useJSON, _ := cmd.Flags().GetBool("json")
	if useJSON {
		data, _ := json.MarshalIndent(resp, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	if resp == nil || resp.Data == nil || len(*resp.Data) == 0 {
		fmt.Println("No active sessions found")
		return nil
	}

	t := ui.NewSessionTable(os.Stdout)
	for _, session := range *resp.Data {
		state := ""
		if session.State != nil {
			state = *session.State
		}

		sessionId := ""
		if session.SessionId != nil {
			sessionId = *session.SessionId
		}

		model := ""
		if session.MetisModel != nil {
			model = *session.MetisModel
		}

		t.AddRow([]interface{}{
			sessionId,
			model,
			state,
			ui.FormatTimestamp(session.StartTime),
		})
	}
	t.Flush()
	return nil
}

func newStartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start [profile-name]",
		Short: "Start a new interactive session",
		Long: `Start a new interactive session with an avatar.

You can start a session in three ways:
1. Using CLI flags: mirako interactive start --avatar abc123 --voice xyz789
2. Using Default profile: mirako interactive start
3. Using named profile: mirako interactive start my-profile

When using a profile, CLI flags will override profile values.`,
		RunE: runStart,
	}

	cmd.Flags().StringP("avatar", "a", "", "Avatar ID to use")
	cmd.Flags().StringP("model", "m", "", "Model to use")
	cmd.Flags().StringP("llm-model", "l", "", "LLM model to use")
	cmd.Flags().StringP("voice", "v", "", "Voice profile ID")
	cmd.Flags().StringP("instruction", "i", "", "Instruction prompt")
	cmd.Flags().StringP("tools", "", "", "Tools to use in the session (JSON array string)")
	cmd.Flags().Int64P("idle-timeout", "t", 15, "Idle timeout in minutes (-1 to disable, default: 15)")

	return cmd
}

func runStart(cmd *cobra.Command, args []string) error {
	cfg, err := util.GetConfig(cmd)
	if err != nil {
		return err
	}

	// Determine which profile to use
	var profile config.InteractiveProfile
	var profileName string

	if len(args) > 0 {
		profileName = args[0]
		if p, exists := cfg.InteractiveProfiles[profileName]; exists {
			profile = p
		} else {
			return fmt.Errorf("profile '%s' not found in config", profileName)
		}
	} else {
		// Use Default profile (viper converts keys to lowercase)
		var defaultProfile *config.InteractiveProfile
		for name, profile := range cfg.InteractiveProfiles {
			if strings.EqualFold(name, "default") {
				defaultProfile = &profile
				break
			}
		}

		if defaultProfile == nil {
			fmt.Printf("❌ No default profile found in config\n\n")
			fmt.Printf("To use interactive sessions without specifying a profile, you need to create a 'default' profile in your config.yml:\n\n")
			fmt.Printf("Location: ~/.mirako/config.yml\n")
			fmt.Printf("Add the following:\n\n")
			fmt.Printf("interactive_profiles:\n")
			fmt.Printf("  default:\n")
			fmt.Printf("    avatar_id: [YOUR_AVATAR_ID]\n")
			fmt.Printf("    model: metis-2.5\n")
			fmt.Printf("    llm_model: gemini-2.0-flash\n")
			fmt.Printf("    voice_profile_id: [YOUR_VOICE_PROFILE_ID]\n")
			fmt.Printf("    instruction: You are a helpful AI assistant.\n")
			fmt.Printf("    tools: []\n\n")
			fmt.Printf("You can also specify a profile name: mirako interactive start [profile-name]\n")
			fmt.Printf("Or use CLI flags directly: mirako interactive start --avatar YOUR_AVATAR_ID --voice YOUR_VOICE_ID\n")
			return nil
		}
		profile = *defaultProfile
	}

	// Get CLI flags (these will override profile values)
	avatarID, _ := cmd.Flags().GetString("avatar")
	model, _ := cmd.Flags().GetString("model")
	llmModel, _ := cmd.Flags().GetString("llm-model")
	voiceID, _ := cmd.Flags().GetString("voice")
	instruction, _ := cmd.Flags().GetString("instruction")
	tools, _ := cmd.Flags().GetString("tools")
	idleTimeout, _ := cmd.Flags().GetInt64("idle-timeout")

	// Apply priority: CLI flags > profile values > defaults
	if avatarID == "" {
		avatarID = profile.AvatarID
	}
	if avatarID == "" {
		return fmt.Errorf("Could not find avatar ID in the profile. Use --avatar flag or set `avatar_id` in profile")
	}
	if model == "" {
		model = profile.Model
	}
	if model == "" {
		model = config.DefaultInteractiveModel
	}
	if llmModel == "" {
		llmModel = profile.LLMModel
	}
	if llmModel == "" {
		llmModel = config.DefaultLLMModel
	}
	if voiceID == "" {
		voiceID = profile.VoiceProfileID
	}
	if voiceID == "" {
		voiceID = cfg.DefaultVoice
	}
	if voiceID == "" {
		return fmt.Errorf("Could not find voice profile ID in the profile. Use --voice flag, set `voice_profile_id` in profile, or set `default_voice` in config")
	}
	if instruction == "" {
		instruction = profile.Instruction
	}
	if instruction == "" {
		instruction = "You are a helpful AI assistant."
	}

	var toolsJSON string
	if tools == "" && len(profile.Tools) > 0 {
		toolsBytes, err := json.Marshal(profile.Tools)
		if err != nil {
			return fmt.Errorf("failed to marshal tools from profile: %w", err)
		}
		toolsJSON = string(toolsBytes)
	} else {
		toolsJSON = tools
	}

	// Handle idle timeout - use profile value if flag is default (15) and profile has a value
	if idleTimeout == 15 && profile.IdleTimeout != 0 {
		idleTimeout = profile.IdleTimeout
	}

	client, err := client.New(cfg)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	var modelPtr *api.StartSessionApiRequestBodyModel
	if model != "" {
		modelValue := api.StartSessionApiRequestBodyModel(model)
		modelPtr = &modelValue
	}

	body := api.StartInteractiveSessionJSONRequestBody{
		AvatarId:       avatarID,
		Model:          modelPtr,
		LlmModel:       llmModel,
		VoiceProfileId: voiceID,
		Instruction:    instruction,
	}

	// Set idle timeout if not default (15) or if explicitly provided
	if idleTimeout != 15 || cmd.Flags().Changed("idle-timeout") {
		body.IdleTimeout = &idleTimeout
	}

	if toolsJSON != "" {
		body.Tools = &toolsJSON
	}

	resp, err := client.StartSession(context.Background(), body)
	if err != nil {
		if apiErr, ok := errors.IsAPIError(err); ok {
			return fmt.Errorf("%s", apiErr.GetUserFriendlyMessage())
		}
		return fmt.Errorf("failed to start session: %w", err)
	}

	if resp.Data == nil {
		return fmt.Errorf("unexpected response from server")
	}

	fmt.Printf("✅ Session started successfully!\n")
	if profileName != "" {
		fmt.Printf("   Profile: %s\n", profileName)
	}
	fmt.Printf("   Session ID: %s\n", *resp.Data.Session.SessionId)
	fmt.Printf("   Model: %s\n", *resp.Data.Session.MetisModel)
	fmt.Printf("   LLM Model: %s\n", llmModel)
	fmt.Printf("   Voice: %s\n", voiceID)
	fmt.Printf("You can use the following token for interactive api calls:\n   %s", resp.Data.SessionToken)
	fmt.Println()
	fmt.Println()
	// try open the url in default browser
	url := fmt.Sprintf("https://interactive.mirako.ai/i/%s", *resp.Data.Session.SessionId)
	if err = utils.OpenURLAndForget(url); err != nil {
		// use test hint instead
		fmt.Printf("You can now visit the url: %s", url)
	} else {
		fmt.Printf("Opened session in browser: %s\n", url)
	}
	fmt.Println()
	return nil
}

func newStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop [session-id...]",
		Short: "Stop interactive sessions",
		Long:  `Stop one or more interactive sessions`,
		Args:  cobra.MinimumNArgs(1),
		RunE:  runStop,
	}
}

func runStop(cmd *cobra.Command, args []string) error {
	cfg, err := util.GetConfig(cmd)
	if err != nil {
		return err
	}

	client, err := client.New(cfg)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	resp, err := client.StopSessions(context.Background(), args)
	if err != nil {
		if apiErr, ok := errors.IsAPIError(err); ok {
			return fmt.Errorf("%s", apiErr.GetUserFriendlyMessage())
		}
		return fmt.Errorf("failed to stop sessions: %w", err)
	}

	if resp.Data == nil {
		return fmt.Errorf("unexpected response from server")
	}

	if resp.Data.StoppedSessions != nil && len(*resp.Data.StoppedSessions) > 0 {
		fmt.Printf("✅ Successfully stopped %d session(s):\n", len(*resp.Data.StoppedSessions))
		for _, sessionID := range *resp.Data.StoppedSessions {
			fmt.Printf("   - %s\n", sessionID)
		}
	} else {
		fmt.Println("No sessions were stopped")
	}

	return nil
}
