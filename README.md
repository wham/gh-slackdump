# gh-slackdump

A [GitHub CLI](https://cli.github.com/) extension that dumps Slack conversations to stdout using [slackdump](https://github.com/rusq/slackdump). Authenticates via Safari cookies with TLS fingerprinting to access the GitHub Slack workspace.

## Installation

```
gh extension install wham/gh-slackdump
```

## Usage

```
gh slackdump <slack-link>
```

Dumps the content of the given Slack link to stdout in Slack's JSON export format.

### Examples

```
gh slackdump https://github-grid.enterprise.slack.com/archives/CMH59UX4P
gh slackdump https://myworkspace.slack.com/archives/C09036MGFJ4
```

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