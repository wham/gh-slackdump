# Agent Context

Background and references for AI coding agents working on this project.

## Overview

This is a GH CLI extension similar to [gh-slack](https://github.com/rneatherway/gh-slack) that uses [slackdump](https://github.com/rusq/slackdump) under the hood.

## References

- [gh-hubber-skills](https://github.com/github/gh-hubber-skills) — Example of a modern GitHub internal `gh` CLI extension (Go + cobra pattern)
- [wham/impact](https://github.com/wham/impact) — Example of using slackdump in a Go app with Safari cookie auth and TLS fingerprinting for the GitHub Slack workspace. The cookie auth and TLS tricks in `internal/auth/safari.go` are ported from this project.

## Architecture

- `main.go` — Entry point with cobra root command
- `internal/auth/safari.go` — Safari cookie auth provider with uTLS fingerprinting, binary cookie parsing, and Slack token extraction
- `scripts/run` — Development script that builds, installs, and runs the extension via `gh`

## Key Implementation Details

- Authentication reads Safari's `Cookies.binarycookies` file and exchanges cookies for a Slack API token via the `/ssb/redirect` endpoint
- TLS connections use [uTLS](https://github.com/refraction-networking/utls) with `HelloSafari_Auto` to mimic Safari's TLS fingerprint
- The User-Agent is detected from the locally installed Safari version
- `slackdump.WithForceEnterprise(true)` is used because GitHub uses Slack Enterprise Grid

## Guidelines

- Always update `README.md` when adding or changing user-facing commands, flags, or behavior
- Always update `AGENTS.md` when changing architecture, key implementation details, or conventions
