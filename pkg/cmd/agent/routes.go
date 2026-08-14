package agent

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/AlecAivazis/survey/v2"
	apierrors "github.com/mirako-ai/mirako-cli/internal/errors"
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

	cmd.AddCommand(newRoutesListCmd())
	cmd.AddCommand(newRoutesViewCmd())
	cmd.AddCommand(newRoutesCreateCmd())
	cmd.AddCommand(newRoutesRevokeCmd())
	return cmd
}

func newRoutesListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List agent routes",
		Long:  `List all bearer-capability routes owned by the authenticated user across agents`,
		Args:  cobra.NoArgs,
		RunE:  runRoutesList,
	}
	cmd.Flags().BoolP("json", "j", false, "Output in JSON format")
	return cmd
}

func runRoutesList(cmd *cobra.Command, _ []string) error {
	c, err := newClient(cmd)
	if err != nil {
		return err
	}
	resp, err := c.ListOwnerAgentRoutes(cmd.Context())
	if err != nil {
		return formatAgentRouteListAPIError(err, "failed to list agent routes")
	}
	if resp == nil {
		return fmt.Errorf("unexpected response from server")
	}

	if resp.Data != nil {
		for _, route := range *resp.Data {
			if err := validateAgentRoute(route); err != nil {
				return err
			}
		}
	}

	useJSON, _ := cmd.Flags().GetBool("json")
	if useJSON {
		return printSafeJSON(resp)
	}
	if resp.Data == nil || len(*resp.Data) == 0 {
		fmt.Println("No agent routes found")
		return nil
	}

	table := ui.NewAgentRouteTable(os.Stdout)
	for _, route := range *resp.Data {
		table.AddRow([]interface{}{
			sanitizeAgentRouteOutput(optionalString(route.Label)),
			sanitizeAgentRouteOutput(route.AgentId),
			sanitizeAgentRouteOutput(route.Id),
			route.Status,
			formatRouteTimestamp(route.ExpiresAt, "Never"),
			ui.FormatTimestamp(route.UpdatedAt),
		})
	}
	return table.Flush()
}

func newRoutesViewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "view [route-id]",
		Short: "View an agent route",
		Long:  `View lifecycle and capability details for an owned agent route`,
		Args:  cobra.ExactArgs(1),
		RunE:  runRoutesView,
	}
	cmd.Flags().BoolP("json", "j", false, "Output in JSON format")
	return cmd
}

func runRoutesView(cmd *cobra.Command, args []string) error {
	routeID := strings.TrimSpace(args[0])
	if routeID == "" {
		return fmt.Errorf("route ID is required")
	}

	c, err := newClient(cmd)
	if err != nil {
		return err
	}
	resp, err := c.GetAgentRoute(cmd.Context(), routeID)
	if err != nil {
		return formatAgentRouteAPIError(err, "failed to get agent route", routeID)
	}
	if resp == nil {
		return fmt.Errorf("unexpected response from server")
	}
	if err := validateAgentRoute(resp.Data); err != nil {
		return err
	}
	if resp.Data.Id != routeID {
		return fmt.Errorf("unexpected response from server: agent route has the wrong route ID")
	}

	useJSON, _ := cmd.Flags().GetBool("json")
	if useJSON {
		return printSafeJSON(resp)
	}
	printAgentRouteDetails(resp.Data)
	return nil
}

func newRoutesRevokeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "revoke [route-id]",
		Short: "Revoke an agent route",
		Long:  `Terminally and idempotently revoke an owned agent route`,
		Args:  cobra.ExactArgs(1),
		RunE:  runRoutesRevoke,
	}
	cmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")
	cmd.Flags().BoolP("json", "j", false, "Output in JSON format")
	return cmd
}

func runRoutesRevoke(cmd *cobra.Command, args []string) error {
	routeID := strings.TrimSpace(args[0])
	if routeID == "" {
		return fmt.Errorf("route ID is required")
	}

	force, _ := cmd.Flags().GetBool("force")
	useJSON, _ := cmd.Flags().GetBool("json")
	if useJSON && !force {
		return fmt.Errorf("--json requires --force for route revocation")
	}
	if !force {
		confirmed := false
		prompt := &survey.Confirm{
			Message: fmt.Sprintf("Revoke agent route %s? This permanently disables the route.", sanitizeAgentRouteOutput(routeID)),
			Default: false,
		}
		if err := survey.AskOne(prompt, &confirmed); err != nil {
			return fmt.Errorf("error getting confirmation: %w", err)
		}
		if !confirmed {
			fmt.Println("Revocation cancelled")
			return nil
		}
	}

	c, err := newClient(cmd)
	if err != nil {
		return err
	}
	resp, err := c.RevokeAgentRoute(cmd.Context(), routeID)
	if err != nil {
		return formatAgentRouteAPIError(err, "failed to revoke agent route", routeID)
	}
	if resp == nil {
		return fmt.Errorf("unexpected response from server")
	}
	if err := validateAgentRoute(resp.Data); err != nil {
		return err
	}
	if resp.Data.Id != routeID {
		return fmt.Errorf("unexpected response from server: agent route has the wrong route ID")
	}
	if resp.Data.Status != api.Revoked || resp.Data.RevokedAt == nil {
		return fmt.Errorf("unexpected response from server: revoked route has inconsistent lifecycle state")
	}

	if useJSON {
		return printSafeJSON(resp)
	}
	fmt.Println("Agent route revoked successfully.")
	fmt.Println()
	printAgentRouteDetails(resp.Data)
	return nil
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
		return validateAgentRoute(route)
	}
}

