# Architecture

This document explains how `repodungeon` is built, why it is split the way it
is, and where to change things. For what the game does, read
[`docs/SPEC.md`](docs/SPEC.md). That file is the source of truth for behavior.
For how it was built step by step, see [`docs/PLAN.md`](docs/PLAN.md).

## The idea in one paragraph

`repodungeon` reads the HEAD commit of a git repository, turns its directories
and files into a dungeon, and lets you play it in a terminal. The same commit
always gives the same dungeon and the same combat results for the same key
presses. The game never writes to the repository.

## Data flow

```
 git repo ──► repo.Scan ──► dungeon.Generate ──► game.State ◄── tui.Model ◄── keys
 (HEAD)       *repo.Repo     *dungeon.Dungeon       │
                                                    ▼
                                         Render() screen / Card() summary
```

1. `internal/repo` runs read-only `git` commands and returns a list of files
   with line counts, TODO/FIXME counts and churn.
2. `internal/dungeon` turns that list into levels, rooms, monsters, items and a
   boss. This step is pure: no I/O, no clock, no global state.
3. `internal/game` holds the live game state. It changes only when `Apply`
   receives a key string.
4. `internal/tui` is a thin Bubble Tea adapter. It forwards key presses to
   `game` and shows `game.Render()`.
5. `internal/cli` wires all of the above together and prints the final card.
   `main.go` only builds the real environment and calls `cli.Run`.

## Packages

| Package | Responsibility | Imports |
|---|---|---|
| `main` | Builds the real `cli.Env` (terminal check, Bubble Tea runner, clock, version) and exits with `cli.Run`'s code. | `cli`, Bubble Tea |
| `internal/cli` | Parses flags, runs scan, generate, TUI, prints the card. Maps errors to messages and exit codes. | `repo`, `dungeon`, `game`, `tui` |
| `internal/repo` | Reads the repository through `git`. Produces `repo.Repo`. | stdlib only |
| `internal/dungeon` | Deterministic generation: RNG, directory plan, map carving, entity placement. | `repo` (types only) |
| `internal/game` | Rules: movement, combat, monster AI, items, stairs, field of view, screen text, summary card. | `dungeon` |
| `internal/tui` | Bubble Tea `Model` over `game.State`. | `game`, Bubble Tea |
| `internal/testrepo` | Test helper that builds fixture git repos with fixed dates and identity. | stdlib only |
| `internal/golden` | Test helper that compares output to files in `testdata/golden`. | stdlib only |

Rules that keep this tidy:

- `dungeon` and `game` never import Bubble Tea. All game logic can be tested by
  calling `Apply` with key strings and reading the resulting state.
- Only `tui`, `cli` and `main` import Bubble Tea. `main` also uses
  `charmbracelet/x/term` to detect a real terminal (`/dev/null` and pipes are
  rejected). These are the only third-party dependencies.
- `time.Now()` is called only in `main.go`. The card date comes from
  `Env.Now`, so tests can fix it.
- All randomness goes through `dungeon.RNG`. Nothing uses `math/rand`.

## Reading the repo (`internal/repo`)

`Scan(dir)` makes these git calls, each as `git -C <dir> ...` with
`LC_ALL=C` so output is not translated:

1. `rev-parse --show-toplevel` finds the repo and its name.
2. `rev-parse --verify --quiet HEAD` finds the commit hash.
3. `ls-tree -r -z -l --full-tree HEAD` lists every committed file with its size.
   Only regular blobs are kept. Symlinks and submodules are dropped. Blobs over
   1 MiB are marked `Skipped` and never read.
4. One `cat-file --batch` process streams the remaining blobs. A goroutine
   writes the hashes while the main goroutine reads the contents, so neither
   side blocks on a full pipe. Each file gets a line count, a binary flag (a NUL
   byte in the first 8000 bytes) and TODO and FIXME counts.
5. `log -z --no-merges --no-renames --max-count=5000 --name-only` counts how
   often each path changed. This count is the file's churn.

Details worth knowing:

- Only committed files appear. Ignored and untracked files are invisible.
- `GIT_NO_LAZY_FETCH=1` stops git from downloading missing blobs one by one in
  a partial clone. Such blobs are marked `Skipped` instead.
- File paths come from untrusted repositories and are later printed to the
  terminal. `Scan` replaces non-printable characters with `?` and keeps paths
  unique if that makes two collide.

## Generating the dungeon (`internal/dungeon`)

### Random numbers

`rng.go` has a small SplitMix64 generator. The standard library generators do
not promise the same output across Go versions, and this game needs a dungeon
that never changes for a given commit. The seed is the first 16 hex digits of
the HEAD hash. Generation uses `NewRNG(seed)`. Combat uses `NewRNG(seed + 1)`,
so fights never disturb the map.

The order of RNG calls is part of the file format. Reordering any call changes
every dungeon and breaks the golden files.

### Plan: directories to levels and rooms (`plan.go`)

- A file's room is its directory, cut to at most 7 path components.
- A directory at depth `d` lives on level `d`. The repo root is level 0.
- A level may hold at most 9 rooms (a 3x3 grid). If a level has more
  directories, the 8 with the most files keep a room and the rest merge into
  one `misc/` room.

