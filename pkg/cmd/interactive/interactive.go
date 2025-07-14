package interactive

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"github.com/mirako-ai/mirako-cli/pkg/ui"

	"github.com/spf13/cobra"
	"github.com/mirako-ai/mirako-cli/internal/api"
	"github.com/mirako-ai/mirako-cli/internal/client"
	"github.com/mirako-ai/mirako-cli/internal/config"
	"github.com/mirako-ai/mirako-cli/internal/errors"
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
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	client, err := client.New(cfg)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	resp, err := client.ListSessions(context.Background())
	if err != nil {
		if apiErr, ok := errors.IsAPIError(err); ok {
			return fmt.Errorf(apiErr.GetUserFriendlyMessage())
		}
		return fmt.Errorf("failed to list sessions: %w", err)
	}

	useJSON, _ := cmd.Flags().GetBool("json")
	if useJSON {
		data, _ := json.MarshalIndent(resp, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	if resp.JSON200 == nil || resp.JSON200.Data == nil || len(*resp.JSON200.Data) == 0 {
		fmt.Println("No active sessions found")
		return nil
	}

	t := ui.NewSessionTable(os.Stdout)
	for _, session := range *resp.JSON200.Data {
		state := ""
		if session.State != nil {
			state = *session.State
		}

		desiredState := ""
		if session.DesiredState != nil {
			desiredState = *session.DesiredState
		}

		t.AddRow([]interface{}{
			session.SessionId,
			session.MetisModel,
			state,
			desiredState,
			ui.FormatTimestamp(session.StartTime),
		})
	}
	t.Flush()
	return nil
}

func newStartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start a new interactive session",
		Long:  `Start a new interactive session with an avatar`,
		RunE:  runStart,
	}

	cmd.Flags().StringP("avatar", "a", "", "Avatar ID to use")
	cmd.Flags().StringP("model", "m", "metis-2.5", "Model to use")
	cmd.Flags().StringP("llm-model", "l", "gemini-2.0-flash", "LLM model to use")
	cmd.Flags().StringP("voice", "v", "", "Voice profile ID")
	cmd.Flags().StringP("instruction", "i", "You are a helpful AI assistant.", "Instruction prompt")

	return cmd
}

func runStart(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	avatarID, _ := cmd.Flags().GetString("avatar")
	if avatarID == "" {
		return fmt.Errorf("avatar ID is required. Use --avatar flag")
	}

	model, _ := cmd.Flags().GetString("model")
	llmModel, _ := cmd.Flags().GetString("llm-model")
	voiceID, _ := cmd.Flags().GetString("voice")
	instruction, _ := cmd.Flags().GetString("instruction")

	if voiceID == "" {
		voiceID = cfg.DefaultVoice
	}

	client, err := client.New(cfg)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	body := api.StartSessionApiRequestBody{
		AvatarId:       avatarID,
		Model:          api.StartSessionApiRequestBodyModel(model),
		LlmModel:       llmModel,
		VoiceProfileId: voiceID,
		Instruction:    instruction,
	}

	resp, err := client.StartSession(context.Background(), body)
	if err != nil {
		if apiErr, ok := errors.IsAPIError(err); ok {
			return fmt.Errorf(apiErr.GetUserFriendlyMessage())
		}
		return fmt.Errorf("failed to start session: %w", err)
	}

	if resp.JSON200 == nil {
		return fmt.Errorf("unexpected response from server")
	}

	fmt.Printf("✅ Session started successfully!\n")
	fmt.Printf("   Session ID: %s\n", resp.JSON200.Data.Session.SessionId)
	fmt.Printf("   Session Token: %s\n", resp.JSON200.Data.SessionToken)
	fmt.Printf("   Model: %s\n", resp.JSON200.Data.Session.MetisModel)

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
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	client, err := client.New(cfg)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	resp, err := client.StopSessions(context.Background(), args)
	if err != nil {
		if apiErr, ok := errors.IsAPIError(err); ok {
			return fmt.Errorf(apiErr.GetUserFriendlyMessage())
		}
		return fmt.Errorf("failed to stop sessions: %w", err)
	}

	if resp.JSON200 == nil || resp.JSON200.Data == nil {
		return fmt.Errorf("unexpected response from server")
	}

	if resp.JSON200.Data.StoppedSessions != nil && len(*resp.JSON200.Data.StoppedSessions) > 0 {
		fmt.Printf("✅ Successfully stopped %d session(s):\n", len(*resp.JSON200.Data.StoppedSessions))
		for _, sessionID := range *resp.JSON200.Data.StoppedSessions {
			fmt.Printf("   - %s\n", sessionID)
		}
	} else {
		fmt.Println("No sessions were stopped")
	}

	return nil
}