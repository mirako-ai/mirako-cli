package agent

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/mirako-ai/mirako-cli/internal/client"
	"github.com/mirako-ai/mirako-cli/internal/config"
	"github.com/mirako-ai/mirako-cli/internal/errors"
	"github.com/mirako-ai/mirako-cli/pkg/cmd/util"
	"github.com/mirako-ai/mirako-cli/pkg/ui"
	promptui "github.com/mirako-ai/mirako-cli/pkg/ui/prompt"
	"github.com/mirako-ai/mirako-go/api"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

const (
	managedAgentRuntimeKind        = string(api.CreateAgentInputRuntimeKindManagedAgent)
	customAgentRuntimeKind         = string(api.CreateAgentInputRuntimeKindCustomAgent)
	customAgentProtocolVercelAISDK = string(api.CreateAgentInputCustomAgentProtocolVercelAiSdk)

	managedAgentTypeLabel       = "managed agent"
	managedAgentTypeDescription = "provide prompt/tools and host runtime on Mirako"
	customAgentTypeLabel        = "custom agent"
	customAgentTypeDescription  = "integrate your existing agent endpoint"
	instructionFilePromptLabel  = "Instruction file path (.txt, .md, or .markdown)"
)

type agentPrompter interface {
	Select(message string, options []promptui.SelectOption, defaultValue string) (string, error)
	SearchSelect(message string, options []promptui.SelectOption, defaultValue string) (string, error)
	Input(message string, defaultValue string, required bool) (string, error)
	PathInput(message string, defaultValue string, required bool) (string, error)
	Password(message string) (string, error)
}

type promptAgentPrompter struct {
	prompter *promptui.Prompter
}

var (
	defaultAgentPrompter agentPrompter = promptAgentPrompter{prompter: promptui.NewPrompter()}
	stdinIsTTY                         = func() bool { return term.IsTerminal(int(os.Stdin.Fd())) }
)

func (p promptAgentPrompter) Select(message string, options []promptui.SelectOption, defaultValue string) (string, error) {
	return p.prompts().Select(message, options, defaultValue)
}

func (p promptAgentPrompter) SearchSelect(message string, options []promptui.SelectOption, defaultValue string) (string, error) {
	return p.prompts().SearchSelect(message, options, defaultValue)
}

func (p promptAgentPrompter) Input(message string, defaultValue string, required bool) (string, error) {
	return p.prompts().Input(message, defaultValue, required)
}

func (p promptAgentPrompter) PathInput(message string, defaultValue string, required bool) (string, error) {
	return p.prompts().PathInput(message, defaultValue, required)
}

func (p promptAgentPrompter) Password(message string) (string, error) {
	return p.prompts().Password(message)
}

func (p promptAgentPrompter) prompts() *promptui.Prompter {
	if p.prompter != nil {
		return p.prompter
	}
	return promptui.NewPrompter()
}

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
		return printSafeJSON(resp)
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
			agent.RuntimeKind,
			agent.AvatarId,
			agent.VoiceProfileId,
			agent.Model,
			agent.HasCustomAgentBearerToken,
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
		return printSafeJSON(resp)
	}

	return printAgentDetails(resp.Data)
}

func printAgentDetails(agent api.AgentResponse) error {
	printViewField("ID", agent.Id)
	printViewField("Name", agent.Name)
	if agent.Description != nil && *agent.Description != "" {
		printViewField("Description", *agent.Description)
	}
	printViewField("Avatar ID", agent.AvatarId)
	printViewField("Voice Profile ID", agent.VoiceProfileId)
	printViewField("Model", agent.Model)
	printViewField("Runtime Kind", agent.RuntimeKind)
	printViewField("Custom Agent Bearer Token Configured", fmt.Sprintf("%t", agent.HasCustomAgentBearerToken))

	if agent.RuntimeKind == customAgentRuntimeKind {
		printViewField("Custom Agent URL", optionalString(agent.CustomAgentUrl))
		printViewField("Custom Agent Protocol", optionalString(agent.CustomAgentProtocol))
	}

	printViewField("Created", ui.FormatTimestamp(agent.CreatedAt))
	printViewField("Updated", ui.FormatTimestamp(agent.UpdatedAt))

	if agent.RuntimeKind != customAgentRuntimeKind {
		tools := []any{}
		if agent.Tools != nil {
			tools = *agent.Tools
		}
		toolsJSON, err := json.MarshalIndent(tools, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to format agent tools: %w", err)
		}

		printViewField("Instruction", optionalString(agent.Instruction))
		printViewField("Tools", string(toolsJSON))
	}

	return nil
}

