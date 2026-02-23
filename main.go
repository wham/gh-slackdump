package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strings"
	"time"

	sdauth "github.com/wham/gh-slackdump/internal/auth"

	"github.com/rusq/slackdump/v3"
	"github.com/spf13/cobra"
)

var version = "dev"

var (
	testFlag   bool
	outputFile string
	fromTime   string
	toTime     string
)

var rootCmd = &cobra.Command{
	Use:   "gh slackdump <slack-link>",
	Short: "Dump Slack conversations to stdout in JSON export format",
	Long: `GH CLI extension that uses slackdump to dump the content of a Slack link
to stdout in Slack's JSON export format.

Supports channels, threads, and direct messages in both regular (*.slack.com)
and enterprise (*.enterprise.slack.com) workspaces. Authenticates via Safari's
cookie storage — requires Safari to be signed in to your Slack workspace.

Use --from and --to to restrict the dump to a specific time range. Both flags
accept RFC3339 timestamps (e.g. 2024-01-15T09:00:00Z) or plain dates
(e.g. 2024-01-15, interpreted as midnight UTC). When omitted, all messages
are dumped. The time range filters by parent message timestamp; thread
replies are included or excluded together with their parent.`,
	Example: `  gh slackdump https://myworkspace.slack.com/archives/C09036MGFJ4
  gh slackdump -o output.json https://myworkspace.enterprise.slack.com/archives/CMH59UX4P
  gh slackdump --from 2024-01-01 --to 2024-01-31 https://myworkspace.slack.com/archives/C09036MGFJ4
  gh slackdump --from 2024-01-15T09:00:00Z --to 2024-01-15T17:00:00Z https://myworkspace.slack.com/archives/C09036MGFJ4
  gh slackdump --test`,
	Version:      version,
	Args:         cobra.ExactArgs(1),
	RunE:         run,
	SilenceUsage: true,
}

func init() {
	rootCmd.Flags().BoolVar(&testFlag, "test", false, "Show detected User-Agent and parsed cookies, then exit")
	rootCmd.Flags().StringVarP(&outputFile, "output", "o", "", "Write output to file instead of stdout")
	rootCmd.Flags().StringVar(&fromTime, "from", "", "Dump messages after this time (RFC3339 or YYYY-MM-DD)")
	rootCmd.Flags().StringVar(&toTime, "to", "", "Dump messages before this time (RFC3339 or YYYY-MM-DD)")
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
	provider, err := sdauth.NewSafariProvider(ctx, workspaceURL)
	if err != nil {
		return err
	}

	isEnterprise := strings.Contains(workspaceURL, ".enterprise.slack.com")
	sd, err := slackdump.New(ctx, provider, slackdump.WithForceEnterprise(isEnterprise))
	if err != nil {
		return err
	}

	slog.Info("dumping conversation", "link", slackLink)
	oldest, err := parseTime(fromTime)
	if err != nil {
		return fmt.Errorf("--from: %w", err)
	}
	latest, err := parseTime(toTime)
	if err != nil {
		return fmt.Errorf("--to: %w", err)
	}
	conv, err := sd.Dump(ctx, slackLink, oldest, latest)
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

// parseTime parses a time string in RFC3339 or YYYY-MM-DD format. An empty
// string returns a zero time.Time (meaning no bound).
func parseTime(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid time %q: use RFC3339 (e.g. 2024-01-15T09:00:00Z) or YYYY-MM-DD", s)
}

func runTest() error {
	cookies, ua, err := sdauth.ReadSafariCookies()
	if err != nil {
		return err
	}
	if ua == "" {
		ua = "(Safari not found)"
	}
	slog.Info("safari", "user-agent", ua)
	for _, c := range cookies {
		v := c.Value
		if len(v) > 40 {
			v = v[:40] + "..."
		}
		slog.Info("cookie", "name", c.Name, "secure", c.Secure, "httponly", c.HttpOnly, "value", v)
	}
	slog.Info("total", "cookies", len(cookies))
	return nil
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
