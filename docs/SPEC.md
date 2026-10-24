# repodungeon — Specification (v0.1)

`repodungeon` turns the git repository you are standing in into a small
roguelike. Directories are rooms, each directory depth is a dungeon level,
files are monsters, TODO and FIXME comments are loot, and the boss is the file
that changed most often. The same commit always gives the same dungeon. Combat
is turn-based bump-to-attack with permadeath. When the run ends, a shareable
text card is printed.

This document is the source of truth for v0.1 behavior. `docs/PLAN.md` says how
to build it. If the plan and the spec disagree, the spec wins.

## 1. Goals and non-goals

### Goals (v0.1)
- `repodungeon`: play the dungeon of the repo in the current directory in a full-screen TUI.
- Deterministic: seed = first 16 hex digits of the HEAD commit hash. Same commit, same dungeon, same combat rolls for the same key presses.
- Only committed, tracked files (`git ls-tree HEAD`), so `.gitignore`d and untracked files never appear.
- Read-only: only `git` subprocesses that read.
- Game logic is a pure package, tested by feeding key strings and checking state.
- A summary card printed to stdout after the game.

### Non-goals (v0.1)
- No saving, no going back up stairs, no diagonal moves, no ranged attacks.
- No line-of-sight: field of view is a plain radius (§7.4).
- No terminal-size adaptation: the screen is a fixed 80x24.

## 2. Tech stack (fixed)

| Item | Choice | Why |
|---|---|---|
| Language | Go, `go 1.26.0` in go.mod (builds with 1.27) | Bubble Tea v2.1.0 requires go 1.26.0 |
| Module path | `github.com/dpologdvinity/repodungeon` | |
| TUI | `charm.land/bubbletea/v2` v2.1.0 | Input handling, alt screen, and rendering. Import path is `charm.land/bubbletea/v2` (the v2 vanity path), package name `tea`. |
| Flags | stdlib `flag` | |
| Randomness | own SplitMix64 (§5.1) | `math/rand/v2` does not promise identical `IntN`/`Perm` output across Go releases; 10 lines of SplitMix64 do |
| Tests | stdlib `testing`, golden files | |
| Release | GoReleaser v2 + GitHub Actions | |

Add no other dependencies (no lipgloss: colors are plain ANSI codes, §8). Versions verified against `proxy.golang.org` on 2026-10-23.

## 3. CLI

```
repodungeon [-C dir]
```

| Flag | Default | Meaning |
|---|---|---|
| `-C` | `.` | Repository directory. |
| `--version` | | Print `repodungeon <version>`, exit 0. |
| `-h`, `--help` | | Print usage, exit 0. |

Usage text (exact; stdout for `-h`, stderr after usage errors):
```
usage: repodungeon [-C dir]

A roguelike generated from your git repository.
Directories are rooms, files are monsters, TODOs are loot.

flags:
  -C dir      repository directory (default ".")
  --version   print version

keys: hjkl or arrows move, . wait, > descend, i inventory, ? help, q quit
```

Flow: parse flags; scan the repo (§4); generate the dungeon (§5); if stdin or stdout is not a terminal, fail; run the TUI (§8); print the card (§7.8) to stdout; exit 0.

| Condition | stderr | Code |
|---|---|---|
| bad flag | flag package error line, then usage | 2 |
| positional argument | `repodungeon: unexpected argument: <arg>` | 2 |
| git missing | `repodungeon: git not found in PATH` | 2 |
| not a repo | `repodungeon: not a git repository: <dir>` | 2 |
| no HEAD | `repodungeon: repository has no commits` | 2 |
| no text files | `repodungeon: no text files to fight` | 2 |
| other git failure | `repodungeon: git <args>: <first stderr line>` | 2 |
| not a terminal | `repodungeon: needs an interactive terminal` | 2 |
| TUI error | `repodungeon: <err>` | 2 |

## 4. Reading the repo (package `internal/repo`)

All git calls are `git -C <dir> <args>` with `LC_ALL=C` added.

