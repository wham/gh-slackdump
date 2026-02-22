package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	sdauth "github.com/wham/gh-slackdump/internal/auth"

	"github.com/rusq/slackdump/v3"
	"github.com/spf13/cobra"
)

var testFlag bool

var rootCmd = &cobra.Command{
	Use:   "gh slackdump <slack-link>",
	Short: "Dump Slack conversations to stdout in JSON export format",
	Long:  "GH CLI extension that uses slackdump to dump the content of a Slack link to stdout in Slack's JSON export format.",
	Args:  cobra.ExactArgs(1),
	RunE:  run,
}

func init() {
	rootCmd.Flags().BoolVar(&testFlag, "test", false, "Show detected User-Agent and parsed cookies, then exit")
	rootCmd.Args = func(cmd *cobra.Command, args []string) error {
		if testFlag {
			return cobra.NoArgs(cmd, args)
		}
		return cobra.ExactArgs(1)(cmd, args)
	}
}

func run(cmd *cobra.Command, args []string) error {
	if testFlag {
		return runTest()
	}

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

func runTest() error {
	cookies, ua, err := sdauth.ReadSafariCookies()
	if err != nil {
		return err
	}
	if ua == "" {
		ua = "(Safari not found)"
	}
	fmt.Printf("User-Agent: %s\n\n", ua)
	fmt.Printf("%-30s %-8s %-6s %s\n", "NAME", "SECURE", "HTTP", "VALUE (truncated)")
	fmt.Printf("%-30s %-8s %-6s %s\n", strings.Repeat("-", 30), "------", "----", strings.Repeat("-", 40))
	for _, c := range cookies {
		v := c.Value
		if len(v) > 40 {
			v = v[:40] + "..."
		}
		fmt.Printf("%-30s %-8v %-6v %s\n", c.Name, c.Secure, c.HttpOnly, v)
	}
	fmt.Printf("\nTotal: %d Slack cookies\n", len(cookies))
	return nil
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
