package agent

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/mirako-ai/mirako-cli/pkg/ui"
	"github.com/mirako-ai/mirako-go/api"
	"github.com/spf13/cobra"
)

const maxAgentRouteLabelLength = 100

func newRoutesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "routes",
		Short: "Manage agent routes",
		Long:  `Manage bearer-capability routes for persistent agents`,
	}

	cmd.AddCommand(newRoutesCreateCmd())
	return cmd
}

func newRoutesCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create [agent-id]",
		Short: "Create an agent route",
		Long:  `Create a permanent or temporary bearer-capability route for an owned agent`,
		Args:  cobra.ExactArgs(1),
		RunE:  runRoutesCreate,
	}

	cmd.Flags().String("label", "", "Optional route label (maximum 100 characters)")
	cmd.Flags().String("valid-for", "", "Route validity duration, for example 24h (omit for permanent)")
	cmd.Flags().BoolP("json", "j", false, "Output in JSON format")

	return cmd
}

func runRoutesCreate(cmd *cobra.Command, args []string) error {
	agentID := strings.TrimSpace(args[0])
	if agentID == "" {
		return fmt.Errorf("agent ID is required")
	}

	body, err := buildCreateAgentRouteBody(cmd)
	if err != nil {
		return err
	}

	c, err := newClient(cmd)
	if err != nil {
		return err
	}

	resp, err := c.CreateAgentRoute(cmd.Context(), agentID, body)
	if err != nil {
		return formatAPIError(err, "failed to create agent route")
	}
	if resp == nil {
		return fmt.Errorf("unexpected response from server")
	}
	if err := validateCreatedAgentRoute(resp.Data, agentID, body); err != nil {
		return err
	}

	useJSON, _ := cmd.Flags().GetBool("json")
	if useJSON {
		return printSafeJSON(resp)
	}

	printAgentRouteCreateSuccess(resp.Data)
	return nil
}

func buildCreateAgentRouteBody(cmd *cobra.Command) (api.CreateAgentRouteJSONRequestBody, error) {
	body := api.CreateAgentRouteJSONRequestBody{}

	label := strings.TrimSpace(stringFlag(cmd, "label"))
	if utf8.RuneCountInString(label) > maxAgentRouteLabelLength {
		return body, fmt.Errorf("label must be at most %d characters", maxAgentRouteLabelLength)
	}
	if label != "" {
		body.Label = &label
	}

	validitySeconds, err := parseValidFor(stringFlag(cmd, "valid-for"))
	if err != nil {
		return body, err
	}
	body.ValiditySeconds = validitySeconds

	return body, nil
}

func parseValidFor(value string) (*int64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}

	duration, err := time.ParseDuration(value)
	if err != nil {
		return nil, fmt.Errorf("invalid --valid-for %q: %w", value, err)
	}
	if duration <= 0 {
		return nil, fmt.Errorf("--valid-for must be a positive duration")
	}
	if duration%time.Second != 0 {
		return nil, fmt.Errorf("--valid-for must be a whole number of seconds")
	}

	seconds := int64(duration / time.Second)
	return &seconds, nil
}

func validateCreatedAgentRoute(route api.AgentRouteResponse, agentID string, request api.CreateAgentRouteJSONRequestBody) error {
	switch {
	case strings.TrimSpace(route.Id) == "":
		return fmt.Errorf("unexpected response from server: created route is missing its ID")
	case route.AgentId != agentID:
		return fmt.Errorf("unexpected response from server: created route has the wrong agent ID")
	case strings.TrimSpace(route.Path) == "":
		return fmt.Errorf("unexpected response from server: created route is missing its path")
	case route.Status != api.Active:
		return fmt.Errorf("unexpected response from server: created route is not active")
	case route.RevokedAt != nil:
		return fmt.Errorf("unexpected response from server: created route is revoked")
	case route.RouteVersion <= 0:
		return fmt.Errorf("unexpected response from server: created route has an invalid version")
	case route.CreatedAt.IsZero() || route.UpdatedAt.IsZero():
		return fmt.Errorf("unexpected response from server: created route is missing lifecycle timestamps")
	case (request.ValiditySeconds == nil) != (route.ExpiresAt == nil):
		return fmt.Errorf("unexpected response from server: created route has inconsistent expiration")
	default:
		return nil
	}
}

func printAgentRouteCreateSuccess(route api.AgentRouteResponse) {
	fmt.Println("Agent route created successfully.")
	fmt.Println()
	printViewField("Route ID", route.Id)
	printViewField("Agent ID", route.AgentId)
	if route.Url != nil && *route.Url != "" {
		printViewField("URL", *route.Url)
	}
	printViewField("Path", route.Path)
	if route.Label != nil {
		printViewField("Label", *route.Label)
	}
	printViewField("Status", string(route.Status))
	printViewField("Expires", formatRouteTimestamp(route.ExpiresAt, "Never"))
	printViewField("Revoked", formatRouteTimestamp(route.RevokedAt, "Not revoked"))
	printViewField("Route Version", fmt.Sprintf("%d", route.RouteVersion))
	printViewField("Created", ui.FormatTimestamp(route.CreatedAt))
	printViewField("Updated", ui.FormatTimestamp(route.UpdatedAt))
}

func formatRouteTimestamp(value *time.Time, fallback string) string {
	if value == nil {
		return fallback
	}
	return ui.FormatTimestamp(*value)
}
