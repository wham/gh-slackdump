# gh-slackdump

- GH CLI extension similar to https://github.com/rneatherway/gh-slack
- Uses slackdump under the hood
- Use https://github.com/github/gh-hubber-skills as an example of modern GitHub internal extension
- Check https://github.com/wham/impact for example how to use slackdump in a Go app and sign in to GitHub Slack workspace. Preserve all the cookie auth and TLS tricks

## Usage

`gh slackdump <slack-link>`

Dumps the content of the given slack link to stdout. The output is Slack's JSON export format.

## Development

Build and run:

```
scripts/run <slack-link>
```

This builds the binary into `build/` and passes all arguments through to the extension.