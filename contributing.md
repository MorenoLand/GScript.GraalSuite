# Contributing

Work on a feature branch and open changes against the project’s active development branch. Keep generated output, private signing keys, credentials, and local paths out of commits.

## Requirements

- Go version from `go.mod`
- Node.js and pnpm
- Wails CLI `v3.0.0-beta.10`

Install dependencies with `pnpm install --frozen-lockfile`.

## Checks

Run `go test -mod=mod ./...` and `git diff --check` before opening a pull request.

## Release builds

Release CI produces exactly these application assets:

- `GSuite-windows-amd64.exe`
- `GSuite-darwin-amd64.zip`, containing only `GSuite.app`
- `GSuite-linux-amd64.deb`

The updater reads `https://github.com/MorenoLand/GScript.GSuite/releases/latest/download/latest.json`. The Wails updater private key belongs only in the GitHub Actions secret `WAILS_UPDATER_PRIVATE_KEY`; never commit or paste it into source. The checked-in `wails-updater.key.pub` is the public verification key.
