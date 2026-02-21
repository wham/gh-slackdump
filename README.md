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

### Example

```
gh slackdump https://github-grid.enterprise.slack.com/archives/C01234ABCDE
```

## Prerequisites

- [GitHub CLI](https://cli.github.com/) (`gh`)
- Go 1.21+ (for building from source)
- macOS with Safari signed in to the GitHub Slack workspace

## Development

Build, install, and run locally:

```
scripts/run <slack-link>
```

This builds the binary, installs it as a `gh` extension, and runs `gh slackdump` with the provided arguments.