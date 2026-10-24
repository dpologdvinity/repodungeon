# Contributing

Thanks for helping with `repodungeon`. This page covers local setup, the
checks CI runs, and how to send a change. For how the code is organised read
[ARCHITECTURE.md](ARCHITECTURE.md); for exact game rules read
[docs/SPEC.md](docs/SPEC.md), which is the source of truth for behavior.

## Setup

You need Go 1.26 or newer and `git` in your `PATH`.

```sh
git clone https://github.com/dpologdvinity/repodungeon.git
cd repodungeon
go build ./...
go run . -C .          # play the dungeon of this repo
```

## Checks (run these before you push)

CI runs the same commands.

```sh
test -z "$(gofmt -l .)"              # formatting
go vet ./...                          # static checks
golangci-lint run ./...               # linters (install: https://golangci-lint.run)
go test -race -cover ./...            # tests; total coverage must stay at or above 85%
go run golang.org/x/vuln/cmd/govulncheck@latest ./...   # known vulnerabilities
```

To check the release build locally: `goreleaser check` and
`goreleaser build --snapshot --clean`.

## Working rules

- **Tests first.** Add or update a test for every new rule and every bug fix.
  For a bug, write a failing test, then fix it.
- **Game logic stays pure.** `internal/dungeon` and `internal/game` do no I/O,
  never read the clock, and never import Bubble Tea. Use `dungeon.FromArt` to
  build small levels for game tests.
- **Randomness** comes only from `dungeon.RNG`. The order of RNG calls is part
  of the dungeon format: changing it changes every dungeon, so refresh the
  golden files (`go test ./internal/dungeon/ -update`), review the diff, and say
  so in the commit message.
- **Read-only.** The tool may only run git commands that read.
- **Comments.** Every package, type, function, field and test has a short
  comment in plain English. Explain why, not only what.
- **Errors** are returned, not hidden. Do not add `//nolint` or skip tests to
  get a green build; fix the cause.
- **Dependencies.** The only third-party dependencies are Bubble Tea and
  `charmbracelet/x/term` (terminal detection), which Bubble Tea already uses.
  Ask before adding another.

## Commits and pull requests

- Use [Conventional Commits](https://www.conventionalcommits.org/):
  `type(scope): description`, for example `fix(repo): skip missing blobs`.
  Types: `feat`, `fix`, `docs`, `test`, `refactor`, `perf`, `build`, `ci`, `chore`.
- One logical change per commit. Keep commits small.
- Never commit secrets (`.env`, keys, tokens) or tool artifacts such as
  `.claude/`, `CLAUDE.md`, `AGENTS.md`.
- Open pull requests against `master`. Describe what changed and why, and link
  the SPEC section if a rule changed. All CI jobs must pass.

## Releases

Maintainers release by pushing a tag: `git tag vX.Y.Z && git push origin vX.Y.Z`.
The `release` workflow tests, then GoReleaser publishes binaries for linux,
darwin and windows on amd64 and arm64 and updates the Homebrew cask in
`dpologdvinity/homebrew-tap` (needs the `HOMEBREW_TAP_TOKEN` secret).
