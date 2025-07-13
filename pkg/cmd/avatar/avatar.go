package avatar

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/mirako-ai/mirako-cli/internal/config"
	"github.com/mirako-ai/mirako-cli/internal/client"
)

func NewAvatarCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "avatar",
		Short: "Manage avatars",
		Long:  `Create, list, and manage AI avatars`,
	}

	cmd.AddCommand(newListCmd())
	cmd.AddCommand(newViewCmd())
	cmd.AddCommand(newGenerateCmd())
	cmd.AddCommand(newStatusCmd())

	return cmd
}

func newListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all avatars",
		Long:  `List all avatars for the current user`,
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

	resp, err := client.ListAvatars(context.Background())
	if err != nil {
		return fmt.Errorf("failed to list avatars: %w", err)
	}

	useJSON, _ := cmd.Flags().GetBool("json")
	if useJSON {
		data, _ := json.MarshalIndent(resp, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	if resp.Data == nil || len(*resp.Data) == 0 {
		fmt.Println("No avatars found")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tSTATUS\tCREATED")

	for _, avatar := range *resp.Data {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			avatar.Id,
			avatar.Name,
			avatar.Status,
			avatar.CreatedAt.Format("2006-01-02 15:04"),
		)
	}

	w.Flush()
	return nil
}

func newViewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "view [avatar-id]",
		Short: "View avatar details",
		Long:  `View detailed information about a specific avatar`,
		Args:  cobra.ExactArgs(1),
		RunE:  runView,
	}
}

func runView(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	client, err := client.New(cfg)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	avatarID := args[0]
	resp, err := client.GetAvatar(context.Background(), avatarID)
	if err != nil {
		return fmt.Errorf("failed to get avatar: %w", err)
	}

	fmt.Printf("ID: %s\n", resp.Data.Id)
	fmt.Printf("Name: %s\n", resp.Data.Name)
	fmt.Printf("Status: %s\n", resp.Data.Status)
	fmt.Printf("Created: %s\n", resp.Data.CreatedAt.Format("2006-01-02 15:04"))
	fmt.Printf("User ID: %s\n", resp.Data.UserId)

	if resp.Data.Themes != nil && len(*resp.Data.Themes) > 0 {
		fmt.Println("\nThemes:")
		for _, theme := range *resp.Data.Themes {
			fmt.Printf("  - %s:\n", theme.Name)
			if theme.KeyImage != nil {
				fmt.Printf("    Key Image: %s\n", *theme.KeyImage)
			}
			if theme.LiveVideo != nil {
				fmt.Printf("    Live Video: %s\n", *theme.LiveVideo)
			}
		}
	}

	return nil
}

func newGenerateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate a new avatar",
		Long:  `Generate a new avatar using AI`,
		RunE:  runGenerate,
	}

	cmd.Flags().StringP("prompt", "p", "", "Prompt for avatar generation")
	cmd.Flags().Int64P("seed", "s", 0, "Seed for reproducible generation (optional)")

	return cmd
}

func runGenerate(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	prompt, _ := cmd.Flags().GetString("prompt")
	if prompt == "" {
		return fmt.Errorf("prompt is required. Use --prompt flag")
	}

	seed, _ := cmd.Flags().GetInt64("seed")
	var seedPtr *int64
	if seed != 0 {
		seedPtr = &seed
	}

	client, err := client.New(cfg)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	resp, err := client.GenerateAvatar(context.Background(), prompt, seedPtr)
	if err != nil {
		return fmt.Errorf("failed to generate avatar: %w", err)
	}

	if resp.JSON200 == nil {
		return fmt.Errorf("unexpected response from server")
	}

	fmt.Printf("✅ Avatar generation started!\n")
	fmt.Printf("   Task ID: %s\n", resp.JSON200.Data.TaskId)
	fmt.Printf("   Status: %s\n", resp.JSON200.Data.Status)
	fmt.Printf("\nCheck status with: mirako avatar status %s\n", resp.JSON200.Data.TaskId)

	return nil
}

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status [task-id]",
		Short: "Check avatar generation status",
		Long:  `Check the status of an avatar generation task`,
		Args:  cobra.ExactArgs(1),
		RunE:  runStatus,
	}
}

func runStatus(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	taskID := args[0]

	client, err := client.New(cfg)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	resp, err := client.GetAvatarStatus(context.Background(), taskID)
	if err != nil {
		return fmt.Errorf("failed to get status: %w", err)
	}

	if resp.JSON200 == nil || resp.JSON200.Data == nil {
		return fmt.Errorf("unexpected response from server")
	}

	fmt.Printf("Task ID: %s\n", resp.JSON200.Data.TaskId)
	fmt.Printf("Status: %s\n", resp.JSON200.Data.Status)

	if resp.JSON200.Data.Image != nil {
		fmt.Printf("✅ Avatar generated successfully!\n")
		fmt.Printf("   Image: %d bytes\n", len(*resp.JSON200.Data.Image))
	}

	return nil
}