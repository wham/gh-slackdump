package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	sdauth "github.com/wham/gh-slackdump/internal/auth"

	"github.com/rusq/slackdump/v3"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "gh-slackdump <slack-link>",
	Short: "Dump Slack conversations to stdout in JSON export format",
	Long:  "GH CLI extension that uses slackdump to dump the content of a Slack link to stdout in Slack's JSON export format.",
	Args:  cobra.ExactArgs(1),
	RunE:  run,
}

func run(cmd *cobra.Command, args []string) error {
	slackLink := args[0]
	ctx := context.Background()

	provider, err := sdauth.NewSafariProvider(ctx)
	if err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	sd, err := slackdump.New(ctx, provider, slackdump.WithForceEnterprise(true))
	if err != nil {
		return fmt.Errorf("creating slackdump session: %w", err)
	}

	conv, err := sd.DumpAll(ctx, slackLink)
	if err != nil {
		return fmt.Errorf("dumping conversation: %w", err)
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(conv); err != nil {
		return fmt.Errorf("encoding output: %w", err)
	}

	return nil
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
