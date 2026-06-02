package agent

import (
	"encoding/json"
	"fmt"
	"net/url"
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
	"golang.org/x/term"
)

const (
	managedAgentRuntimeKind        = string(api.CreateAgentInputRuntimeKindManagedAgent)
	customAgentRuntimeKind         = string(api.CreateAgentInputRuntimeKindCustomAgent)
	customAgentProtocolVercelAISDK = string(api.CreateAgentInputCustomAgentProtocolVercelAiSdk)

	managedAgentTypeChoice = "managed agent - provide prompt, model/tools and host runtime on Mirako"
	customAgentTypeChoice  = "custom agent - integrate your existing agent endpoint"
)

type agentPrompter interface {
	Select(message string, options []string, defaultValue string) (string, error)
	Input(message string, defaultValue string, required bool) (string, error)
	Multiline(message string, defaultValue string, required bool) (string, error)
	Password(message string) (string, error)
}

type surveyAgentPrompter struct{}

var (
	defaultAgentPrompter agentPrompter = surveyAgentPrompter{}
	stdinIsTTY                         = func() bool { return term.IsTerminal(int(os.Stdin.Fd())) }
)

func (surveyAgentPrompter) Select(message string, options []string, defaultValue string) (string, error) {
	var answer string
	prompt := &survey.Select{Message: message, Options: options, Default: defaultValue}
	if err := survey.AskOne(prompt, &answer); err != nil {
		return "", err
	}
	return answer, nil
}

func (surveyAgentPrompter) Input(message string, defaultValue string, required bool) (string, error) {
	var answer string
	prompt := &survey.Input{Message: message, Default: defaultValue}
	if err := survey.AskOne(prompt, &answer, surveyValidators(required)...); err != nil {
		return "", err
	}
	return answer, nil
}

func (surveyAgentPrompter) Multiline(message string, defaultValue string, required bool) (string, error) {
	var answer string
	prompt := &survey.Multiline{Message: message, Default: defaultValue}
	if err := survey.AskOne(prompt, &answer, surveyValidators(required)...); err != nil {
		return "", err
	}
	return answer, nil
}

func (surveyAgentPrompter) Password(message string) (string, error) {
	var answer string
	prompt := &survey.Password{Message: message}
	if err := survey.AskOne(prompt, &answer); err != nil {
		return "", err
	}
	return answer, nil
}

func surveyValidators(required bool) []survey.AskOpt {
	if !required {
		return nil
	}
	return []survey.AskOpt{survey.WithValidator(survey.Required)}
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
			optionalString(agent.LlmModel),
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
	fmt.Printf("ID: %s\n", agent.Id)
	fmt.Printf("Name: %s\n", agent.Name)
	if agent.Description != nil && *agent.Description != "" {
		fmt.Printf("Description: %s\n", *agent.Description)
	}
	fmt.Printf("Avatar ID: %s\n", agent.AvatarId)
	fmt.Printf("Voice Profile ID: %s\n", agent.VoiceProfileId)
	fmt.Printf("Model: %s\n", agent.Model)
	fmt.Printf("Runtime Kind: %s\n", agent.RuntimeKind)
	fmt.Printf("Custom Agent Bearer Token Configured: %t\n", agent.HasCustomAgentBearerToken)

	if agent.RuntimeKind == customAgentRuntimeKind || agent.CustomAgentUrl != nil || agent.CustomAgentProtocol != nil {
		fmt.Printf("Custom Agent URL: %s\n", optionalString(agent.CustomAgentUrl))
		fmt.Printf("Custom Agent Protocol: %s\n", optionalString(agent.CustomAgentProtocol))
	} else {
		fmt.Printf("LLM Model: %s\n", optionalString(agent.LlmModel))
	}

	fmt.Printf("Created: %s\n", ui.FormatTimestamp(agent.CreatedAt))
	fmt.Printf("Updated: %s\n", ui.FormatTimestamp(agent.UpdatedAt))

	if agent.RuntimeKind != customAgentRuntimeKind {
		tools := []any{}
		if agent.Tools != nil {
			tools = *agent.Tools
		}
		toolsJSON, err := json.MarshalIndent(tools, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to format agent tools: %w", err)
		}

		fmt.Printf("\nInstruction:\n%s\n", optionalString(agent.Instruction))
		fmt.Printf("\nTools:\n%s\n", string(toolsJSON))
	}

	return nil
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
	cmd.Flags().StringP("llm-model", "l", "", "LLM model to use for managed agents")
	cmd.Flags().StringP("instruction", "i", "", "Instruction prompt text for managed agents")
	cmd.Flags().String("instruction-file", "", "Path to a text or Markdown file containing the managed-agent instruction prompt")
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
	body, err := buildCreateAgentBody(cmd, defaultAgentPrompter, stdinIsTTY())
	if err != nil {
		return err
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
		return printSafeJSON(resp)
	}

	printAgentCreateSuccess(resp.Data)
	return nil
}

