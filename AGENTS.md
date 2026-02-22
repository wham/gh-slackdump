# Agent Context

Background and references for AI coding agents working on this project.

## Overview

This is a GH CLI extension similar to [gh-slack](https://github.com/rneatherway/gh-slack) that uses [slackdump](https://github.com/rusq/slackdump) under the hood.

## References

- [gh-hubber-skills](https://github.com/github/gh-hubber-skills) — Example of a modern GitHub internal `gh` CLI extension (Go + cobra pattern)
- [wham/impact](https://github.com/wham/impact) — Example of using slackdump in a Go app

## Architecture

- `main.go` — Entry point with cobra root command, flags (`--test`, `-o`), and `slog`-based logging
- `internal/auth/desktop.go` — Slack desktop app cookie auth provider: reads the `d` cookie from Slack's SQLite cookie database, decrypts it using the system keychain, and exchanges it for a Slack API token
- `scripts/run` — Development script that builds and runs the binary directly
- `scripts/test` — Runs `go test ./...`
- `scripts/release` — Release script that bumps the semver tag (patch/minor/major) and pushes it to trigger GoReleaser

## Key Implementation Details

- Authentication reads the Slack desktop app's cookie database (SQLite), decrypts the `d` cookie using the system keychain password, and exchanges it for a Slack API token via the workspace homepage
- The approach is based on how [gh-slack](https://github.com/rneatherway/gh-slack) handles auth via the [rneatherway/slack](https://github.com/rneatherway/slack) library
- On macOS, the cookie password is retrieved from the Keychain (`Slack Safe Storage`) using the `security` CLI
- On Linux, the cookie password is retrieved from the Secret Service using `secret-tool`
- Cookie decryption uses PBKDF2 + AES-CBC (Chromium's cookie encryption scheme)
- Handles Chromium's domain hash prefix (added in Chromium 128+) by stripping SHA256 domain hashes
- The workspace URL is derived from the Slack link provided by the user
- `slackdump.WithForceEnterprise(true)` is automatically set when the link is an `*.enterprise.slack.com` URL
- Logging uses `slog`; suppressed when outputting to stdout, enabled when `-o` is set

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
