# gh-slackdump

A [GitHub CLI](https://cli.github.com/) extension that dumps Slack conversations into Slack's [JSON export format](https://slack.com/help/articles/220556107-How-to-read-Slack-data-exports) using [slackdump](https://github.com/rusq/slackdump). Inspired by [gh-slack](https://github.com/rneatherway/gh-slack), but can export entire channels and DMs, not just threads.

It authenticates via Safari's cookie storage and uses [TLS fingerprinting](https://github.com/rusq/slackdump/discussions/526#discussioncomment-14370498) to work with enterprise Slack workspaces without triggering [security notifications](https://slack.com/help/articles/37506096763283-Understand-Slack-Security-notifications). Currently macOS-only — requires Safari to be signed in to your Slack workspace.

## Installation

```
gh extension install wham/gh-slackdump
```

To update to the latest version:

```
gh extension upgrade wham/gh-slackdump
```

## Usage

Sign in to your Slack workspace in **Safari** first.

```
gh slackdump <slack-link>
```

Supports channels, threads, and direct messages in both regular (`*.slack.com`) and enterprise (`*.enterprise.slack.com`) workspaces. Copy the link from Slack and pass it as the argument.

<img src="docs/link.png" alt="Copy Slack link" width="400">

The output is written to stdout by default. Use `-o` to write to a file instead.

### Examples

```
gh slackdump https://myworkspace.slack.com/archives/C09036MGFJ4
gh slackdump -u https://myworkspace.slack.com/archives/C09036MGFJ4
gh slackdump -u -f https://myworkspace.slack.com/archives/C09036MGFJ4
gh slackdump -o output.json https://myworkspace.enterprise.slack.com/archives/CMH59UX4P
gh slackdump --from 2024-01-01 --to 2024-01-31 https://myworkspace.slack.com/archives/C09036MGFJ4
gh slackdump --test
```

### Flags

| Flag | Description |
|---|---|
| `-o, --output <file>` | Write JSON output to a file instead of stdout. When set, progress is logged to stdout. |
| `-u, --users` | Replace user IDs with Slack handles. Fetches the workspace user list on first use and caches it. |
| `-f, --force` | Force re-fetch of the cached user list (implies `-u`). |
| `--from <time>` | Dump only messages after this time. Accepts RFC3339 (e.g. `2024-01-02T15:04:05Z`) or date-only (`2024-01-02`). Filters by parent message timestamp; thread replies follow their parent. |
| `--to <time>` | Dump only messages before this time. Accepts RFC3339 (e.g. `2024-01-31T23:59:59Z`) or date-only (`2024-01-31`). Filters by parent message timestamp; thread replies follow their parent. |
| `--test` | Show the detected Safari User-Agent and parsed Slack cookies, then exit. Useful for verifying that cookie access is working. |
| `-v, --version` | Print the version number and exit. |
| `-h, --help` | Show help with all available flags and usage examples. |

## Output format

The output follows [Slack's export format](https://slack.com/help/articles/220556107-How-to-read-Slack-data-exports) with slackdump extensions:

```json
{
  "channel_id": "C09036MGFJ4",
  "thread_ts": "1771747003.176409",  // only present for thread links
  "name": "channel-name",
  "messages": [
    {
      "type": "message",
      "user": "U09036M8VEU",
      "text": "Hello",
      "ts": "1771747003.176409",
      "reactions": [{ "name": "eyes", "count": 1, "users": ["U09036M8VEU"] }],
      "slackdump_thread_replies": [
        { "type": "message", "user": "U09036M8VEU", "text": "Reply", "ts": "..." }
      ]
    }
  ]
}
```

Thread replies are nested under `slackdump_thread_replies` on the parent message. Users are identified by ID, not display name.

When `-u` is passed, user IDs are replaced with Slack handles everywhere in the JSON — message authors, reactions, thread participants, and `<@mention>` patterns in message text. The workspace user list is fetched once and cached in the gh CLI cache directory (`~/.cache/gh/slackdump/<workspace>/users.json`). Use `-f` to force a re-fetch.

## Development & Releasing

Build and run locally (requires Go 1.21+):

```
scripts/run <slack-link>
```

Run tests:

```
scripts/test
```

Releases are automated with [GoReleaser](https://goreleaser.com/) via GitHub Actions. To publish a new version:

```
scripts/release patch   # v0.2.0 → v0.2.1
scripts/release minor   # v0.2.0 → v0.3.0
scripts/release major   # v0.2.0 → v1.0.0
```

The script reads the latest git tag, bumps the version, and pushes the new tag after confirmation. The workflow then builds macOS binaries (amd64 + arm64) and creates a GitHub Release, enabling `gh extension install` without requiring Go.