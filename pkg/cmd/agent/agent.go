package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/mirako-ai/mirako-cli/internal/client"
	"github.com/mirako-ai/mirako-cli/internal/config"
	"github.com/mirako-ai/mirako-cli/internal/errors"
	"github.com/mirako-ai/mirako-cli/pkg/cmd/util"
	"github.com/mirako-ai/mirako-cli/pkg/ui"
	"github.com/mirako-ai/mirako-go/api"
	"github.com/spf13/cobra"
)

func NewAgentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent",
		Short: "Manage agents",
		Long:  `Create, list, view, and delete persistent agent configurations`,
	}

	cmd.AddCommand(newListCmd())
	cmd.AddCommand(newViewCmd())
	cmd.AddCommand(newCreateCmd())
	cmd.AddCommand(newDeleteCmd())

	return cmd
}

func newListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List agents",
		Long:  `List all persistent agent configurations for the current user`,
		RunE:  runList,
	}

	cmd.Flags().BoolP("json", "j", false, "Output in JSON format")

	return cmd
}

func runList(cmd *cobra.Command, args []string) error {
	c, err := newClient(cmd)
	if err != nil {
		return err
	}

	resp, err := c.ListAgents(cmd.Context())
	if err != nil {
		return formatAPIError(err, "failed to list agents")
	}

	useJSON, _ := cmd.Flags().GetBool("json")
	if useJSON {
		data, _ := json.MarshalIndent(resp, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	if resp == nil || resp.Data == nil || len(*resp.Data) == 0 {
		fmt.Println("No agents found")
		return nil
	}

	t := ui.NewAgentTable(os.Stdout)
	for _, agent := range *resp.Data {
		t.AddRow([]interface{}{
			agent.Name,
			agent.Id,
			agent.AvatarId,
			agent.VoiceProfileId,
			agent.Model,
			agent.LlmModel,
			ui.FormatTimestamp(agent.UpdatedAt),
		})
	}
	return t.Flush()
}

func newViewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "view [agent-id]",
		Short: "View agent details",
		Long:  `View detailed information about a persistent agent configuration`,
		Args:  cobra.ExactArgs(1),
		RunE:  runView,
	}

	cmd.Flags().BoolP("json", "j", false, "Output in JSON format")

	return cmd
}

func runView(cmd *cobra.Command, args []string) error {
	c, err := newClient(cmd)
	if err != nil {
		return err
	}

	resp, err := c.GetAgent(cmd.Context(), args[0])
	if err != nil {
		return formatAPIError(err, "failed to get agent")
	}
	if resp == nil {
		return fmt.Errorf("unexpected response from server")
	}

	useJSON, _ := cmd.Flags().GetBool("json")
	if useJSON {
		data, _ := json.MarshalIndent(resp, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	agent := resp.Data
	tools := []any{}
	if agent.Tools != nil {
		tools = *agent.Tools
	}
	toolsJSON, err := json.MarshalIndent(tools, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to format agent tools: %w", err)
	}

	fmt.Printf("ID: %s\n", agent.Id)
	fmt.Printf("Name: %s\n", agent.Name)
	if agent.Description != nil && *agent.Description != "" {
		fmt.Printf("Description: %s\n", *agent.Description)
	}
	fmt.Printf("Avatar ID: %s\n", agent.AvatarId)
	fmt.Printf("Voice Profile ID: %s\n", agent.VoiceProfileId)
	fmt.Printf("Model: %s\n", agent.Model)
	fmt.Printf("LLM Model: %s\n", agent.LlmModel)
	fmt.Printf("Created: %s\n", ui.FormatTimestamp(agent.CreatedAt))
	fmt.Printf("Updated: %s\n", ui.FormatTimestamp(agent.UpdatedAt))
	fmt.Printf("\nInstruction:\n%s\n", agent.Instruction)
	fmt.Printf("\nTools:\n%s\n", string(toolsJSON))

	return nil
}

func newCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create an agent",
		Long:  `Create a persistent agent configuration using an avatar, voice profile, and instruction prompt`,
		RunE:  runCreate,
	}

	cmd.Flags().StringP("name", "n", "", "Name for the agent")
	cmd.Flags().StringP("description", "d", "", "Description for the agent")
	cmd.Flags().StringP("avatar", "a", "", "Avatar ID to use")
	cmd.Flags().StringP("voice", "v", "", "Voice profile ID to use")
	cmd.Flags().StringP("model", "m", config.DefaultInteractiveModel, "Interactive model to use")
	cmd.Flags().StringP("llm-model", "l", "", "LLM model to use")
	cmd.Flags().StringP("instruction", "i", "", "Instruction prompt text")
	cmd.Flags().String("instruction-file", "", "Path to a text or Markdown file containing the instruction prompt")
	cmd.Flags().String("tools", "", "Tools to use for the agent (JSON array string)")
	cmd.Flags().String("tools-file", "", "Path to a JSON file containing a tools array")
	cmd.Flags().BoolP("json", "j", false, "Output in JSON format")

	return cmd
}

