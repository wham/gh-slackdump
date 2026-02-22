# gh-slackdump

A [GitHub CLI](https://cli.github.com/) extension that dumps Slack conversations into Slack's [JSON export format](https://slack.com/help/articles/220556107-How-to-read-Slack-data-exports) using [slackdump](https://github.com/rusq/slackdump). It authenticates via Safari's cookie storage and uses a [TLS fingerprinting trick](https://github.com/rusq/slackdump/discussions/526#discussioncomment-14370498) to work with enterprise Slack workspaces without triggering [security notifications](https://slack.com/help/articles/37506096763283-Understand-Slack-Security-notifications).

Currently macOS-only. Requires Safari to be signed in to your Slack workspace. File an issue to request support for other platforms or browsers.

## Installation

```
gh extension install wham/gh-slackdump
```

## Usage

Sign in to your Slack workspace in **Safari** first.

```
gh slackdump <slack-link>
```

The output is written to stdout by default. Use `-o` to write to a file instead.

### Examples

```
gh slackdump https://myworkspace.slack.com/archives/C09036MGFJ4
gh slackdump -o output.json https://github-grid.enterprise.slack.com/archives/CMH59UX4P
gh slackdump --test
```

### Flags

| Flag | Description |
|---|---|
| `-o, --output <file>` | Write JSON output to a file instead of stdout. When set, progress is logged to stdout. |
| `--test` | Show the detected Safari User-Agent and parsed Slack cookies, then exit. Useful for verifying that cookie access is working. |

## Prerequisites

- [GitHub CLI](https://cli.github.com/) (`gh`)
- macOS with Safari signed in to the target Slack workspace

## Development & Releasing

Build and run locally (requires Go 1.21+):

```
scripts/run <slack-link>
```

Releases are automated with [GoReleaser](https://goreleaser.com/) via GitHub Actions. To publish a new version:

```
git tag v0.1.0
git push origin v0.1.0
```

The workflow builds macOS binaries (amd64 + arm64) and creates a GitHub Release, enabling `gh extension install` without requiring Go.