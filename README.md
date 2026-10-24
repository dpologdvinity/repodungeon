# repodungeon

[![CI](https://github.com/dpologdvinity/repodungeon/actions/workflows/ci.yml/badge.svg)](https://github.com/dpologdvinity/repodungeon/actions/workflows/ci.yml)
[![Latest release](https://img.shields.io/github/v/release/dpologdvinity/repodungeon)](https://github.com/dpologdvinity/repodungeon/releases/latest)
[![Go version](https://img.shields.io/github/go-mod/go-version/dpologdvinity/repodungeon)](go.mod)
[![Go Reference](https://pkg.go.dev/badge/github.com/dpologdvinity/repodungeon.svg)](https://pkg.go.dev/github.com/dpologdvinity/repodungeon)
[![License: MIT](https://img.shields.io/github/license/dpologdvinity/repodungeon)](LICENSE)

Turn the git repository you are standing in into a small roguelike.
Directories are rooms, files are monsters, TODO and FIXME comments are loot,
and the boss is the file that changed most often. The same commit always gives
the same dungeon. Combat is turn-based, bump-to-attack, with permadeath. When
the run ends, a shareable text card is printed.

repodungeon is read-only: it only runs `git` commands that read, and only
committed files appear (no ignored or untracked files).

## Install

```sh
brew install dpologdvinity/tap/repodungeon
```

On Debian, Ubuntu and other apt systems:

```sh
curl -fsSL https://dpologdvinity.github.io/repodungeon/repodungeon.gpg | sudo tee /usr/share/keyrings/repodungeon.gpg >/dev/null
echo "deb [signed-by=/usr/share/keyrings/repodungeon.gpg] https://dpologdvinity.github.io/repodungeon stable main" | sudo tee /etc/apt/sources.list.d/repodungeon.list
sudo apt update && sudo apt install repodungeon
```

Or with Go:

```sh
go install github.com/dpologdvinity/repodungeon@latest
```

Or download a binary, `.deb` or `.rpm` for linux, macOS or Windows (amd64, arm64)
from the [GitHub releases](https://github.com/dpologdvinity/repodungeon/releases).

Binaries downloaded directly are not code-signed, so macOS and Windows may warn
about an unidentified developer. Homebrew and `go install` avoid this.

Requirements: `git` in your `PATH`. Building from source needs Go 1.26 or newer.

## Play

```sh
cd your-repo
repodungeon          # or: repodungeon -C path/to/repo
```

The game opens on a how-to-play screen. Press any key to start; press `?` at
any time to read it again.

The map is in color (walls gray, you green, monsters red, the boss bold red,
potions magenta, scrolls cyan). Set `NO_COLOR=1` for plain text. The card printed
at the end is always plain.

| Key | Action |
|---|---|
| `h` `j` `k` `l` or arrows | move / attack |
| `.` | wait a turn |
| `>` | descend (on the stairs) |
| `i` | inventory (`a`-`j` uses an item, `esc` closes) |
| `?` | show the how-to-play screen again |
| `q` | quit |

## How your repo becomes a dungeon

- The seed is the first 16 hex digits of the HEAD commit hash. Same commit,
  same dungeon, same combat rolls for the same key presses.
- Each directory is a room. Directory depth is the dungeon level: the repo
  root is level 1, `src/` is level 2, `src/ui/` is level 3, and so on (at most
  8 levels; deeper directories fold into depth 7). A level with more than 9
  directories keeps the 8 busiest as rooms and merges the rest into `misc/`.
- Each text file is a monster in its directory's room. Its HP grows with its
  line count, and its ATK grows with how often it was changed in git history.
  A room shows at most its 6 most-changed files. Binary files, symlinks,
  submodules and files over 1 MiB never appear.
- Each `TODO` becomes a potion (heals up to 10 HP) and each `FIXME` a scroll
  (+1 ATK). A room holds at most 3 potions and 2 scrolls; the item remembers
  which file it came from.
- The boss waits in the last room of the deepest level: the file with the most
  commits touching it, with a title like "Whisperer of Flaky Tests".
- Slay monsters to gain XP and levels. Reach the boss and kill it to win. Die
  and the run is over; there are no saves.

Flags:

| Flag | Meaning |
|---|---|
| `-C dir` | Repository directory (default `.`). |
| `--version` | Print the version and exit. |
| `-h`, `--help` | Print usage and exit. |

repodungeon needs an interactive terminal. If something goes wrong it prints
`repodungeon: <message>` to stderr and exits with code 2.

## The card

When the run ends, repodungeon prints a card to share:

```
+------------------------------------------+
| repodungeon: demo @ abcdef0              |
| slain by main.go on depth 1              |
| depth 1/1  level 1  turns 1              |
| slain: 0 files                           |
| loot: 0 TODO, 0 FIXME                    |
| boss: server.go, Devourer of Weekends    |
| 2026-10-23                               |
+------------------------------------------+
```

## Development

```sh
go build ./...
test -z "$(gofmt -l .)" && go vet ./... && go test ./...
```

Generated maps are pinned by golden files. After an intended generation change,
refresh them with `go test ./internal/dungeon/ -update` and review the diff.

See [CONTRIBUTING.md](CONTRIBUTING.md) for the workflow and checks,
[ARCHITECTURE.md](ARCHITECTURE.md) for how the code is organised, and
[docs/SPEC.md](docs/SPEC.md) for the exact rules.

## License

MIT. See [LICENSE](LICENSE).