### Map (`gen.go`, `buildLevel`)

Each level is 80x21 tiles. The grid has 9 cells; a random permutation picks a
cell per room. Inside its cell a room gets a random offset. Room size grows with
the number of files. Rooms are joined in order by L-shaped corridors; a corridor
crossing a wall makes a door. The first room holds the start. Every level but
the last has stairs.

### Entities (`gen.go`, `populate`)

Per room, using a free list of floor tiles (minus start and stairs):

1. Monsters: the room's files except the boss, sorted by churn, lines, path;
   the first 6 are placed.
2. Potions: one per TODO, at most 3.
3. Scrolls: one per FIXME, at most 2.
4. The boss, on the last level in the last room.

The boss is the text file with the highest churn. Stat formulas live in
`stats.go`.

## Game rules (`internal/game`)

`State` holds the dungeon, the player, messages and counters. `Apply(key)` is
the only way to change it. One call is one key press:

1. Do nothing if the game is over.
2. Clear the message list.
3. Run the key in the current mode (play, inventory or the how-to-play help screen; any key closes help).
4. If the action took a turn: run the monster phase (unless the game just
   ended), increment the turn counter, update the field of view.

Notes:

- Combat damage is `max(1, atk + roll - 1)` with `roll` from `IntN(3)`.
- Slain monsters stay in the slice with `HP <= 0`. Every loop skips them. This
  keeps indexes stable.
- Monsters sleep until they enter the player's field of view (radius 5, walls do
  not block). Awake monsters step toward the player, or attack if adjacent.
- Taking stairs costs a turn but monsters do not act.
- `Render()` produces exactly 24 rows of plain text. `Card()` produces the
  44-column summary printed after the game.

## Terminal UI (`internal/tui`)

`Model` wraps `*game.State`. `Update` forwards each key press to `Apply` and
returns `tea.Quit` once the player quits, or on any key after the game is over.
`View` returns `Render()` on the alternate screen. There is no other logic here.

## Command line (`internal/cli`)

`Run(args, stdout, stderr, env)` returns the exit code and never calls
`os.Exit`. Everything that touches the outside world is in `Env`: version,
whether stdin and stdout are terminals, how to run the TUI, and the clock.
Tests pass a fake `Env`, so the full flow runs without a real terminal.
Errors print `repodungeon: <message>` to stderr and exit 2. A finished game
prints the card and exits 0.

## Testing

- Unit tests live next to the code and use only the standard library.
- `internal/testrepo` builds real git repositories in temp dirs with a fixed
  author, committer and dates, so scan results are reproducible.
- Golden files in `internal/dungeon/testdata/golden` pin the exact generated
  maps. After an intended change, run
  `go test ./internal/dungeon/ -update` and review the diff.
- Game tests use `dungeon.FromArt` to build small hand-drawn levels, feed key
  strings to `Apply`, and check positions, HP, messages and exact screen text.
- `playthrough_test.go` drives a simple bot through a generated dungeon to prove
  a whole run works: pick up loot, drink a potion, descend, end, print the card.

Run everything the same way CI does:

```sh
test -z "$(gofmt -l .)" && go vet ./... && go test ./...
```

## Build, CI and release

`.github/workflows/ci.yml` runs on pushes and pull requests to `master`. Jobs:

| Job | What it does |
|---|---|
| `deps` | `go mod download`, `go mod verify`, and checks `go.mod`/`go.sum` are tidy. |
| `lint` | `gofmt`, `go vet`, `golangci-lint`, and `govulncheck`. |
| `test` | `go test -race` with coverage on Linux and macOS. Fails if total coverage is below 85%. Coverage is written to the job summary and uploaded. |
| `build` | `goreleaser check` and a snapshot build of every target. Runs only after `lint` and `test` pass. |

Details: third-party actions are pinned to commit SHAs, the matrix has two
entries and fails fast, old runs of the same ref are cancelled, and `setup-go`
caches modules and builds.

`.github/workflows/release.yml` runs on tags named `v*`. It re-runs format, vet
and tests, then GoReleaser publishes binaries for linux, darwin and windows on
amd64 and arm64 and pushes a Homebrew cask to `dpologdvinity/homebrew-tap`
(secret `HOMEBREW_TAP_TOKEN`). `.github/dependabot.yml` proposes weekly updates for Go
modules and actions.

The version string is set at build time with `-X main.version`. For
`go install`, `main.go` falls back to the module version.

## Where to change things

| I want to... | Edit |
|---|---|
| Change monster or boss stats, room sizes | `internal/dungeon/stats.go` (and update golden files) |
| Change how directories map to rooms | `internal/dungeon/plan.go` |
| Change map layout or placement | `internal/dungeon/gen.go` (RNG order matters) |
| Add a key or rule | `internal/game/game.go`, then add a test |
| Change screen text or the card | `internal/game/render.go` |
| Read more data from git | `internal/repo/repo.go` |
| Add a flag or error message | `internal/cli/cli.go` |

Any change that alters generation changes every dungeon. Say so in the commit
message.