1. `exec.LookPath("git")` fails: `ErrGitMissing`. `rev-parse --show-toplevel` fails: `ErrNotRepo`. `Name = filepath.Base(toplevel)`.
2. `rev-parse --verify --quiet HEAD` fails: `ErrNoCommits`. Else `Head` = the trimmed hash.
3. `ls-tree -r -z -l --full-tree HEAD`. Each NUL-terminated entry is `<mode> <type> <hash> <size>\t<path>` (size is space-padded: use `strings.Fields` on the part before the tab). Keep entries with type `blob` and mode not `120000` (symlinks). Submodules have type `commit` and are dropped.
4. Blobs with size > 1,048,576 bytes are `Skipped` (not read). The rest are read with one `cat-file --batch` process: write `<hash>\n` per blob to stdin, then read for each blob a header line `<hash> blob <size>\n`, then `size` bytes, then one `\n`.
5. For each read blob: `Binary` if a NUL byte is in the first 8000 bytes. Else `Lines` = count of `\n`, plus 1 if the content is non-empty and does not end in `\n`. `Todos` = matches of `\bTODO\b`, `Fixmes` = matches of `\bFIXME\b` (case-sensitive).
6. Churn: `log -z --no-merges --no-renames --max-count=5000 --format=%x1e --name-only HEAD`. Split on `\x1e`; in each chunk split on `\x00`, trim a leading `\n` from each field, and count every non-empty field as one touch of that path. `Churn` = touches, only for paths in the file list.

```go
type File struct {
    Path            string // repo-relative, forward slashes
    Lines, Churn    int
    Todos, Fixmes   int
    Binary, Skipped bool
}
type Repo struct {
    Name, Head string
    Files      []File // sorted by Path
}
func Scan(dir string) (*Repo, error)
```
A file is a **text file** when `!Binary && !Skipped`. Only text files become monsters, loot, or the boss.

## 5. Dungeon generation (package `internal/dungeon`, pure)

### 5.1 RNG

```go
type RNG struct{ s uint64 }
func NewRNG(seed uint64) *RNG
func (r *RNG) Next() uint64 // SplitMix64
func (r *RNG) IntN(n int) int // int(Next() % uint64(n)); n > 0
func (r *RNG) Perm(n int) []int
func SeedFromHash(hash string) uint64 // strconv.ParseUint(hash[:16], 16, 64)
```
```go
func (r *RNG) Next() uint64 {
    r.s += 0x9E3779B97F4A7C15
    z := r.s
    z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
    z = (z ^ (z >> 27)) * 0x94D049BB133111EB
    return z ^ (z >> 31)
}
```
`Perm(n)`: `p = [0, 1, ..., n-1]`; for `i = n-1` down to 1: `j = IntN(i+1)`; swap `p[i]`, `p[j]`.
Test vectors: `NewRNG(0).Next()` gives `0xe220a8397b1dcdaf`, `0x6e789e6aa1b965f4`, `0x06c45d188009454f`. `NewRNG(0).Perm(5)` = `[2 3 1 4 0]`.

Two generators: generation uses `NewRNG(seed)`, combat uses `NewRNG(seed + 1)`.

### 5.2 Directories to levels and rooms
- `dirOf(path)`: `path.Dir`, with `"."` meaning the root `""`. `depth("") = 0`, otherwise `strings.Count(dir, "/") + 1`.
- A file's **room dir** is its dir cut to at most 7 components (depth ≤ 7).
- Room dirs = every room dir of a text file, plus all their ancestors (including `""`). Level `d` holds the room dirs with depth `d`. Levels `L = max depth + 1` (1..8).
- On a level, sort dirs by path. If there are more than 9: rank dirs by (count of text files whose room dir is exactly that dir) desc, then path asc; the top 8 keep a room, and every other dir's files go to one extra room named `misc/`. Kept rooms are ordered by path, with `misc/` last.
- Room name: `dir + "/"`, or `/` for the root.

```go
type RoomSpec struct { Name string; Files []repo.File } // Files sorted by Path
func Plan(files []repo.File) [][]RoomSpec // [level][room]; non-text files ignored
```

### 5.3 Map
Each level is 80 x 21 tiles.

```go
type Tile uint8
const (Rock Tile = iota; WallH; WallV; Floor; Corridor; Door; Stairs)
```
Glyphs: Rock ` `, WallH `-`, WallV `|`, Floor `.`, Corridor `#`, Door `+`, Stairs `>`. Walkable: Floor, Corridor, Door, Stairs.

