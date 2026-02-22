# Agent Context

Background and references for AI coding agents working on this project.

## Overview

This is a GH CLI extension similar to [gh-slack](https://github.com/rneatherway/gh-slack) that uses [slackdump](https://github.com/rusq/slackdump) under the hood.

## References

- [gh-hubber-skills](https://github.com/github/gh-hubber-skills) — Example of a modern GitHub internal `gh` CLI extension (Go + cobra pattern)
- [wham/impact](https://github.com/wham/impact) — Example of using slackdump in a Go app with Safari cookie auth and TLS fingerprinting for the GitHub Slack workspace. The cookie auth and TLS tricks in `internal/auth/safari.go` are ported from this project.

## Architecture

- `main.go` — Entry point with cobra root command, flags (`--test`, `-o`), and `slog`-based logging
- `internal/auth/safari.go` — Safari cookie auth provider with uTLS fingerprinting, binary cookie parsing, and Slack token extraction
- `scripts/run` — Development script that builds and runs the binary directly
- `scripts/release` — Release script that bumps the semver tag (patch/minor/major) and pushes it to trigger GoReleaser

## Key Implementation Details

- Authentication reads Safari's `Cookies.binarycookies` file and exchanges cookies for a Slack API token via the `/ssb/redirect` endpoint
- The workspace URL is derived from the Slack link provided by the user
- TLS connections use [uTLS](https://github.com/refraction-networking/utls) with `HelloSafari_Auto` to mimic Safari's TLS fingerprint
- The User-Agent is detected from the locally installed Safari version
- `slackdump.WithForceEnterprise(true)` is automatically set when the link is an `*.enterprise.slack.com` URL
- Logging uses `slog`; suppressed when outputting to stdout, enabled when `-o` is set

## Guidelines

- Always update `README.md` when adding or changing user-facing commands, flags, or behavior
- Always update `AGENTS.md` when changing architecture, key implementation details, or conventions
- Always keep `--help` output up to date: when adding, removing, or changing flags, update the cobra command definition in `main.go` (including `Long`, `Example`, and flag descriptions) so that `gh slackdump --help` accurately documents all available options

## Testing

Build and test with `scripts/run`. The user is signed in to a test Slack workspace. Test with the following links, outputting to the `/dumps` directory (gitignored):

- Channel: https://slack-mdworkspace.slack.com/archives/C09036MGFJ4
- Thread: https://slack-mdworkspace.slack.com/archives/C09036MGFJ4/p1771747003176409
- DM: https://slack-mdworkspace.slack.com/archives/D09036MAT96
- Bot: https://slack-mdworkspace.slack.com/archives/D09036MAB16