func printViewField(label, value string) {
	fmt.Println(label)
	fmt.Printf("   %s\n\n", formatViewValue(value))
}

func formatViewValue(value string) string {
	if value == "" {
		return ""
	}
	return strings.ReplaceAll(value, "\n", "\n   ")
}

func newCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create an agent",
		Long:  `Create a persistent managed or custom agent configuration using an avatar and voice profile`,
		RunE:  runCreate,
	}

	cmd.Flags().StringP("name", "n", "", "Name for the agent")
	cmd.Flags().StringP("description", "d", "", "Description for the agent")
	cmd.Flags().StringP("avatar", "a", "", "Avatar ID to use")
	cmd.Flags().StringP("voice", "v", "", "Voice profile ID to use")
	cmd.Flags().StringP("model", "m", config.DefaultInteractiveModel, "Interactive model to use")
	cmd.Flags().String("runtime-kind", "", "Runtime kind for the agent (managed_agent or custom_agent)")
	cmd.Flags().StringP("instruction", "i", "", "Instruction prompt text for managed agents (non-interactive)")
	cmd.Flags().String("instruction-file", "", "Path to a .txt, .md, or .markdown file containing the managed-agent instruction prompt")
	cmd.Flags().String("tools", "", "Tools to use for the managed agent (JSON array string)")
	cmd.Flags().String("tools-file", "", "Path to a JSON file containing a managed-agent tools array")
	cmd.Flags().String("custom-agent-url", "", "Custom agent endpoint URL")
	cmd.Flags().String("custom-agent-bearer-token", "", "Bearer token sent to the custom agent endpoint")
	cmd.Flags().String("custom-agent-bearer-token-file", "", "Path to a file containing the custom agent bearer token")
	cmd.Flags().String("custom-agent-protocol", customAgentProtocolVercelAISDK, "Custom agent streaming protocol")
	cmd.Flags().BoolP("json", "j", false, "Output in JSON format")

	return cmd
}

func runCreate(cmd *cobra.Command, args []string) error {
	stdinTTY := stdinIsTTY()
	if runtimeValue := strings.TrimSpace(stringFlag(cmd, "runtime-kind")); runtimeValue != "" {
		if _, err := parseRuntimeKind(runtimeValue); err != nil {
			return err
		}
	}

	var c *client.Client
	var selectionProvider agentSelectionProvider
	if stdinTTY && createNeedsSelectionProvider(cmd) {
		var err error
		c, err = newClient(cmd)
		if err != nil {
			return err
		}
		selectionProvider = apiAgentSelectionProvider{client: c}
	}

	body, err := buildCreateAgentBody(cmd, defaultAgentPrompter, stdinTTY, selectionProvider)
	if err != nil {
		return err
	}

	if c == nil {
		c, err = newClient(cmd)
		if err != nil {
			return err
		}
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
		return printSafeJSON(resp)
	}

	printAgentCreateSuccess(resp.Data)
	return nil
}