Per level `d`, with rooms `0..n-1` (`n <= 9`), using the generation RNG in exactly this order:
1. `perm = rng.Perm(9)`. Room `i` goes in cell `c = perm[i]`; cell origin `(1 + (c%3)*26, (c/3)*7)`.
2. For each room `i` in order: `fc = len(Files)`; interior `w = clamp(6 + 2*isqrt(fc), 6, 22)`, `h = clamp(3 + isqrt(fc)/2, 3, 5)`; `ox = rng.IntN(26 - (w+2) + 1)`; `oy = rng.IntN(7 - (h+2) + 1)`. The outer rectangle's top-left is `(originX + ox, originY + oy)`, size `(w+2) x (h+2)`. Top and bottom rows (corners included) are WallH, left and right columns (without corners) are WallV, the inside is Floor.
3. Corridors, for `i = 0 .. n-2`: `horizFirst = rng.IntN(2) == 0`. From `a = center(i)` to `b = center(i+1)`, where `center = (X+1+w/2, Y+1+h/2)`: if horizFirst, carve `(x, a.Y)` for x from a.X to b.X, then `(b.X, y)` for y from a.Y to b.Y; else carve `(a.X, y)` for y from a.Y to b.Y, then `(x, b.Y)` for x from a.X to b.X. Carving: Rock becomes Corridor, WallH/WallV become Door, everything else stays.
4. `Start = center(0)`. If `d < L-1`: stairs at `center(n-1)`, or, if `n == 1`, at the interior bottom-right `(X+w, Y+h)`. Set that tile to Stairs.
5. Entities, for each room `i` in order. The free list is the room's interior tiles in row-major order, minus Start, the stairs, and tiles already used. Each placement is `free[rng.IntN(len(free))]`, then removed from the list.
   - Monsters: the room's files minus the boss file, sorted by (Churn desc, Lines desc, Path asc), first 6, placed in that order.
   - Potions: `min(3, Σ Todos)`. Scrolls: `min(2, Σ Fixmes)`. The k-th potion's `Source` is the path of the k-th TODO when files are walked in path order and each file contributes `Todos` entries (same for scrolls with FIXMEs). Potions are placed before scrolls.
   - Boss: on the last level only, after the last room's other entities, placed in the last room.

Interior is at least 6x3 = 18 tiles; at most 14 are used (start, stairs, 6 monsters, 3 potions, 2 scrolls, boss).

### 5.4 Monsters and the boss
- `isqrt(n) = floor(sqrt(n))`.
- Monster: `HP = MaxHP = min(60, 2 + isqrt(Lines))`, `ATK = min(8, 1 + bits.Len(uint(Churn)))`. Name = `path.Base(Path)`. Glyph = the first ASCII letter of the name, lowercased; `m` if none.
- Boss file: the text file with the highest Churn; ties by Lines desc, then Path asc. `HP = clamp(20 + 3*isqrt(Lines), 30, 100)`, `ATK = clamp(3 + bits.Len(uint(Churn))/2, 4, 9)`. Glyph `&`. Name = `path.Base(Path) + ", " + Epithets[seed % 8]`:

```go
var Epithets = [8]string{
    "Devourer of Weekends", "Breaker of Builds", "Hoarder of Merge Conflicts",
    "the Unrefactored", "Lord of the Hotfix", "Bane of Code Review",
    "the Ever-Changing", "Whisperer of Flaky Tests",
}
```

### 5.5 Types

```go
type Point struct{ X, Y int }
type Room struct { Name string; X, Y, W, H int } // outer top-left; interior W x H
type Monster struct {
    Name, Path string
    Glyph      rune
    Pos        Point
    HP, MaxHP, ATK int
    Boss, Awake bool
}
type ItemKind uint8
const (Potion ItemKind = iota; Scroll)
type Item struct { Kind ItemKind; Source string; Pos Point }
type Level struct {
    W, H      int
    Tiles     []Tile // row-major
    Rooms     []Room
    Monsters  []Monster // boss, if any, is last
    Items     []Item
    Start     Point
    Stairs    Point
    HasStairs bool
}
type Dungeon struct {
    Seed     uint64
    Repo     string // repo name
    Short    string // Head[:7]
    BossName string
    Levels   []*Level
}
func Generate(r *repo.Repo) (*Dungeon, error) // ErrNoFiles when there are no text files
func Ascii(l *Level) string                   // tiles, then items (! potion, ? scroll), then living monsters, then @ at Start; rows trimmed, "\n" after each
func FromArt(levels ...[]string) *Dungeon     // test helper, §5.6
```

