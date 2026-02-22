# gh-slackdump

A [GitHub CLI](https://cli.github.com/) extension that dumps a provided Slack conversation into a [JSON file](https://slack.com/help/articles/220556107-How-to-read-Slack-data-exports). The extension uses [slackdump](https://github.com/rusq/slackdump) under the hood.
With a [trick](https://github.com/rusq/slackdump/discussions/526#discussioncomment-14370498) to make it work in enterprise Slack workspaces and avoid the [security notifications](https://slack.com/help/articles/37506096763283-Understand-Slack-Security-notifications) followed by sign out.

`gh-slackdump` currently only works on macOS and requires Safari to be signed in to the GitHub Slack workspace, as it relies on Safari's cookie storage for authentication. File an issue to request support for other platforms or browsers.

## Installation

```
gh extension install wham/gh-slackdump
```

## Usage

Sign it to your Slack workspace in **Safari** first. `gh-slackdump` will read the necessary cookies from Safari to authenticate with Slack.

```
gh slackdump <slack-link>
```

Dumps the content of the given Slack link to stdout in Slack's JSON export format.

### Examples

```
gh slackdump https://github-grid.enterprise.slack.com/archives/CMH59UX4P
gh slackdump -o output.json https://myworkspace.slack.com/archives/C09036MGFJ4
```

### Flags

- `-o, --output <file>` — Write JSON output to a file instead of stdout. When set, progress is logged to stdout.

### Test cookies

```
gh slackdump --test
```

Shows the detected Safari User-Agent and parsed Slack cookies without connecting to Slack. Useful for verifying that Safari cookie access is working.

## Prerequisites

- [GitHub CLI](https://cli.github.com/) (`gh`)
- macOS with Safari signed in to the GitHub Slack workspace

## Development

Build, install, and run locally (requires Go 1.21+):

```
scripts/run <slack-link>
```

This builds the binary, installs it as a `gh` extension, and runs `gh slackdump` with the provided arguments.

## Releasing

Releases are automated with [GoReleaser](https://goreleaser.com/) via GitHub Actions. To publish a new version:

```
git tag v0.1.0
git push origin v0.1.0
```

The workflow builds macOS binaries (amd64 + arm64) and creates a GitHub Release, enabling `gh extension install` without requiring Go.