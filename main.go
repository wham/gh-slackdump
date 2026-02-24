package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/url"
	"os"
	"strings"

	sdauth "github.com/wham/gh-slackdump/internal/auth"

	"github.com/rusq/slackdump/v3"
	"github.com/spf13/cobra"
)

var version = "dev"

var (
	testFlag   bool
	outputFile string
)

var rootCmd = &cobra.Command{
	Use:   "gh slackdump <slack-link>",
	Short: "Dump Slack conversations to stdout in JSON export format",
	Long: `GH CLI extension that uses slackdump to dump the content of a Slack link
to stdout in Slack's JSON export format.

Supports channels, threads, and direct messages in both regular (*.slack.com)
and enterprise (*.enterprise.slack.com) workspaces. Authenticates via Safari's
cookie storage (macOS) or the Slack desktop app's local cookie storage —
requires Safari or the Slack desktop app to be signed in to your workspace.`,
	Example: `  gh slackdump https://myworkspace.slack.com/archives/C09036MGFJ4
  gh slackdump -o output.json https://myworkspace.enterprise.slack.com/archives/CMH59UX4P
  gh slackdump --test`,
	Version:      version,
	Args:         cobra.ExactArgs(1),
	RunE:         run,
	SilenceUsage: true,
}

func init() {
	rootCmd.Flags().BoolVar(&testFlag, "test", false, "Show detected Slack cookie source and value, then exit")
	rootCmd.Flags().StringVarP(&outputFile, "output", "o", "", "Write output to file instead of stdout")
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

	// When outputting to stdout, suppress all logging so only JSON is emitted.
	// When writing to a file, log progress to stdout.
	if outputFile == "" {
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})))
	}

	slackLink := args[0]
	ctx := context.Background()

	workspaceURL, err := extractWorkspaceURL(slackLink)
	if err != nil {
		return err
	}

	slog.Info("authenticating", "workspace", workspaceURL)
	provider, err := sdauth.NewProvider(ctx, workspaceURL)
	if err != nil {
		return err
	}

	isEnterprise := strings.Contains(workspaceURL, ".enterprise.slack.com")
	sd, err := slackdump.New(ctx, provider, slackdump.WithForceEnterprise(isEnterprise))
	if err != nil {
		return err
	}

	slog.Info("dumping conversation", "link", slackLink)
	conv, err := sd.DumpAll(ctx, slackLink)
	if err != nil {
		return err
	}

	var out *os.File
	if outputFile != "" {
		f, err := os.Create(outputFile)
		if err != nil {
			return err
		}
		defer f.Close()
		out = f
	} else {
		out = os.Stdout
	}

	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(conv); err != nil {
		return err
	}

	if outputFile != "" {
		slog.Info("output written", "file", outputFile)
	}

	return nil
}

// extractWorkspaceURL derives the workspace base URL from a Slack link.
func extractWorkspaceURL(slackLink string) (string, error) {
	u, err := url.Parse(slackLink)
	if err != nil {
		return "", err
	}
	host := u.Hostname()
	if !strings.HasSuffix(host, ".slack.com") {
		return "", &url.Error{Op: "parse", URL: slackLink, Err: os.ErrInvalid}
	}
	return u.Scheme + "://" + host, nil
}

func runTest() error {
	cookie, err := sdauth.ReadCookie()
	if err != nil {
		return err
	}
	v := cookie
	if len(v) > 40 {
		v = v[:40] + "..."
	}
	slog.Info("cookie", "value", v)
	return nil
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