### 5.6 FromArt (test helper)
Builds a dungeon from ASCII rows: ` -|.#+>` map to the tiles of §5.3 (`>` also sets `Stairs`/`HasStairs`), `@` is Floor and sets `Start`. Any other char panics. `W` = longest row, `H` = number of rows. `Seed = 42`, `Repo = "demo"`, `Short = "abcdef0"`, `BossName = "server.go, Devourer of Weekends"`. No rooms, monsters, or items; tests add them.

## 6. Game state (package `internal/game`, pure, no Bubble Tea)

```go
type Mode int
const (Play Mode = iota; Inventory; Help)
type Outcome int
const (Running Outcome = iota; Won, Died, Quit)
type Player struct {
    Pos                        Point
    HP, MaxHP, ATK, Level, XP  int
    Inv                        []dungeon.Item
}
type State struct {
    D        *dungeon.Dungeon
    Depth    int // index into D.Levels
    P        Player
    Mode     Mode
    Outcome  Outcome
    Turn     int
    Msgs     []string
    Kills    int
    KilledBy string
    LootTodo, LootFixme int // items picked up
    seen     [][]bool       // per level, W*H
    rng      *dungeon.RNG   // combat RNG
}
func New(d *dungeon.Dungeon) *State
func (s *State) Apply(key string)
func (s *State) Done() bool          // Outcome != Running
func (s *State) Visible(p Point) bool
func (s *State) Render() string
func (s *State) Card(now time.Time) string
```

`New`: player `HP = MaxHP = 30`, `ATK = 3`, `Level = 1`, `XP = 0`, at level 0 `Start`. Combat RNG `NewRNG(D.Seed + 1)`. Compute FOV (§7.4). `Msgs = ["Welcome to <Repo>! Slay <BossName> on depth <len(Levels)>."]`.

## 7. Rules

### 7.1 Keys (strings are Bubble Tea `KeyPressMsg.String()` values)
`Apply(key)` first returns at once if `Done()`. Otherwise it clears `Msgs`, then:

| Mode | Key | Effect | Takes a turn |
|---|---|---|---|
| Play | `h`/`left`, `l`/`right`, `k`/`up`, `j`/`down` | move or attack (§7.2) | yes, unless the target is not walkable and has no monster |
| Play | `.` | wait | yes |
| Play | `>` | descend if on Stairs (§7.6), else message `There are no stairs here.` | descend: special (§7.6); else no |
| Play | `i` | Mode = Inventory | no |
| Play | `?` | Mode = Help | no |
| Help | any key except `q`, `ctrl+c` | Mode = Play (the key does nothing else) | no |
| Play, Inventory, Help | `q`, `ctrl+c` | Outcome = Quit | no |
| Inventory | `a`..`j` | if an item exists at that index (`a` = 0): use it (§7.5) and set Mode = Play; otherwise nothing | yes, if an item was used |
| Inventory | `esc`, `i` | Mode = Play | no |
| any | other | nothing | no |

### 7.2 Turn order (exact)
1. Player action.
2. If the action took a turn and `Outcome == Running`: monster phase (§7.3).
3. `Turn++` (only if the action took a turn).
4. Update FOV (§7.4), only if the action took a turn.

Move: target `t = Pos + delta`. If a living monster (`HP > 0`) is at `t`: attack it. Else if `t` is walkable: move there, then pick up an item at `t` (§7.5). Else nothing happens and no turn passes.

