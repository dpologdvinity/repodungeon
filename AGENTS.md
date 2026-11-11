# AGENTS.md

Instructions for coding agents (Claude Code, Codex and others) working in this
repo. `CLAUDE.md` only imports this file.

## What this is

`repodungeon` is a Go terminal roguelike generated from the git repository it
runs in: directories are rooms, files are monsters and the most-changed file is
the boss. The same commit always gives the same dungeon. Humans start with
[README.md](README.md). Read [ARCHITECTURE.md](ARCHITECTURE.md) for design and
[docs/specs/](docs/specs/) for the rules: the v0.1 design, then later feature
specs that change it. Where they disagree, the newer spec wins. Code comments
such as `(SPEC 7.3)` cite sections of the v0.1 design spec.

## Layout

| Path | Role |
|------|------|
| `main.go` | Wires the real terminal, clock and version into `cli.Run` |
| `internal/cli` | Scan, then generate, then TUI, then end card |
| `internal/repo` | Reads HEAD tree, contents and churn, using read-only git commands |
| `internal/dungeon` | Pure dungeon generation from a scan (SplitMix64 RNG) |
| `internal/game` | Pure game state machine, driven by key strings |
| `internal/tui` | Thin Bubble Tea v2 model over `game.State` |
| `internal/golden`, `internal/testrepo` | Test helpers: golden files and throwaway repos |

## Commands

Needs Go 1.26+ and `git`. CI (`.github/workflows/ci.yml`) runs the same checks.

```sh
go run . -C .                          # play this repo's dungeon
test -z "$(gofmt -l .)"                # formatting
go vet ./...
golangci-lint run ./...
go test -race -cover ./...             # total coverage must stay >= 85%
go test ./internal/dungeon/ -update    # rewrite golden files, then review the diff
go run golang.org/x/vuln/cmd/govulncheck@latest ./...
goreleaser check && goreleaser build --snapshot --clean
lefthook install                       # pre-commit: gitleaks, gofmt, golangci-lint, typos
```

## Rules

- Write the test first. For a bug, write a failing test, then fix it.
- `internal/dungeon` and `internal/game` stay pure: no I/O, no clock and no
  Bubble Tea. Build small test levels with `dungeon.FromArt`.
- All randomness comes from `dungeon.RNG`. The order of RNG calls is part of
  the dungeon format: if you change it, refresh the golden files and say so in
  the commit message.
- The tool only runs git commands that read.
- Every package, type, function and test has a short plain-English comment.
- Return errors; never hide them. No `//nolint` and no skipped tests.
- Add no dependencies besides Bubble Tea and `charmbracelet/x/term` without
  asking first.
- Use [Conventional Commits](https://www.conventionalcommits.org/), one logical
  change per commit. The branch is `master`.

## Environment and secrets

Local development and tests need no environment variables. Releases run when a
`v*` tag is pushed (`.github/workflows/release.yml`) and need these repository
secrets: `HOMEBREW_TAP_TOKEN` and `APT_GPG_PRIVATE_KEY`. Never commit `.env*`
or `*.local.*` files.

## Agent workflow

- **Specs and plans:** Features follow the
  [superpowers](https://github.com/obra/superpowers) flow: brainstorm, then
  write a spec, then write a plan, then do subagent-driven implementation and
  code review. The finished documents are kept as the design record:
  - Specs: `docs/specs/YYYY-MM-DD-<topic>-design.md`
  - Plans: `docs/plans/YYYY-MM-DD-<topic>.md`
- **Claude Code plugins:** `.claude/settings.json` enables `gopls-lsp`,
  `commit-commands` and `ponytail`, which pushes for the smallest complete
  change.
- **Codex plugins:** `.codex/config.toml` enables the same `ponytail` plugin.
- **Secret gate:** Lefthook runs `gitleaks` on every commit, so agents can't
  commit secrets.