func buildCreateAgentBody(cmd *cobra.Command, prompter agentPrompter, stdinTTY bool, selectionProvider agentSelectionProvider) (api.CreateAgentJSONRequestBody, error) {
	if prompter == nil {
		prompter = defaultAgentPrompter
	}

	if runtimeValue := strings.TrimSpace(stringFlag(cmd, "runtime-kind")); runtimeValue != "" {
		if _, err := parseRuntimeKind(runtimeValue); err != nil {
			return api.CreateAgentJSONRequestBody{}, err
		}
	}

	prompt := stdinTTY && createHasMissingRequiredFields(cmd)
	runtimeKind, err := resolveRuntimeKind(cmd, prompter, prompt)
	if err != nil {
		return api.CreateAgentJSONRequestBody{}, err
	}

	name, err := resolveRequiredStringField(cmd, prompter, prompt, "name", "Agent name", "", "name is required. Use --name flag")
	if err != nil {
		return api.CreateAgentJSONRequestBody{}, err
	}
	avatarID, err := resolveAvatarID(cmd, prompter, prompt, selectionProvider)
	if err != nil {
		return api.CreateAgentJSONRequestBody{}, err
	}
	voiceID, err := resolveVoiceProfileID(cmd, prompter, prompt, selectionProvider)
	if err != nil {
		return api.CreateAgentJSONRequestBody{}, err
	}

	description := stringFlag(cmd, "description")
	if prompt && !flagChanged(cmd, "description") {
		description, err = prompter.Input("Description (optional)", description, false)
		if err != nil {
			return api.CreateAgentJSONRequestBody{}, fmt.Errorf("error getting description: %w", err)
		}
	}
	description = strings.TrimSpace(description)

	model := strings.TrimSpace(stringFlag(cmd, "model"))
	if prompt && !flagChanged(cmd, "model") {
		model, err = prompter.Input("Interactive model", defaultIfEmpty(model, config.DefaultInteractiveModel), true)
		if err != nil {
			return api.CreateAgentJSONRequestBody{}, fmt.Errorf("error getting interactive model: %w", err)
		}
		model = strings.TrimSpace(model)
	}

	body := api.CreateAgentJSONRequestBody{
		Name:           name,
		AvatarId:       avatarID,
		VoiceProfileId: voiceID,
		RuntimeKind:    &runtimeKind,
	}
	if description != "" {
		body.Description = &description
	}
	if model != "" {
		body.Model = &model
	}

	switch runtimeKind {
	case api.CreateAgentInputRuntimeKindManagedAgent:
		if err := populateManagedAgentCreateBody(cmd, prompter, prompt, &body); err != nil {
			return api.CreateAgentJSONRequestBody{}, err
		}
	case api.CreateAgentInputRuntimeKindCustomAgent:
		if err := populateCustomAgentCreateBody(cmd, prompter, prompt, &body); err != nil {
			return api.CreateAgentJSONRequestBody{}, err
		}
	default:
		return api.CreateAgentJSONRequestBody{}, fmt.Errorf("runtime kind must be one of %s, %s", managedAgentRuntimeKind, customAgentRuntimeKind)
	}

	return body, nil
}

func populateManagedAgentCreateBody(cmd *cobra.Command, prompter agentPrompter, prompt bool, body *api.CreateAgentJSONRequestBody) error {
	instruction, err := resolveInstructionWithPrompt(cmd, prompter, prompt)
	if err != nil {
		return err
	}
	if strings.TrimSpace(instruction) == "" {
		return fmt.Errorf("instruction is required. Use --instruction or --instruction-file")
	}

	tools, err := resolveToolsWithPrompt(cmd, prompter, prompt)
	if err != nil {
		return err
	}

	body.Instruction = &instruction
	body.Tools = &tools
	return nil
}

func populateCustomAgentCreateBody(cmd *cobra.Command, prompter agentPrompter, prompt bool, body *api.CreateAgentJSONRequestBody) error {
	customAgentURL := strings.TrimSpace(stringFlag(cmd, "custom-agent-url"))
	if prompt && customAgentURL == "" {
		answer, err := prompter.Input("Custom agent URL", "", true)
		if err != nil {
			return fmt.Errorf("error getting custom agent URL: %w", err)
		}
		customAgentURL = strings.TrimSpace(answer)
	}
	if customAgentURL == "" {
		return fmt.Errorf("custom agent URL is required. Use --custom-agent-url flag")
	}
	if err := validateCustomAgentURL(customAgentURL); err != nil {
		return err
	}

	bearerToken, err := resolveCustomAgentBearerToken(cmd, prompter, prompt)
	if err != nil {
		return err
	}

	protocol := strings.TrimSpace(stringFlag(cmd, "custom-agent-protocol"))
	if protocol == "" {
		protocol = customAgentProtocolVercelAISDK
	}
	if prompt && !flagChanged(cmd, "custom-agent-protocol") {
		answer, err := prompter.Input("Custom agent protocol", protocol, true)
		if err != nil {
			return fmt.Errorf("error getting custom agent protocol: %w", err)
		}
		protocol = strings.TrimSpace(answer)
	}
	customProtocol, err := parseCustomAgentProtocol(protocol)
	if err != nil {
		return err
	}

	body.CustomAgentUrl = &customAgentURL
	body.CustomAgentProtocol = &customProtocol
	if bearerToken != "" {
		body.CustomAgentBearerToken = &bearerToken
	}
	return nil
}