func buildCreateAgentBody(cmd *cobra.Command, prompter agentPrompter, stdinTTY bool) (api.CreateAgentJSONRequestBody, error) {
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
	avatarID, err := resolveRequiredStringField(cmd, prompter, prompt, "avatar", "Avatar ID", "", "avatar ID is required. Use --avatar flag")
	if err != nil {
		return api.CreateAgentJSONRequestBody{}, err
	}
	voiceID, err := resolveRequiredStringField(cmd, prompter, prompt, "voice", "Voice profile ID", "", "voice profile ID is required. Use --voice flag")
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
	llmModel := strings.TrimSpace(stringFlag(cmd, "llm-model"))
	if prompt && llmModel == "" {
		answer, err := prompter.Input("LLM model", config.DefaultLLMModel, true)
		if err != nil {
			return fmt.Errorf("error getting LLM model: %w", err)
		}
		llmModel = strings.TrimSpace(answer)
	}
	if llmModel == "" {
		return fmt.Errorf("LLM model is required. Use --llm-model flag")
	}

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

	body.LlmModel = &llmModel
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
		defaultChoice := managedAgentTypeChoice
		if hasCustomAgentFlagValues(cmd) {
			defaultChoice = customAgentTypeChoice
		}
		choice, err := prompter.Select("Agent type", []string{managedAgentTypeChoice, customAgentTypeChoice}, defaultChoice)
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

func runtimeKindFromChoice(choice string) (api.CreateAgentInputRuntimeKind, error) {
	switch choice {
	case managedAgentTypeChoice:
		return api.CreateAgentInputRuntimeKindManagedAgent, nil
	case customAgentTypeChoice:
		return api.CreateAgentInputRuntimeKindCustomAgent, nil
	case managedAgentRuntimeKind, customAgentRuntimeKind:
		return parseRuntimeKind(choice)
	default:
		return "", fmt.Errorf("unsupported agent type %q", choice)
	}
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
	if strings.TrimSpace(stringFlag(cmd, "name")) == "" || strings.TrimSpace(stringFlag(cmd, "avatar")) == "" || strings.TrimSpace(stringFlag(cmd, "voice")) == "" {
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
		return strings.TrimSpace(stringFlag(cmd, "llm-model")) == "" ||
			(strings.TrimSpace(stringFlag(cmd, "instruction")) == "" && strings.TrimSpace(stringFlag(cmd, "instruction-file")) == "")
	case api.CreateAgentInputRuntimeKindCustomAgent:
		return strings.TrimSpace(stringFlag(cmd, "custom-agent-url")) == ""
	default:
		return false
	}
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
	fmt.Printf("Agent created successfully!\n")
	fmt.Printf("   Agent ID: %s\n", agent.Id)
	fmt.Printf("   Name: %s\n", agent.Name)
	fmt.Printf("   Avatar ID: %s\n", agent.AvatarId)
	fmt.Printf("   Voice Profile ID: %s\n", agent.VoiceProfileId)
	fmt.Printf("   Model: %s\n", agent.Model)
	fmt.Printf("   Runtime Kind: %s\n", agent.RuntimeKind)
	fmt.Printf("   Custom Agent Bearer Token Configured: %t\n", agent.HasCustomAgentBearerToken)
	if agent.RuntimeKind == customAgentRuntimeKind || agent.CustomAgentUrl != nil || agent.CustomAgentProtocol != nil {
		fmt.Printf("   Custom Agent URL: %s\n", optionalString(agent.CustomAgentUrl))
		fmt.Printf("   Custom Agent Protocol: %s\n", optionalString(agent.CustomAgentProtocol))
	} else {
		fmt.Printf("   LLM Model: %s\n", optionalString(agent.LlmModel))
	}
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
	instructionFile, _ := cmd.Flags().GetString("instruction-file")
	if instruction != "" && instructionFile != "" {
		return "", fmt.Errorf("use either --instruction or --instruction-file, not both")
	}
	if instructionFile != "" {
		data, err := os.ReadFile(instructionFile)
		if err != nil {
			return "", fmt.Errorf("failed to read instruction file: %w", err)
		}
		return string(data), nil
	}
	if prompt && strings.TrimSpace(instruction) == "" && !flagChanged(cmd, "instruction") && !flagChanged(cmd, "instruction-file") {
		if prompter == nil {
			prompter = defaultAgentPrompter
		}
		answer, err := prompter.Multiline("Instruction prompt", "", true)
		if err != nil {
			return "", fmt.Errorf("error getting instruction: %w", err)
		}
		return answer, nil
	}
	return instruction, nil
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