### 7.3 Combat and monster AI
- Damage roll: `dmg = max(1, atk + rng.IntN(3) - 1)` with the combat RNG. Rolls happen in action order: player first, then monsters in slice order.
- Player hits: `You hit <name> for <dmg>.` If the monster's HP drops to ≤ 0: `You slay <name>!`, `Kills++`, `XP += MaxHP`, then level up while `XP >= 20*Level`: `XP -= 20*Level`, `Level++`, `MaxHP += 5`, `HP = min(MaxHP, HP+5)`, `ATK++`, message `You feel stronger! Level <Level>.` If the slain monster is the boss: message `<name> is defeated! You win!`, `Outcome = Won`.
- Monster phase: for each monster in slice order with `HP > 0 && Awake`:
  - If `|dx| + |dy| == 1` to the player: attack. `<name> hits you for <dmg>.` If player HP ≤ 0: `HP = 0`, `Outcome = Died`, `KilledBy = <name>`, message `You die...`, end the phase.
  - Else step: `sx = sign(px - mx)`, `sy = sign(py - my)`. If `|px-mx| >= |py-my|`, try `(sx, 0)` then `(0, sy)`; else `(0, sy)` then `(sx, 0)`. A try is valid if its delta is non-zero, the target is walkable, and no living monster and not the player is there. Take the first valid try, or stay.

### 7.4 Field of view
`Visible(p)` is true when `(p.X-px)² + (p.Y-py)² <= 25`. Updating the FOV marks all visible tiles of the current level as seen and sets `Awake = true` on every visible living monster. Walls do not block sight (v0.1).

### 7.5 Items
- Pickup when moving onto an item: if `len(Inv) < 10`, move it from the level to `Inv`, message `You pick up a <TODO potion|FIXME scroll>.`, and increment `LootTodo` or `LootFixme`. Otherwise message `Your pack is full.` and the item stays.
- Potion: `heal = min(10, MaxHP - HP)`, `HP += heal`, message `You drink a TODO potion. +<heal> HP.`
- Scroll: `ATK++`, message `You read a FIXME scroll. ATK +1.`
- The used item is removed from `Inv`; the order of the others is kept.

### 7.6 Stairs
On `>` while standing on Stairs: `Depth++`, `Pos = Levels[Depth].Start`, message `You descend to depth <Depth+1>.`, `Turn++`, update FOV. No monster phase. There is no way back up.

### 7.7 Screen (`Render`, 80 x 24, plain text)
Each row has trailing spaces trimmed and is followed by `\n`.
- Row 0: `strings.Join(Msgs, " ")`, cut to 80 runes.
- Rows 1–21: map rows y = 0..20 of the current level. For a tile: if visible, draw the player `@`, else a living monster's glyph, else an item (`!` potion, `?` scroll), else the tile glyph. If not visible but seen: the tile glyph only. Otherwise a space. Tiles outside the level size are spaces.
- Row 22: `Depth <Depth+1>/<L>  HP <HP>/<MaxHP>  ATK <ATK>  LV <Level>  XP <XP>/<20*Level>  T <Turn>  <place>`, where place is the name of the first room whose outer rectangle contains the player, or `corridor`.
- Row 23: `hjkl/arrows move  . wait  > descend  i inventory  ? help  q quit`; when `Done()`: `Game over. Press any key.`; in Help mode: `Press any key to continue.`
- Help mode replaces rows 1–21 with the how-to-play text (`HOW TO PLAY`, `KEYS`, `THE MAP` sections; it names the boss and the number of levels). `cli.Run` calls `OpenHelp()` before the first frame, so the game starts on this screen.
- Inventory mode replaces rows 1–21: row 1 `Inventory (a-j to use, esc to close)`; then one row per item `<letter>) <TODO potion|FIXME scroll> (from <Source>)`; or `(empty)` if the pack is empty.

### 7.8 Card (`Card(now)`)
Width 44. Border `+` + 42 `-` + `+`. Each content line is `"| " + text + " |"`, with text cut to 40 runes and right-padded with spaces to 40.
The seven content lines, in order (each formatted with `fmt.Sprintf("| %-40s |", cut40(text))`):
1. `repodungeon: <Repo> @ <Short>`
2. `<outcome>`
3. `depth <Depth+1>/<L>  level <Level>  turns <Turn>`
4. `slain: <Kills> files`
5. `loot: <LootTodo> TODO, <LootFixme> FIXME`
6. `boss: <BossName>`
7. `<now.Format("2006-01-02")>`

Example (died on a one-level dungeon):
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
Outcome text: Won `VICTORY! the boss is slain`; Died `slain by <KilledBy> on depth <Depth+1>`; Quit or Running `fled the dungeon on depth <Depth+1>`. The card ends with `\n`.

## 8. TUI (package `internal/tui`)

