# Agent Context

Background and references for AI coding agents working on this project.

## Overview

This is a GH CLI extension similar to [gh-slack](https://github.com/rneatherway/gh-slack) that uses [slackdump](https://github.com/rusq/slackdump) under the hood.

## References

- [gh-hubber-skills](https://github.com/github/gh-hubber-skills) — Example of a modern GitHub internal `gh` CLI extension (Go + cobra pattern)
- [wham/impact](https://github.com/wham/impact) — Example of using slackdump in a Go app

## Architecture

- `main.go` — Entry point with cobra root command, flags (`--test`, `-o`, `--from`, `--to`, `-u`, `-f`), and `slog`-based logging
- `internal/auth/desktop.go` — Auth provider with uTLS transport: reads the `d` cookie from the Slack desktop app's cookie database, exchanges it for a Slack API token
- `internal/auth/cookie_password_darwin.go` — macOS Keychain access via `go-keychain` for decrypting the Slack desktop app's cookie
- `internal/users/users.go` — User ID resolution: fetches workspace users via `slackdump.Session.GetUsers`, caches as `users.json` in the gh CLI cache directory, and replaces user IDs with Slack handles throughout the conversation struct
- `scripts/run` — Development script that builds and runs the binary directly
- `scripts/test` — Runs `go test ./...`
- `scripts/release` — Release script that bumps the semver tag (patch/minor/major) and pushes it to trigger GoReleaser

## Key Implementation Details

- Authentication reads the `d` cookie from the Slack desktop app's SQLite cookie database (Chromium-based)
- The `d` cookie is exchanged for a Slack API token by fetching the workspace URL and extracting `api_token` from the response
- The approach is based on how [gh-slack](https://github.com/rneatherway/gh-slack) handles auth via the [rneatherway/slack](https://github.com/rneatherway/slack) library
- On macOS, the cookie password is retrieved from the Keychain (`Slack Safe Storage`) using `go-keychain`
- Cookie decryption uses PBKDF2 + AES-CBC (Chromium's cookie encryption scheme)
- Handles Chromium's domain hash prefix (added in Chromium 128+) by stripping SHA256 domain hashes
- The workspace URL is derived from the Slack link provided by the user
- TLS connections use [uTLS](https://github.com/refraction-networking/utls) with `HelloSafari_Auto` to mimic Safari's TLS fingerprint
- `slackdump.WithForceEnterprise(true)` is automatically set when the link is an `*.enterprise.slack.com` URL
- Logging uses `slog`; suppressed when outputting to stdout, enabled when `-o` is set
- User cache is stored at `config.CacheDir()/slackdump/<workspace-host>/users.json` using the `go-gh` library's XDG-based cache directory

## Guidelines

- Always update `README.md` when adding or changing user-facing commands, flags, or behavior
- Always update `AGENTS.md` when changing architecture, key implementation details, or conventions
- Always keep `--help` output up to date: when adding, removing, or changing flags, update the cobra command definition in `main.go` (including `Long`, `Example`, and flag descriptions) so that `gh slackdump --help` accurately documents all available options

## Testing

Run unit tests with `scripts/test`. Build and manually test with `scripts/run`. The user is signed in to a test Slack workspace. Test with the following links, outputting to the `/dumps` directory (gitignored):

- Channel: https://slack-mdworkspace.slack.com/archives/C09036MGFJ4
- Thread: https://slack-mdworkspace.slack.com/archives/C09036MGFJ4/p1771747003176409
- DM: https://slack-mdworkspace.slack.com/archives/D09036MAT96
- Bot: https://slack-mdworkspace.slack.com/archives/D09036MAB16

Example runs:

```
scripts/run -o dumps/channel.json https://slack-mdworkspace.slack.com/archives/C09036MGFJ4
scripts/run -u -o dumps/channel.json https://slack-mdworkspace.slack.com/archives/C09036MGFJ4
scripts/run -u -f -o dumps/channel.json https://slack-mdworkspace.slack.com/archives/C09036MGFJ4
scripts/run -o dumps/thread.json https://slack-mdworkspace.slack.com/archives/C09036MGFJ4/p1771747003176409
scripts/run -o dumps/dm.json https://slack-mdworkspace.slack.com/archives/D09036MAT96
scripts/run --from 2025-06-01 --to 2025-07-01 -o dumps/channel-june.json https://slack-mdworkspace.slack.com/archives/C09036MGFJ4
scripts/run --from 2025-06-15T00:00:00Z --to 2025-06-15T23:59:59Z -o dumps/channel-day.json https://slack-mdworkspace.slack.com/archives/C09036MGFJ4
```