func resolveRuntimeKind(cmd *cobra.Command, prompter agentPrompter, prompt bool) (api.CreateAgentInputRuntimeKind, error) {
	runtimeValue := strings.TrimSpace(stringFlag(cmd, "runtime-kind"))
	if runtimeValue != "" {
		return parseRuntimeKind(runtimeValue)
	}

	if prompt {
		defaultChoice := managedAgentRuntimeKind
		if hasCustomAgentFlagValues(cmd) {
			defaultChoice = customAgentRuntimeKind
		}
		choice, err := prompter.Select("Agent type", agentTypePromptOptions(), defaultChoice)
		if err != nil {
			return "", fmt.Errorf("error getting agent type: %w", err)
		}
		return runtimeKindFromChoice(choice)
	}

	if hasCustomAgentFlagValues(cmd) {
		return api.CreateAgentInputRuntimeKindCustomAgent, nil
	}
	return api.CreateAgentInputRuntimeKindManagedAgent, nil
}

func parseRuntimeKind(value string) (api.CreateAgentInputRuntimeKind, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case managedAgentRuntimeKind:
		return api.CreateAgentInputRuntimeKindManagedAgent, nil
	case customAgentRuntimeKind:
		return api.CreateAgentInputRuntimeKindCustomAgent, nil
	default:
		return "", fmt.Errorf("runtime kind must be one of %s, %s", managedAgentRuntimeKind, customAgentRuntimeKind)
	}
}

func agentTypePromptOptions() []promptui.SelectOption {
	return []promptui.SelectOption{
		{
			Label:       managedAgentTypeLabel,
			Description: managedAgentTypeDescription,
			Value:       managedAgentRuntimeKind,
		},
		{
			Label:       customAgentTypeLabel,
			Description: customAgentTypeDescription,
			Value:       customAgentRuntimeKind,
		},
	}
}

func runtimeKindFromChoice(choice string) (api.CreateAgentInputRuntimeKind, error) {
	choice = strings.TrimSpace(choice)
	for _, option := range agentTypePromptOptions() {
		if choice == option.Value || choice == option.Label || choice == fmt.Sprintf("%s - %s", option.Label, option.Description) {
			return parseRuntimeKind(option.Value)
		}
	}
	return "", fmt.Errorf("unsupported agent type %q", choice)
}

func parseCustomAgentProtocol(value string) (api.CreateAgentInputCustomAgentProtocol, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case customAgentProtocolVercelAISDK:
		return api.CreateAgentInputCustomAgentProtocolVercelAiSdk, nil
	default:
		return "", fmt.Errorf("custom agent protocol must be one of %s", customAgentProtocolVercelAISDK)
	}
}

func validateCustomAgentURL(value string) error {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("custom agent URL must be a valid absolute URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("custom agent URL must use http or https")
	}
	return nil
}

func createHasMissingRequiredFields(cmd *cobra.Command) bool {
	if strings.TrimSpace(stringFlag(cmd, "name")) == "" || createNeedsSelectionProvider(cmd) {
		return true
	}

	runtimeKind := api.CreateAgentInputRuntimeKindManagedAgent
	if runtimeValue := strings.TrimSpace(stringFlag(cmd, "runtime-kind")); runtimeValue != "" {
		parsed, err := parseRuntimeKind(runtimeValue)
		if err != nil {
			return false
		}
		runtimeKind = parsed
	} else if hasCustomAgentFlagValues(cmd) {
		runtimeKind = api.CreateAgentInputRuntimeKindCustomAgent
	}

	switch runtimeKind {
	case api.CreateAgentInputRuntimeKindManagedAgent:
		return strings.TrimSpace(stringFlag(cmd, "instruction")) == "" && strings.TrimSpace(stringFlag(cmd, "instruction-file")) == ""
	case api.CreateAgentInputRuntimeKindCustomAgent:
		return strings.TrimSpace(stringFlag(cmd, "custom-agent-url")) == ""
	default:
		return false
	}
}

func createNeedsSelectionProvider(cmd *cobra.Command) bool {
	return strings.TrimSpace(stringFlag(cmd, "avatar")) == "" || strings.TrimSpace(stringFlag(cmd, "voice")) == ""
}

func resolveRequiredStringField(cmd *cobra.Command, prompter agentPrompter, prompt bool, flagName string, promptMessage string, defaultValue string, errorMessage string) (string, error) {
	value := strings.TrimSpace(stringFlag(cmd, flagName))
	if prompt && value == "" {
		answer, err := prompter.Input(promptMessage, defaultValue, true)
		if err != nil {
			return "", fmt.Errorf("error getting %s: %w", strings.ToLower(promptMessage), err)
		}
		value = strings.TrimSpace(answer)
	}
	if value == "" {
		return "", fmt.Errorf("%s", errorMessage)
	}
	return value, nil
}