```go
type Model struct{ S *game.State }
func (m Model) Init() tea.Cmd                           // nil
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd)
func (m Model) View() tea.View                          // tea.NewView(S.Render()), AltScreen = true
```
**Colors.** `View` adds ANSI SGR color codes to `S.Render()` (walls light gray, floor and corridors dark gray, doors yellow, stairs bold yellow, player bold green, monsters red, boss bold red, potions magenta, scrolls cyan; HP green/yellow/red by health; headings yellow). Colors are decoration only: with the escape codes removed, the text equals `S.Render()` exactly. Bubble Tea lowers them to the terminal's color depth and drops them when `NO_COLOR` is set. The end card is plain text.

`Update` on `tea.KeyPressMsg k`: if `S.Done()`, return `tea.Quit`. Else `S.Apply(k.String())`; if `S.Outcome == game.Quit`, return `tea.Quit`. Other messages: no-op.

## 9. Package layout and injection

```
repodungeon/
├── main.go                   // real Env, os.Exit(cli.Run(...))
├── internal/
│   ├── cli/cli.go
│   ├── repo/repo.go          // Scan
│   ├── dungeon/rng.go        // RNG, SeedFromHash
│   ├── dungeon/stats.go      // MonsterStats, BossStats, RoomSize, Epithets
│   ├── dungeon/plan.go       // Plan
│   ├── dungeon/gen.go        // types, Generate, Ascii, FromArt
│   ├── game/game.go          // State, New, Apply, rules
│   ├── game/render.go        // Render, Card
│   ├── tui/tui.go            // Model
│   ├── testrepo/testrepo.go  // tests only
│   └── golden/golden.go      // tests only
├── .goreleaser.yaml
├── .github/workflows/{ci.yml,release.yml}
├── README.md
└── LICENSE                   // MIT, Copyright (c) 2026 Kaitlyn Bassford
```
`dungeon` imports `repo` (types only) and the stdlib. `game` imports `dungeon`. Only `tui` and `main.go` import Bubble Tea.

```go
type Env struct {
    Version string
    IsTTY   bool                                   // stdin and stdout are both terminals (stdlib: Stat().Mode()&os.ModeCharDevice != 0)
    RunTUI  func(m tea.Model) (tea.Model, error)   // real: tea.NewProgram(m).Run()
    Now     func() time.Time                       // the injected clock, for the card date
}
func Run(args []string, stdout, stderr io.Writer, env Env) int
```

## 10. Testing
- Fixture repos via `internal/testrepo` (same helper as gitgarden and replay): `t.TempDir()`, `GIT_CONFIG_GLOBAL=/dev/null`, `GIT_CONFIG_NOSYSTEM=1`, committer `Fixture`, dates `2024-01-15T12:00:00Z` plus N months.
- Golden files with `-update`, per package.
- Unit tests: RNG vectors, stats formulas, `Plan` (folding, `misc/`), `Generate` invariants (bounds, reachability by BFS, no shared tiles, counts, determinism, boss on the last level) and a golden of every level, `Scan` (gitignore, uncommitted, binary, churn, TODO counts).
- Game tests drive `Apply` with key sequences on `FromArt` dungeons and assert positions, HP, messages, outcomes, and the exact `Render` and `Card` text.
- TUI test calls `Update` with `tea.KeyPressMsg` values.
- CI: `gofmt -l .` empty, `go vet ./...`, `go test ./...`.

## 11. Definition of done (v0.1)
- All of §3–§8 works as specified; tests pass.
- `go test ./...`, `go vet ./...`, `gofmt -l .` clean; `goreleaser check` passes.
- Manual run on a real repo: reach depth 2, use a potion, die or win, see the card.
- README with install, controls, the mapping rules, and an example card.
- `v0.1.0` tag builds linux/darwin/windows × amd64/arm64.

## 12. Name availability (checked 2026-10-23)
- GitHub `dpologdvinity/repodungeon`: free; 0 repos named `repodungeon` on GitHub.
- Homebrew formula and cask: free. npm: free.

## 13. Later (not v0.1)
- Line-of-sight FOV, diagonal moves, going back up.
- Monster special abilities by language (e.g. `.js` files multiply).
- `--seed` override and a daily-challenge mode.
- Saving the card to a file, and a high-score list in the user cache dir (never in the repo).
- Mimics for binary files.