func validateAgentRoute(route api.AgentRouteResponse) error {
	if strings.TrimSpace(route.Id) == "" {
		return fmt.Errorf("unexpected response from server: agent route is missing its ID")
	}
	if strings.TrimSpace(route.AgentId) == "" {
		return fmt.Errorf("unexpected response from server: agent route is missing its agent ID")
	}
	if strings.TrimSpace(route.Path) == "" {
		return fmt.Errorf("unexpected response from server: agent route is missing its path")
	}
	if route.RouteVersion <= 0 {
		return fmt.Errorf("unexpected response from server: agent route has an invalid version")
	}
	if route.CreatedAt.IsZero() || route.UpdatedAt.IsZero() {
		return fmt.Errorf("unexpected response from server: agent route is missing lifecycle timestamps")
	}
	if route.UpdatedAt.Before(route.CreatedAt) {
		return fmt.Errorf("unexpected response from server: agent route has inconsistent lifecycle timestamps")
	}
	if route.ExpiresAt != nil && route.ExpiresAt.Before(route.CreatedAt) {
		return fmt.Errorf("unexpected response from server: agent route expires before it was created")
	}
	if route.RevokedAt != nil && (route.RevokedAt.Before(route.CreatedAt) || route.RevokedAt.After(route.UpdatedAt)) {
		return fmt.Errorf("unexpected response from server: agent route has an invalid revocation timestamp")
	}

	switch route.Status {
	case api.Active:
		if route.RevokedAt != nil {
			return fmt.Errorf("unexpected response from server: active route has a revocation timestamp")
		}
	case api.Expired:
		if route.ExpiresAt == nil || route.RevokedAt != nil {
			return fmt.Errorf("unexpected response from server: expired route has inconsistent lifecycle state")
		}
	case api.Revoked:
		if route.RevokedAt == nil {
			return fmt.Errorf("unexpected response from server: revoked route is missing its revocation timestamp")
		}
	default:
		return fmt.Errorf("unexpected response from server: agent route has an invalid status")
	}
	return nil
}

func printAgentRouteCreateSuccess(route api.AgentRouteResponse) {
	fmt.Println("Agent route created successfully.")
	fmt.Println()
	printAgentRouteDetails(route)
}

func printAgentRouteDetails(route api.AgentRouteResponse) {
	printViewField("Route ID", sanitizeAgentRouteOutput(route.Id))
	printViewField("Agent ID", sanitizeAgentRouteOutput(route.AgentId))
	if route.Url != nil && *route.Url != "" {
		printViewField("URL", sanitizeAgentRouteOutput(*route.Url))
	}
	printViewField("Path", sanitizeAgentRouteOutput(route.Path))
	if route.Label != nil {
		printViewField("Label", sanitizeAgentRouteOutput(*route.Label))
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

func sanitizeAgentRouteOutput(value string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case '\n', '\r', '\t':
			return ' '
		default:
			if unicode.IsControl(r) {
				return -1
			}
			return r
		}
	}, value)
}

func formatAgentRouteListAPIError(err error, fallback string) error {
	if apiErr, ok := apierrors.IsAPIError(err); ok {
		safeError := apierrors.NewAPIError(apiErr.StatusCode, "", apiErr.Context)
		return errors.New(safeError.GetUserFriendlyMessage())
	}
	return errors.New(fallback)
}

func formatAgentRouteAPIError(err error, fallback, routeID string) error {
	formatted := formatAPIError(err, fallback).Error()
	escapedID := url.PathEscape(routeID)
	formatted = strings.ReplaceAll(formatted, "/agent-routes/"+escapedID, "/agent-routes/REDACTED")
	if len(routeID) >= 8 {
		formatted = strings.ReplaceAll(formatted, routeID, "REDACTED")
	}
	return errors.New(formatted)
}