func resolveAvatarID(cmd *cobra.Command, prompter agentPrompter, prompt bool, selectionProvider agentSelectionProvider) (string, error) {
	value := strings.TrimSpace(stringFlag(cmd, "avatar"))
	if value != "" {
		return value, nil
	}
	if !prompt {
		return "", fmt.Errorf("avatar ID is required. Use --avatar flag")
	}
	if selectionProvider == nil {
		return "", fmt.Errorf("avatar selector is unavailable")
	}

	options, err := selectionProvider.AvatarOptions(cmd.Context())
	if err != nil {
		return "", formatAPIError(err, "failed to list avatars")
	}
	if len(options) == 0 {
		return "", fmt.Errorf("no READY avatars found. Create or finish building an avatar with 'mirako avatar build' or 'mirako avatar generate'")
	}

	choice, err := prompter.SearchSelect("Choose avatar", options, "")
	if err != nil {
		return "", fmt.Errorf("error choosing avatar: %w", err)
	}
	choice = strings.TrimSpace(choice)
	if choice == "" {
		return "", fmt.Errorf("avatar ID is required. Use --avatar flag")
	}
	return choice, nil
}

func resolveVoiceProfileID(cmd *cobra.Command, prompter agentPrompter, prompt bool, selectionProvider agentSelectionProvider) (string, error) {
	value := strings.TrimSpace(stringFlag(cmd, "voice"))
	if value != "" {
		return value, nil
	}
	if !prompt {
		return "", fmt.Errorf("voice profile ID is required. Use --voice flag")
	}
	if selectionProvider == nil {
		return "", fmt.Errorf("voice profile selector is unavailable")
	}

	options, err := selectionProvider.VoiceProfileOptions(cmd.Context())
	if err != nil {
		return "", formatAPIError(err, "failed to list voice profiles")
	}
	if len(options) == 0 {
		return "", fmt.Errorf("no voice profiles found")
	}

	choice, err := prompter.SearchSelect("Choose voice profile", options, "")
	if err != nil {
		return "", fmt.Errorf("error choosing voice profile: %w", err)
	}
	choice = strings.TrimSpace(choice)
	if choice == "" {
		return "", fmt.Errorf("voice profile ID is required. Use --voice flag")
	}
	return choice, nil
}

func resolveCustomAgentBearerToken(cmd *cobra.Command, prompter agentPrompter, prompt bool) (string, error) {
	bearerToken := strings.TrimSpace(stringFlag(cmd, "custom-agent-bearer-token"))
	bearerTokenFile := strings.TrimSpace(stringFlag(cmd, "custom-agent-bearer-token-file"))
	if bearerToken != "" && bearerTokenFile != "" {
		return "", fmt.Errorf("use either --custom-agent-bearer-token or --custom-agent-bearer-token-file, not both")
	}
	if bearerTokenFile != "" {
		data, err := os.ReadFile(bearerTokenFile)
		if err != nil {
			return "", fmt.Errorf("failed to read custom agent bearer token file: %w", err)
		}
		return strings.TrimSpace(string(data)), nil
	}
	if bearerToken != "" {
		return bearerToken, nil
	}
	if prompt && !flagChanged(cmd, "custom-agent-bearer-token") && !flagChanged(cmd, "custom-agent-bearer-token-file") {
		answer, err := prompter.Password("Custom agent bearer token (optional)")
		if err != nil {
			return "", fmt.Errorf("error getting custom agent bearer token: %w", err)
		}
		return strings.TrimSpace(answer), nil
	}
	return "", nil
}

func hasCustomAgentFlagValues(cmd *cobra.Command) bool {
	return strings.TrimSpace(stringFlag(cmd, "custom-agent-url")) != "" ||
		strings.TrimSpace(stringFlag(cmd, "custom-agent-bearer-token")) != "" ||
		strings.TrimSpace(stringFlag(cmd, "custom-agent-bearer-token-file")) != "" ||
		flagChanged(cmd, "custom-agent-protocol")
}

func printAgentCreateSuccess(agent api.AgentResponse) {
	fmt.Printf("Agent created successfully. Use this agent ID for interactive.\n\n")
	fmt.Printf("   %s\n\n", agent.Id)
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
	return resolveInstructionWithPrompt(cmd, nil, false)
}