func runCreate(cmd *cobra.Command, args []string) error {
	name, _ := cmd.Flags().GetString("name")
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("name is required. Use --name flag")
	}

	avatarID, _ := cmd.Flags().GetString("avatar")
	if strings.TrimSpace(avatarID) == "" {
		return fmt.Errorf("avatar ID is required. Use --avatar flag")
	}

	voiceID, _ := cmd.Flags().GetString("voice")
	if strings.TrimSpace(voiceID) == "" {
		return fmt.Errorf("voice profile ID is required. Use --voice flag")
	}

	llmModel, _ := cmd.Flags().GetString("llm-model")
	if strings.TrimSpace(llmModel) == "" {
		return fmt.Errorf("LLM model is required. Use --llm-model flag")
	}

	instruction, err := resolveInstruction(cmd)
	if err != nil {
		return err
	}
	if strings.TrimSpace(instruction) == "" {
		return fmt.Errorf("instruction is required. Use --instruction or --instruction-file")
	}

	tools, err := resolveTools(cmd)
	if err != nil {
		return err
	}

	description, _ := cmd.Flags().GetString("description")
	model, _ := cmd.Flags().GetString("model")
	body := api.CreateAgentJSONRequestBody{
		Name:           name,
		AvatarId:       avatarID,
		VoiceProfileId: voiceID,
		LlmModel:       llmModel,
		Instruction:    instruction,
		Tools:          &tools,
	}
	if description != "" {
		body.Description = &description
	}
	if model != "" {
		body.Model = &model
	}

	c, err := newClient(cmd)
	if err != nil {
		return err
	}

	resp, err := c.CreateAgent(cmd.Context(), body)
	if err != nil {
		return formatAPIError(err, "failed to create agent")
	}
	if resp == nil {
		return fmt.Errorf("unexpected response from server")
	}

	useJSON, _ := cmd.Flags().GetBool("json")
	if useJSON {
		data, _ := json.MarshalIndent(resp, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	agent := resp.Data
	fmt.Printf("Agent created successfully!\n")
	fmt.Printf("   Agent ID: %s\n", agent.Id)
	fmt.Printf("   Name: %s\n", agent.Name)
	fmt.Printf("   Avatar ID: %s\n", agent.AvatarId)
	fmt.Printf("   Voice Profile ID: %s\n", agent.VoiceProfileId)
	fmt.Printf("   Model: %s\n", agent.Model)
	fmt.Printf("   LLM Model: %s\n", agent.LlmModel)

	return nil
}

func newDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete [agent-id]",
		Short: "Delete an agent",
		Long:  `Delete a persistent agent configuration. This action cannot be undone.`,
		Args:  cobra.ExactArgs(1),
		RunE:  runDelete,
	}

	cmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")

	return cmd
}

func runDelete(cmd *cobra.Command, args []string) error {
	agentID := args[0]
	force, _ := cmd.Flags().GetBool("force")

	if !force {
		confirm := false
		prompt := &survey.Confirm{
			Message: fmt.Sprintf("Are you sure you want to delete agent %s? This action cannot be undone.", agentID),
			Default: false,
		}
		if err := survey.AskOne(prompt, &confirm); err != nil {
			return fmt.Errorf("error getting confirmation: %w", err)
		}
		if !confirm {
			fmt.Println("Deletion cancelled")
			return nil
		}
	}

	c, err := newClient(cmd)
	if err != nil {
		return err
	}

	if err := c.DeleteAgent(cmd.Context(), agentID); err != nil {
		return formatAPIError(err, "failed to delete agent")
	}

	fmt.Printf("Successfully deleted agent: %s\n", agentID)
	return nil
}

func resolveInstruction(cmd *cobra.Command) (string, error) {
	instruction, _ := cmd.Flags().GetString("instruction")
	instructionFile, _ := cmd.Flags().GetString("instruction-file")
	if instruction != "" && instructionFile != "" {
		return "", fmt.Errorf("use either --instruction or --instruction-file, not both")
	}
	if instructionFile == "" {
		return instruction, nil
	}

	data, err := os.ReadFile(instructionFile)
	if err != nil {
		return "", fmt.Errorf("failed to read instruction file: %w", err)
	}
	return string(data), nil
}

func resolveTools(cmd *cobra.Command) ([]any, error) {
	toolsJSON, _ := cmd.Flags().GetString("tools")
	toolsFile, _ := cmd.Flags().GetString("tools-file")
	if toolsJSON != "" && toolsFile != "" {
		return nil, fmt.Errorf("use either --tools or --tools-file, not both")
	}
	if toolsFile != "" {
		data, err := os.ReadFile(toolsFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read tools file: %w", err)
		}
		toolsJSON = string(data)
	}
	if strings.TrimSpace(toolsJSON) == "" {
		return []any{}, nil
	}

	var tools []any
	if err := json.Unmarshal([]byte(toolsJSON), &tools); err != nil {
		return nil, fmt.Errorf("tools must be a valid JSON array: %w", err)
	}
	if tools == nil {
		return []any{}, nil
	}
	return tools, nil
}

func newClient(cmd *cobra.Command) (*client.Client, error) {
	cfg, err := util.GetConfig(cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	c, err := client.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}
	return c, nil
}

func formatAPIError(err error, fallback string) error {
	if apiErr, ok := errors.IsAPIError(err); ok {
		return fmt.Errorf("%s", apiErr.GetUserFriendlyMessage())
	}
	return fmt.Errorf("%s: %w", fallback, err)
}