func resolveInstructionWithPrompt(cmd *cobra.Command, prompter agentPrompter, prompt bool) (string, error) {
	instruction, _ := cmd.Flags().GetString("instruction")
	instructionFile := strings.TrimSpace(stringFlag(cmd, "instruction-file"))
	if strings.TrimSpace(instruction) != "" && instructionFile != "" {
		return "", fmt.Errorf("use either --instruction or --instruction-file, not both")
	}
	if flagChanged(cmd, "instruction-file") && instructionFile == "" {
		return "", fmt.Errorf("instruction file path is required")
	}
	if instructionFile != "" {
		return readInstructionFile(instructionFile)
	}
	if prompt && strings.TrimSpace(instruction) == "" && !flagChanged(cmd, "instruction") && !flagChanged(cmd, "instruction-file") {
		if prompter == nil {
			prompter = defaultAgentPrompter
		}
		answer, err := prompter.PathInput(instructionFilePromptLabel, "", true)
		if err != nil {
			return "", fmt.Errorf("error getting instruction file path: %w", err)
		}
		return readInstructionFile(answer)
	}
	return instruction, nil
}

func readInstructionFile(path string) (string, error) {
	displayPath := strings.TrimSpace(path)
	if displayPath == "" {
		return "", fmt.Errorf("instruction file path is required")
	}
	resolvedPath, err := expandInstructionFilePath(displayPath)
	if err != nil {
		return "", fmt.Errorf("failed to read instruction file %q: %w", displayPath, err)
	}
	info, err := os.Stat(resolvedPath)
	if err != nil {
		return "", fmt.Errorf("failed to read instruction file %q: %w", displayPath, err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("failed to read instruction file %q: path must point to a file, not a directory", displayPath)
	}
	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		return "", fmt.Errorf("failed to read instruction file %q: %w", displayPath, err)
	}
	instruction := string(data)
	if strings.TrimSpace(instruction) == "" {
		return "", fmt.Errorf("instruction file is empty: %s", displayPath)
	}
	return instruction, nil
}

func expandInstructionFilePath(path string) (string, error) {
	if path == "~" {
		return os.UserHomeDir()
	}
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, path[2:]), nil
	}
	if os.PathSeparator != '/' && strings.HasPrefix(path, "~"+string(os.PathSeparator)) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, path[2:]), nil
	}
	return path, nil
}

func resolveTools(cmd *cobra.Command) ([]any, error) {
	return resolveToolsWithPrompt(cmd, nil, false)
}

func resolveToolsWithPrompt(cmd *cobra.Command, prompter agentPrompter, prompt bool) ([]any, error) {
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
	if prompt && strings.TrimSpace(toolsJSON) == "" && !flagChanged(cmd, "tools") && !flagChanged(cmd, "tools-file") {
		if prompter == nil {
			prompter = defaultAgentPrompter
		}
		answer, err := prompter.Input("Tools JSON array (optional)", "", false)
		if err != nil {
			return nil, fmt.Errorf("error getting tools: %w", err)
		}
		toolsJSON = answer
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

func printSafeJSON(value any) error {
	sanitized, err := sanitizedJSONValue(value)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(sanitized, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to format JSON output: %w", err)
	}
	fmt.Println(string(data))
	return nil
}

func sanitizedJSONValue(value any) (any, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("failed to format JSON output: %w", err)
	}
	var sanitized any
	if err := json.Unmarshal(data, &sanitized); err != nil {
		return nil, fmt.Errorf("failed to sanitize JSON output: %w", err)
	}
	removeSecretFields(sanitized)
	return sanitized, nil
}

func removeSecretFields(value any) {
	switch typed := value.(type) {
	case map[string]any:
		delete(typed, "custom_agent_bearer_token")
		for _, child := range typed {
			removeSecretFields(child)
		}
	case []any:
		for _, child := range typed {
			removeSecretFields(child)
		}
	}
}

func optionalString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func stringFlag(cmd *cobra.Command, name string) string {
	value, _ := cmd.Flags().GetString(name)
	return value
}

func flagChanged(cmd *cobra.Command, name string) bool {
	flag := cmd.Flags().Lookup(name)
	return flag != nil && flag.Changed
}

func defaultIfEmpty(value, defaultValue string) string {
	if strings.TrimSpace(value) == "" {
		return defaultValue
	}
	return value
}

func formatAPIError(err error, fallback string) error {
	if apiErr, ok := errors.IsAPIError(err); ok {
		return fmt.Errorf("%s", apiErr.GetUserFriendlyMessage())
	}
	return fmt.Errorf("%s: %w", fallback, err)
}
