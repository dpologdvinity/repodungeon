# repodungeon v0.1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build `repodungeon`, a read-only Go roguelike generated deterministically from the HEAD commit of a git repo, played in a Bubble Tea TUI, ending with a shareable text card.

**Architecture:** `repo` scans the HEAD tree, blob contents, and churn through git. `dungeon` is pure: a SplitMix64 RNG seeded from HEAD, a directory-to-level/room plan, map carving, and entity placement. `game` is a pure state machine driven by key strings (`Apply`), with `Render` and `Card` producing plain text. `tui` is a thin Bubble Tea v2 model over `game`, and `cli` wires it with an injected `Env` (TTY check, TUI runner, clock).

**Tech Stack:** Go (`go 1.26.0`), `charm.land/bubbletea/v2` v2.1.0, stdlib `flag`/`os/exec`/`regexp`, GoReleaser v2, GitHub Actions.

**Spec:** `docs/SPEC.md`

## Global Constraints

- Module path `github.com/dpologdvinity/repodungeon`; go.mod says `go 1.26.0`.
- Only dependency: `charm.land/bubbletea/v2 v2.1.0` (import as `tea "charm.land/bubbletea/v2"`). It is imported only by `internal/tui`, `internal/cli`, and `main.go`.
- Read-only: git is only called with `rev-parse`, `ls-tree`, `cat-file --batch`, `log`. Every call is `git -C <dir> ...` with `LC_ALL=C` added.
- All randomness goes through `dungeon.RNG` (SplitMix64). Generation uses `NewRNG(seed)`, combat uses `NewRNG(seed + 1)`. Never use `math/rand`.
- No `time.Now()` outside `main.go`; the card date comes from `Env.Now`.
- Map 80 x 21, screen 80 x 24. Player starts with HP 30, ATK 3, Level 1. FOV radius² 25. Pack size 10.
- Errors go to stderr as `repodungeon: <message>`, exit 2. Normal end (win, death, quit) exits 0 after printing the card.
- Tests use only stdlib `testing`. Fixture repos use `internal/testrepo` (fixed dates, `GIT_CONFIG_GLOBAL=/dev/null`, `GIT_CONFIG_NOSYSTEM=1`).
- `gofmt -l .` prints nothing and `go vet ./...` is clean before every commit.
- Commits use Conventional Commits. Never commit `CLAUDE.md`, `.claude/`, `.superpowers/`, `.codex/`, `AGENTS.md`, `.mcp.json`, `docs/superpowers/`. Never push.
- LICENSE: MIT, `Copyright (c) 2026 Kaitlyn Bassford`.

## Review Focus

1. A repo where every file sits in the root (no directories): one level, one room, no stairs, the boss in the same room as the start. Test: Task 6 `TestGenerateSingleRoom`.
2. More than 9 directories at one depth: the extra ones must merge into `misc/`, not overflow the 3x3 cell grid. Test: Task 4 `TestPlanMisc`.
3. Very deep paths (more than 7 directory levels): must fold into depth 7, giving at most 8 levels. Test: Task 4 `TestPlanFoldsDeepDirs`.
4. Keys pressed after death or quit must not change the state (no moving corpses). Test: Task 8 `TestDeathIgnoresLaterKeys`.
5. Symlinks, submodules, huge blobs, and binary files must never become monsters or crash the scan. Test: Task 3 `TestScan`.

---

### Task 1: Scaffold, fixture repo helper, golden helper

**Files:**
- Create: `go.mod`, `main.go`, `LICENSE`, `.gitignore`, `.goreleaser.yaml`, `.github/workflows/ci.yml`
- Create: `internal/testrepo/testrepo.go`, `internal/testrepo/testrepo_test.go`
- Create: `internal/golden/golden.go`

**Interfaces:**
- Produces:
  - `testrepo.New(t *testing.T) *testrepo.Repo` (fields: `Dir string`)
  - `(*Repo).Write(path, content string)` — creates parent dirs, path is relative to `Dir`
  - `(*Repo).Remove(path string)`
  - `(*Repo).Commit(name, email, msg string) string` — `git add -A`, `git commit -q --allow-empty -m msg`, returns full HEAD hash; date = `Date(n)` then n++
  - `(*Repo).Git(args ...string) string` — runs git in `Dir` with the fixture env and date `Date(n)` (does not increment); `t.Fatalf` on error; returns stdout
  - `testrepo.Date(n int) time.Time` — `2024-01-15T12:00:00Z` plus `n` months (`AddDate(0, n, 0)`)
  - `golden.Assert(t *testing.T, name, got string)` — compares with `testdata/golden/<name>.txt`; with `-update` writes it

- [ ] **Step 1: Create the scaffold**

`go mod init github.com/dpologdvinity/repodungeon`, then set the `go` line to `go 1.26.0`. `main.go` for now: `package main; var version = "dev"; func main() {}`. `.gitignore`: `.env`, `dist/`. LICENSE: standard MIT text with `Copyright (c) 2026 Kaitlyn Bassford`.

`.goreleaser.yaml` (exact; validated with goreleaser 2.19):
```yaml
version: 2
project_name: repodungeon
builds:
  - main: .
    binary: repodungeon
    env: [CGO_ENABLED=0]
    goos: [linux, darwin, windows]
    goarch: [amd64, arm64]
    ldflags: ["-s -w -X main.version={{.Version}}"]
archives:
  - formats: [tar.gz]
    format_overrides:
      - goos: windows
        formats: [zip]
checksum:
  name_template: checksums.txt
```

`.github/workflows/ci.yml` (exact):
```yaml
name: ci
on: [push, pull_request]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v7
      - uses: actions/setup-go@v7
        with:
          go-version-file: go.mod
      - run: test -z "$(gofmt -l .)"
      - run: go vet ./...
      - run: go test ./...
```

- [ ] **Step 2: Write the failing test** in `internal/testrepo/testrepo_test.go`

```go
func TestCommitUsesFixedDatesAndAuthor(t *testing.T) {
	r := New(t)
	r.Write("a/b.txt", "hi\n")
	h1 := r.Commit("Ada", "ada@example.com", "first")
	r.Commit("Bob", "bob@example.com", "second")
	if len(h1) != 40 {
		t.Fatalf("hash %q", h1)
	}
	got := r.Git("log", "--reverse", "--format=%an|%ae|%aI|%cI|%cn")
	want := "Ada|ada@example.com|2024-01-15T12:00:00Z|2024-01-15T12:00:00Z|Fixture\n" +
		"Bob|bob@example.com|2024-02-15T12:00:00Z|2024-02-15T12:00:00Z|Fixture\n"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if !Date(12).Equal(time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)) {
		t.Fatal("Date(12)")
	}
}
```
(Verified with git 2.55: `%aI` prints `Z` for UTC dates passed as `2024-01-15T12:00:00Z`.)

- [ ] **Step 3: Run it to verify it fails**

Run: `go test ./internal/testrepo/`
Expected: FAIL, `undefined: New`.

- [ ] **Step 4: Implement `internal/testrepo/testrepo.go` and `internal/golden/golden.go`**

testrepo env for every git call: `os.Environ()` plus `GIT_CONFIG_GLOBAL=/dev/null`, `GIT_CONFIG_NOSYSTEM=1`, `GIT_AUTHOR_NAME`, `GIT_AUTHOR_EMAIL` (for `Git`, use `Fixture` / `fixture@example.com`), `GIT_COMMITTER_NAME=Fixture`, `GIT_COMMITTER_EMAIL=fixture@example.com`, `GIT_AUTHOR_DATE` and `GIT_COMMITTER_DATE` = `Date(n).Format(time.RFC3339)`. `New` runs `git init -q -b main`.

golden: `var update = flag.Bool("update", false, "rewrite golden files")`. Path `filepath.Join("testdata", "golden", name+".txt")`. On update: `MkdirAll`, write, return. Otherwise read; on mismatch `t.Errorf` showing both texts.

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/testrepo/ && go vet ./... && test -z "$(gofmt -l .)" && goreleaser check`
Expected: PASS; `1 configuration file(s) validated`. (`goreleaser check` needs a git repo; it is one.)

- [ ] **Step 6: Commit**

```bash
git add go.mod main.go LICENSE .gitignore .goreleaser.yaml .github/workflows/ci.yml internal/testrepo internal/golden
git commit -m "chore: scaffold module, CI, release config and test helpers"
```

---

### Task 2: RNG and stat formulas

**Files:**
- Create: `internal/dungeon/rng.go`, `internal/dungeon/stats.go`, `internal/dungeon/rng_test.go`

**Interfaces:**
- Produces: `type RNG struct{ s uint64 }`; `func NewRNG(seed uint64) *RNG`; `(*RNG).Next() uint64`; `(*RNG).IntN(n int) int`; `(*RNG).Perm(n int) []int`; `func SeedFromHash(hash string) uint64`; `func MonsterStats(lines, churn int) (hp, atk int)`; `func BossStats(lines, churn int) (hp, atk int)`; `func RoomSize(files int) (w, h int)`; `var Epithets = [8]string{...}` (SPEC §5.4, exact order); `func Epithet(seed uint64) string`; `func isqrt(n int) int`.

- [ ] **Step 1: Write the failing tests**

```go
func TestRNGVectors(t *testing.T) {
	r := NewRNG(0)
	for _, want := range []uint64{0xe220a8397b1dcdaf, 0x6e789e6aa1b965f4, 0x06c45d188009454f} {
		if got := r.Next(); got != want {
			t.Fatalf("Next %x want %x", got, want)
		}
	}
	if p := NewRNG(0).Perm(5); !slices.Equal(p, []int{2, 3, 1, 4, 0}) {
		t.Fatalf("Perm %v", p)
	}
	r = NewRNG(7)
	var got []int
	for i := 0; i < 5; i++ {
		got = append(got, r.IntN(10))
	}
	if !slices.Equal(got, []int{7, 4, 6, 3, 4}) {
		t.Fatalf("IntN %v", got)
	}
	if SeedFromHash("0123456789abcdef0123456789abcdef01234567") != 0x0123456789abcdef {
		t.Fatal("SeedFromHash")
	}
}

func TestStats(t *testing.T) {
	for _, c := range [][4]int{{0, 0, 2, 1}, {100, 0, 12, 1}, {100, 1, 12, 2}, {9, 3, 5, 3}, {10000, 1000, 60, 8}} {
		if hp, atk := MonsterStats(c[0], c[1]); hp != c[2] || atk != c[3] {
			t.Errorf("MonsterStats(%d,%d)=%d,%d", c[0], c[1], hp, atk)
		}
	}
	for _, c := range [][4]int{{100, 40, 50, 6}, {0, 0, 30, 4}, {10000, 100000, 100, 9}} {
		if hp, atk := BossStats(c[0], c[1]); hp != c[2] || atk != c[3] {
			t.Errorf("BossStats(%d,%d)=%d,%d", c[0], c[1], hp, atk)
		}
	}
	for _, c := range [][3]int{{0, 6, 3}, {9, 12, 4}, {100, 22, 5}} {
		if w, h := RoomSize(c[0]); w != c[1] || h != c[2] {
			t.Errorf("RoomSize(%d)=%d,%d", c[0], w, h)
		}
	}
	if Epithet(0) != "Devourer of Weekends" || Epithet(15) != "Whisperer of Flaky Tests" {
		t.Error("Epithet")
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/dungeon/`
Expected: FAIL, `undefined: NewRNG`.

- [ ] **Step 3: Implement** per SPEC §5.1 (the `Next` body is given there verbatim) and §5.4. `RoomSize` per SPEC §5.3 step 2. `isqrt(n) = int(math.Sqrt(float64(n)))`.

- [ ] **Step 4: Run to verify pass**

Run: `go test ./internal/dungeon/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/dungeon
git commit -m "feat(dungeon): add SplitMix64 RNG and stat formulas"
```

---

### Task 3: Scan the repo

**Files:**
- Create: `internal/repo/repo.go`, `internal/repo/repo_test.go`

**Interfaces:**
- Consumes: `testrepo` (tests).
- Produces: `type File struct{ Path string; Lines, Churn, Todos, Fixmes int; Binary, Skipped bool }`; `func (f File) Text() bool` (`!Binary && !Skipped`); `type Repo struct{ Name, Head string; Files []File }`; `func Scan(dir string) (*Repo, error)`; errors `ErrGitMissing` (`"git not found in PATH"`), `ErrNotRepo` (`"not a git repository"`), `ErrNoCommits` (`"repository has no commits"`); `type GitError struct{ Args []string; Stderr string }`; `const MaxBlob = 1 << 20`.

- [ ] **Step 1: Write the failing tests**

```go
func TestScan(t *testing.T) {
	r := testrepo.New(t)
	r.Write("README.md", "# demo\nTODO: one\nFIXME: two\nTODO three\nTODOS not counted\n")
	r.Write(".gitignore", "ignored.txt\n")
	r.Write("logo.png", "\x89PNG\x00\x00")
	r.Write("noeol.txt", "a\nb")
	r.Write("big.txt", strings.Repeat("a", MaxBlob+1))
	r.Write("src/main.go", "package main\n")
	if err := os.Symlink("README.md", filepath.Join(r.Dir, "link.md")); err != nil {
		t.Skip("symlinks unsupported")
	}
	first := r.Commit("Ada", "ada@example.com", "init")
	r.Write("src/main.go", "package main\n\nfunc main() {}\n")
	r.Commit("Ada", "ada@example.com", "two")
	r.Write("src/main.go", "package main\n\nfunc main() { println() }\n")
	r.Commit("Ada", "ada@example.com", "three")
	// a submodule entry (gitlink) without a real submodule checkout
	r.Git("update-index", "--add", "--cacheinfo", "160000,"+first+",sub")
	r.Git("commit", "-q", "-m", "add gitlink")
	head := strings.TrimSpace(r.Git("rev-parse", "HEAD"))
	r.Write("ignored.txt", "TODO\n")     // ignored, never committed
	r.Write("uncommitted.txt", "TODO\n") // untracked

	got, err := Scan(r.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != filepath.Base(r.Dir) || got.Head != head {
		t.Fatalf("name %q head %q", got.Name, got.Head)
	}
	var paths []string
	by := map[string]File{}
	for _, f := range got.Files {
		paths = append(paths, f.Path)
		by[f.Path] = f
	}
	if !slices.Equal(paths, []string{".gitignore", "README.md", "big.txt", "logo.png", "noeol.txt", "src/main.go"}) {
		t.Fatalf("paths %v", paths)
	}
	if f := by["README.md"]; f.Lines != 5 || f.Todos != 2 || f.Fixmes != 1 || f.Churn != 1 || !f.Text() {
		t.Errorf("README %+v", f)
	}
	if f := by["big.txt"]; !f.Skipped || f.Text() {
		t.Errorf("big %+v", f)
	}
	if f := by["logo.png"]; !f.Binary || f.Text() {
		t.Errorf("logo %+v", f)
	}
	if by["noeol.txt"].Lines != 2 {
		t.Errorf("noeol %+v", by["noeol.txt"])
	}
	if f := by["src/main.go"]; f.Churn != 3 || f.Lines != 3 {
		t.Errorf("main %+v", f)
	}
}

func TestScanErrors(t *testing.T) {
	if _, err := Scan(t.TempDir()); !errors.Is(err, ErrNotRepo) {
		t.Errorf("not repo: %v", err)
	}
	if _, err := Scan(testrepo.New(t).Dir); !errors.Is(err, ErrNoCommits) {
		t.Errorf("empty: %v", err)
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/repo/`
Expected: FAIL, `undefined: Scan`.

- [ ] **Step 3: Implement `Scan`** per SPEC §4 steps 1–6. For `cat-file --batch`: start the command with `StdinPipe` and `StdoutPipe`, write all hashes from a goroutine and close stdin, read with a `bufio.Reader`: `ReadString('\n')` for the header, `io.ReadFull` for the body, then one `ReadByte` for the trailing newline. Regexps `\bTODO\b` and `\bFIXME\b` are package-level `regexp.MustCompile`.

- [ ] **Step 4: Run to verify pass**

Run: `go test ./internal/repo/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/repo
git commit -m "feat(repo): scan HEAD tree, line counts, TODOs and churn"
```

---

### Task 4: Directory plan (levels and rooms)

**Files:**
- Create: `internal/dungeon/plan.go`, `internal/dungeon/plan_test.go`

**Interfaces:**
- Consumes: `repo.File` (Task 3).
- Produces: `type RoomSpec struct{ Name string; Files []repo.File }`; `func Plan(files []repo.File) [][]RoomSpec`; `const MaxDepth = 7`, `MaxRooms = 9`.

- [ ] **Step 1: Write the failing tests**

```go
func tf(path string) repo.File { return repo.File{Path: path, Lines: 1} }

func names(rs []RoomSpec) []string {
	var out []string
	for _, r := range rs {
		out = append(out, r.Name)
	}
	return out
}

func TestPlanLevels(t *testing.T) {
	bin := tf("assets/logo.png")
	bin.Binary = true
	p := Plan([]repo.File{tf("README.md"), tf("go.mod"), tf("main.go"), bin,
		tf("internal/game/game.go"), tf("internal/repo/repo.go"), tf("docs/SPEC.md")})
	if len(p) != 3 {
		t.Fatalf("levels %d", len(p))
	}
	if !slices.Equal(names(p[0]), []string{"/"}) || !slices.Equal(names(p[1]), []string{"docs/", "internal/"}) ||
		!slices.Equal(names(p[2]), []string{"internal/game/", "internal/repo/"}) {
		t.Fatalf("%v %v %v", names(p[0]), names(p[1]), names(p[2]))
	}
	if len(p[0][0].Files) != 3 || p[0][0].Files[2].Path != "main.go" || len(p[1][1].Files) != 0 {
		t.Fatalf("files %+v", p[0][0].Files)
	}
}

func TestPlanFoldsDeepDirs(t *testing.T) {
	p := Plan([]repo.File{tf("a/b/c/d/e/f/g/h/i/deep.txt")})
	if len(p) != 8 || !slices.Equal(names(p[7]), []string{"a/b/c/d/e/f/g/"}) || len(p[7][0].Files) != 1 {
		t.Fatalf("%d levels, last %v", len(p), names(p[len(p)-1]))
	}
}

func TestPlanMisc(t *testing.T) {
	var fs []repo.File
	for i := 0; i < 12; i++ {
		for j := 0; j <= i; j++ {
			fs = append(fs, tf(fmt.Sprintf("d%02d/f%02d.go", i, j)))
		}
	}
	p := Plan(fs)
	want := []string{"d04/", "d05/", "d06/", "d07/", "d08/", "d09/", "d10/", "d11/", "misc/"}
	if !slices.Equal(names(p[1]), want) || len(p[1][8].Files) != 10 {
		t.Fatalf("%v misc=%d", names(p[1]), len(p[1][len(p[1])-1].Files))
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/dungeon/ -run Plan`
Expected: FAIL, `undefined: Plan`.

- [ ] **Step 3: Implement `Plan`** per SPEC §5.2. Only `File.Text()` files count. Files inside each `RoomSpec` are sorted by path (the `misc/` room too).

- [ ] **Step 4: Run to verify pass**

Run: `go test ./internal/dungeon/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/dungeon/plan.go internal/dungeon/plan_test.go
git commit -m "feat(dungeon): map directories to levels and rooms"
```

---

### Task 5: Map carving, Ascii, FromArt

**Files:**
- Create: `internal/dungeon/gen.go`, `internal/dungeon/gen_test.go`, `internal/dungeon/testdata/golden/level_4_rooms.txt`

**Interfaces:**
- Consumes: `RNG`, `RoomSize` (Task 2), `RoomSpec` (Task 4).
- Produces: all types of SPEC §5.5 (`Point`, `Tile` constants, `Room`, `Monster`, `ItemKind`/`Potion`/`Scroll`, `Item`, `Level`, `Dungeon`); `func (l *Level) At(p Point) Tile` (Rock outside bounds); `func (l *Level) Set(p Point, t Tile)`; `func Walkable(t Tile) bool`; `func Glyph(t Tile) rune`; `func (r Room) Center() Point`; `func (r Room) Contains(p Point) bool` (outer rectangle); `func (r Room) Interior() []Point` (row-major); `func buildLevel(rooms []RoomSpec, last bool, rng *RNG) *Level`; `func Ascii(l *Level) string`; `func FromArt(levels ...[]string) *Dungeon`; `const MapW, MapH = 80, 21`.

- [ ] **Step 1: Write the failing tests**

```go
func specs(sizes ...int) []RoomSpec {
	var out []RoomSpec
	for i, n := range sizes {
		rs := RoomSpec{Name: fmt.Sprintf("r%d/", i)}
		for j := 0; j < n; j++ {
			rs.Files = append(rs.Files, repo.File{Path: fmt.Sprintf("r%d/f%d", i, j), Lines: 1})
		}
		out = append(out, rs)
	}
	return out
}

// reach returns every walkable tile reachable from start (4-neighbour BFS).
func reach(l *Level, start Point) map[Point]bool {
	seen := map[Point]bool{start: true}
	q := []Point{start}
	for len(q) > 0 {
		p := q[0]
		q = q[1:]
		for _, d := range []Point{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			n := Point{p.X + d.X, p.Y + d.Y}
			if !seen[n] && Walkable(l.At(n)) {
				seen[n] = true
				q = append(q, n)
			}
		}
	}
	return seen
}

func TestFromArtAsciiRoundTrip(t *testing.T) {
	art := []string{"-------", "|@.+#>|", "-------"}
	d := FromArt(art)
	l := d.Levels[0]
	if got := Ascii(l); got != strings.Join(art, "\n")+"\n" {
		t.Fatalf("%q", got)
	}
	if l.Start != (Point{1, 1}) || !l.HasStairs || l.Stairs != (Point{5, 1}) || l.At(Point{3, 1}) != Door || d.Seed != 42 {
		t.Fatalf("%+v", l)
	}
}

func TestBuildLevelInvariants(t *testing.T) {
	for n := 1; n <= 9; n++ {
		for seed := uint64(0); seed < 20; seed++ {
			sizes := make([]int, n)
			for i := range sizes {
				sizes[i] = (i * 37) % 120
			}
			for _, last := range []bool{false, true} {
				l := buildLevel(specs(sizes...), last, NewRNG(seed))
				if l.W != MapW || l.H != MapH || len(l.Rooms) != n {
					t.Fatalf("n=%d seed=%d size %dx%d rooms %d", n, seed, l.W, l.H, len(l.Rooms))
				}
				r := reach(l, l.Start)
				for i := range l.Tiles {
					p := Point{i % l.W, i / l.W}
					if Walkable(l.Tiles[i]) && !r[p] {
						t.Fatalf("n=%d seed=%d: %v unreachable\n%s", n, seed, p, Ascii(l))
					}
				}
				if l.Start != l.Rooms[0].Center() || l.HasStairs == last {
					t.Fatalf("n=%d seed=%d start/stairs wrong", n, seed)
				}
				if !last {
					want := l.Rooms[n-1].Center()
					if n == 1 {
						rm := l.Rooms[0]
						want = Point{rm.X + rm.W, rm.Y + rm.H}
					}
					if l.Stairs != want || l.At(want) != Stairs {
						t.Fatalf("n=%d seed=%d stairs %v want %v", n, seed, l.Stairs, want)
					}
				}
			}
		}
	}
}

func TestBuildLevelGolden(t *testing.T) {
	golden.Assert(t, "level_4_rooms", Ascii(buildLevel(specs(0, 9, 30, 100), false, NewRNG(1))))
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/dungeon/ -run 'FromArt|BuildLevel'`
Expected: FAIL, `undefined: FromArt`.

- [ ] **Step 3: Implement** per SPEC §5.3 steps 1–4, §5.5, §5.6. RNG call order must be exactly: `Perm(9)`, then `ox, oy` per room in order, then one `IntN(2)` per corridor in order. `Ascii` draws tiles, then items (`!`/`?`), then living monsters (their `Glyph`), then `@` at `Start`; trailing spaces trimmed; `"\n"` after each row.

- [ ] **Step 4: Generate the golden and inspect it**

Run: `go test ./internal/dungeon/ -run BuildLevelGolden -update && cat internal/dungeon/testdata/golden/level_4_rooms.txt`
Expected: 21 lines (blank lines at the bottom are empty), 4 walled rooms joined by `#` corridors with `+` doors, one `@` and one `>`. No line longer than 80.

- [ ] **Step 5: Run to verify pass**

Run: `go test ./internal/dungeon/`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/dungeon
git commit -m "feat(dungeon): carve rooms, corridors and stairs; add Ascii and FromArt"
```

---

### Task 6: Generate the full dungeon with monsters, loot, and boss

**Files:**
- Modify: `internal/dungeon/gen.go`, `internal/dungeon/gen_test.go`
- Create: `internal/dungeon/testdata/golden/generated.txt`

**Interfaces:**
- Consumes: `repo.Repo` (Task 3), `Plan` (Task 4), `buildLevel` (Task 5), stats (Task 2).
- Produces: `func Generate(r *repo.Repo) (*Dungeon, error)`; `var ErrNoFiles = errors.New("no text files to fight")`; `func (k ItemKind) Name() string` (`"TODO potion"` / `"FIXME scroll"`); `func (k ItemKind) Glyph() rune` (`'!'` / `'?'`).

- [ ] **Step 1: Write the failing tests**

```go
func sampleRepo() *repo.Repo {
	f := func(p string, lines, churn, todos, fixmes int) repo.File {
		return repo.File{Path: p, Lines: lines, Churn: churn, Todos: todos, Fixmes: fixmes}
	}
	return &repo.Repo{Name: "demo", Head: "0123456789abcdef0123456789abcdef01234567", Files: []repo.File{
		f("README.md", 40, 5, 2, 0), f("go.mod", 5, 2, 0, 0), f("main.go", 120, 30, 1, 1),
		f("internal/game/game.go", 300, 12, 4, 0), f("internal/game/render.go", 80, 4, 0, 3),
		f("internal/repo/repo.go", 150, 6, 0, 0), f("docs/SPEC.md", 400, 3, 0, 0),
		{Path: "assets/logo.png", Binary: true},
		f("a/b/c/d/e/f/g/h/deep.txt", 1, 1, 0, 0),
	}}
}

func TestGenerateDeterministicAndBoss(t *testing.T) {
	d1, err := Generate(sampleRepo())
	if err != nil {
		t.Fatal(err)
	}
	d2, _ := Generate(sampleRepo())
	if !reflect.DeepEqual(d1, d2) {
		t.Fatal("not deterministic")
	}
	if d1.Seed != 0x0123456789abcdef || d1.Short != "0123456" || d1.Repo != "demo" || len(d1.Levels) != 8 ||
		d1.BossName != "main.go, Whisperer of Flaky Tests" {
		t.Fatalf("%+v", d1)
	}
	last := d1.Levels[7]
	b := last.Monsters[len(last.Monsters)-1]
	if !b.Boss || b.Name != d1.BossName || b.HP != 50 || b.MaxHP != 50 || b.ATK != 5 || b.Glyph != '&' || last.HasStairs {
		t.Fatalf("boss %+v", b)
	}
	for i, l := range d1.Levels[:7] {
		if !l.HasStairs {
			t.Fatalf("level %d has no stairs", i)
		}
	}
	other := sampleRepo()
	other.Head = "f" + other.Head[1:]
	d3, _ := Generate(other)
	if reflect.DeepEqual(d1.Levels[0].Tiles, d3.Levels[0].Tiles) && reflect.DeepEqual(d1.Levels[1].Tiles, d3.Levels[1].Tiles) {
		t.Fatal("different seed gave the same map")
	}
}

func TestGenerateEntities(t *testing.T) {
	d, _ := Generate(sampleRepo())
	l0 := d.Levels[0]
	var mons, srcs []string
	for _, m := range l0.Monsters {
		mons = append(mons, m.Name)
	}
	for _, it := range l0.Items {
		srcs = append(srcs, it.Kind.Name()+":"+it.Source)
	}
	if !slices.Equal(mons, []string{"README.md", "go.mod"}) {
		t.Fatalf("monsters %v", mons)
	}
	if !slices.Equal(srcs, []string{"TODO potion:README.md", "TODO potion:README.md", "TODO potion:main.go", "FIXME scroll:main.go"}) {
		t.Fatalf("items %v", srcs)
	}
	if m := l0.Monsters[0]; m.HP != 8 || m.ATK != 4 || m.Glyph != 'r' {
		t.Fatalf("README monster %+v", m)
	}
	for i, l := range d.Levels {
		used := map[Point]bool{l.Start: true}
		if l.HasStairs {
			used[l.Stairs] = true
		}
		r := reach(l, l.Start)
		check := func(p Point, what string) {
			if used[p] || l.At(p) != Floor || !r[p] {
				t.Fatalf("level %d: %s at %v overlaps, is not floor, or is unreachable", i, what, p)
			}
			used[p] = true
		}
		for _, m := range l.Monsters {
			check(m.Pos, m.Name)
		}
		for _, it := range l.Items {
			check(it.Pos, it.Kind.Name())
		}
	}
	golden.Assert(t, "generated", asciiAll(d))
}

func asciiAll(d *Dungeon) string {
	var b strings.Builder
	for i, l := range d.Levels {
		fmt.Fprintf(&b, "=== level %d ===\n%s", i, Ascii(l))
	}
	return b.String()
}

func TestGenerateSingleRoom(t *testing.T) {
	d, err := Generate(&repo.Repo{Name: "x", Head: strings.Repeat("0", 40), Files: []repo.File{
		{Path: "a.go", Lines: 10, Churn: 1}, {Path: "b.go", Lines: 10, Churn: 5}}})
	if err != nil || len(d.Levels) != 1 || d.Levels[0].HasStairs {
		t.Fatalf("%v %+v", err, d)
	}
	ms := d.Levels[0].Monsters
	if len(ms) != 2 || ms[0].Name != "a.go" || !ms[1].Boss || ms[1].Name != "b.go, Devourer of Weekends" {
		t.Fatalf("%+v", ms)
	}
}

func TestGenerateNoFiles(t *testing.T) {
	_, err := Generate(&repo.Repo{Head: strings.Repeat("0", 40), Files: []repo.File{{Path: "x.png", Binary: true}}})
	if !errors.Is(err, ErrNoFiles) {
		t.Fatalf("%v", err)
	}
}
```
README monster: `HP = min(60, 2 + isqrt(40)) = 8`, `ATK = min(8, 1 + bits.Len(5)) = 4`, glyph `r`.

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/dungeon/ -run Generate`
Expected: FAIL, `undefined: Generate`.

- [ ] **Step 3: Implement `Generate`**

Order (one generation RNG for the whole dungeon): `seed = SeedFromHash(Head)`; pick the boss file (SPEC §5.4); `Plan`; for each level `d`: `buildLevel(rooms, d == L-1, rng)`, then entities for each room per SPEC §5.3 step 5 (monsters, potions, scrolls; boss last on the last level). `BossName` = `path.Base(boss.Path) + ", " + Epithet(seed)`. `Short = Head[:7]`.

- [ ] **Step 4: Generate the golden and inspect it**

Run: `go test ./internal/dungeon/ -run GenerateEntities -update && cat internal/dungeon/testdata/golden/generated.txt`
Expected: 8 `=== level N ===` sections; level 0 has one room with `@`, `>`, `r`, `g`, three `!`, one `?`; level 7 has `&` and no `>`.

- [ ] **Step 5: Run to verify pass**

Run: `go test ./internal/dungeon/`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/dungeon
git commit -m "feat(dungeon): place monsters, loot and the boss"
```

---

### Task 7: Game state, movement, field of view, screen

**Files:**
- Create: `internal/game/game.go`, `internal/game/render.go`, `internal/game/game_test.go`

**Interfaces:**
- Consumes: `dungeon` types, `FromArt`, `NewRNG` (Tasks 2, 5, 6).
- Produces: `Mode` (`Play`, `Inventory`), `Outcome` (`Running`, `Won`, `Died`, `Quit`), `Player`, `State` exactly as SPEC §6; `func New(d *dungeon.Dungeon) *State`; `(*State).Apply(key string)`; `Done() bool`; `Visible(p dungeon.Point) bool`; `Render() string`; `(*State).level() *dungeon.Level`.

- [ ] **Step 1: Write the failing tests**

```go
func mon(name string, x, y, hp, atk int) dungeon.Monster {
	return dungeon.Monster{Name: name, Glyph: 'm', Pos: dungeon.Point{X: x, Y: y}, HP: hp, MaxHP: hp, ATK: atk}
}

func keys(s *State, ks ...string) {
	for _, k := range ks {
		s.Apply(k)
	}
}

func TestMoveAndWalls(t *testing.T) {
	s := New(dungeon.FromArt([]string{"-------", "|@....|", "|.....|", "-------"}))
	keys(s, "l", "l")
	if s.P.Pos != (dungeon.Point{X: 3, Y: 1}) || s.Turn != 2 {
		t.Fatalf("pos %v turn %d", s.P.Pos, s.Turn)
	}
	keys(s, "k") // wall: no move, no turn
	if s.P.Pos.Y != 1 || s.Turn != 2 {
		t.Fatalf("wall bump moved or took a turn")
	}
	keys(s, "down", "j", "x", ".")
	if s.P.Pos != (dungeon.Point{X: 3, Y: 2}) || s.Turn != 4 {
		t.Fatalf("pos %v turn %d", s.P.Pos, s.Turn)
	}
	s = New(dungeon.FromArt([]string{"|@......."}))
	if !s.Visible(dungeon.Point{X: 6, Y: 0}) || s.Visible(dungeon.Point{X: 7, Y: 0}) {
		t.Fatal("FOV radius must be 5")
	}
}

func TestRender(t *testing.T) {
	d := dungeon.FromArt([]string{"-------", "|@....|", "-------"})
	l := d.Levels[0]
	l.Rooms = []dungeon.Room{{Name: "/", X: 0, Y: 0, W: 5, H: 1}}
	l.Items = []dungeon.Item{{Kind: dungeon.Potion, Source: "README.md", Pos: dungeon.Point{X: 3, Y: 1}}}
	l.Monsters = []dungeon.Monster{mon("main.go", 5, 1, 4, 2)}
	s := New(d)
	want := "Welcome to demo! Slay server.go, Devourer of Weekends on depth 1.\n" +
		"------\n|@.!.m|\n------\n" + // corners (6,0) and (6,2) are outside radius 5
		strings.Repeat("\n", 18) +
		"Depth 1/1  HP 30/30  ATK 3  LV 1  XP 0/20  T 0  /\n" +
		"hjkl/arrows move  . wait  > descend  i inventory  q quit\n"
	if got := s.Render(); got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
	if !s.level().Monsters[0].Awake {
		t.Fatal("visible monster must wake in New")
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/game/`
Expected: FAIL, `undefined: New`.

- [ ] **Step 3: Implement** `New`, the key table of SPEC §7.1 (movement, `.`, `i`, `esc`, `q`/`ctrl+c`; attacks, items, and stairs may be stubs that later tasks fill in), the turn order of SPEC §7.2, FOV per SPEC §7.4, and `Render` per SPEC §7.7. Keep `seen` as one `[]bool` of `W*H` per level.

- [ ] **Step 4: Run to verify pass**

Run: `go test ./internal/game/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/game
git commit -m "feat(game): add state, movement, field of view and screen render"
```

---

### Task 8: Combat, monster AI, levelling, win, death

**Files:**
- Modify: `internal/game/game.go`, `internal/game/game_test.go`

**Interfaces:**
- Consumes: Task 7.
- Produces: `(*State).attack(m *dungeon.Monster)`, `(*State).monsterPhase()`, `(*State).gainXP(n int)` (internal; behavior per SPEC §7.3).

- [ ] **Step 1: Write the failing tests**

All rolls come from the combat RNG `NewRNG(42 + 1)` (FromArt seed 42). Its first four `IntN(3)` results are `1, 1, 0, 2`.

```go
func TestCombatSequence(t *testing.T) {
	d := dungeon.FromArt([]string{"-------", "|@....|", "-------"})
	d.Levels[0].Monsters = []dungeon.Monster{mon("main.go", 3, 1, 4, 2)}
	s := New(d)
	keys(s, "l") // move next to it; it hits back: 2 + 1 - 1 = 2
	if s.P.Pos.X != 2 || s.P.HP != 28 || !slices.Equal(s.Msgs, []string{"main.go hits you for 2."}) {
		t.Fatalf("1: %+v %v", s.P, s.Msgs)
	}
	keys(s, "l") // player 3+1-1=3 -> monster HP 1; monster 2+0-1=1
	if s.level().Monsters[0].HP != 1 || s.P.HP != 27 ||
		!slices.Equal(s.Msgs, []string{"You hit main.go for 3.", "main.go hits you for 1."}) {
		t.Fatalf("2: %v", s.Msgs)
	}
	keys(s, "l") // player 3+2-1=4 -> slain
	if s.level().Monsters[0].HP > 0 || s.Kills != 1 || s.P.XP != 4 || s.Turn != 3 ||
		!slices.Equal(s.Msgs, []string{"You hit main.go for 4.", "You slay main.go!"}) {
		t.Fatalf("3: %v", s.Msgs)
	}
}

func TestLevelUpLoop(t *testing.T) {
	d := dungeon.FromArt([]string{"|@...|"})
	m := mon("main.go", 2, 0, 1, 1)
	m.MaxHP = 65
	d.Levels[0].Monsters = []dungeon.Monster{m}
	s := New(d)
	keys(s, "l")
	if s.P.Level != 3 || s.P.XP != 5 || s.P.MaxHP != 40 || s.P.HP != 40 || s.P.ATK != 5 ||
		!slices.Equal(s.Msgs[2:], []string{"You feel stronger! Level 2.", "You feel stronger! Level 3."}) {
		t.Fatalf("%+v %v", s.P, s.Msgs)
	}
}

func TestBossWin(t *testing.T) {
	d := dungeon.FromArt([]string{"|@...|"})
	b := mon("server.go, Devourer of Weekends", 2, 0, 1, 5)
	b.Boss, b.MaxHP, b.Glyph = true, 30, '&'
	d.Levels[0].Monsters = []dungeon.Monster{b}
	s := New(d)
	keys(s, "l")
	want := []string{"You hit server.go, Devourer of Weekends for 3.", "You slay server.go, Devourer of Weekends!",
		"You feel stronger! Level 2.", "server.go, Devourer of Weekends is defeated! You win!"}
	if s.Outcome != Won || !s.Done() || !slices.Equal(s.Msgs, want) {
		t.Fatalf("%v %v", s.Outcome, s.Msgs)
	}
}

func TestMonsterChase(t *testing.T) {
	d := dungeon.FromArt([]string{"-------------", "|@..........|", "|...........|", "|...........|", "-------------"})
	d.Levels[0].Monsters = []dungeon.Monster{mon("a", 5, 2, 5, 1), mon("b", 3, 3, 5, 1), mon("c", 10, 3, 5, 1)}
	s := New(d)
	keys(s, ".")
	ms := s.level().Monsters
	if ms[0].Pos != (dungeon.Point{X: 4, Y: 2}) || ms[1].Pos != (dungeon.Point{X: 2, Y: 3}) ||
		ms[2].Pos != (dungeon.Point{X: 10, Y: 3}) || ms[2].Awake {
		t.Fatalf("%v %v %v awake=%v", ms[0].Pos, ms[1].Pos, ms[2].Pos, ms[2].Awake)
	}
}

func TestDeathIgnoresLaterKeys(t *testing.T) {
	d := dungeon.FromArt([]string{"|@...|"})
	d.Levels[0].Monsters = []dungeon.Monster{mon("main.go", 2, 0, 10, 50)}
	s := New(d)
	keys(s, ".")
	if s.Outcome != Died || s.P.HP != 0 || s.KilledBy != "main.go" || s.Turn != 1 ||
		!slices.Equal(s.Msgs, []string{"main.go hits you for 50.", "You die..."}) {
		t.Fatalf("%v %+v %v", s.Outcome, s.P, s.Msgs)
	}
	keys(s, "h", "l", ".")
	if s.P.Pos.X != 1 || s.Turn != 1 || len(s.Msgs) != 2 {
		t.Fatal("state changed after death")
	}
	if !strings.HasSuffix(s.Render(), "Game over. Press any key.\n") {
		t.Fatal("game over line")
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/game/`
Expected: FAIL (attacks and monster phase not implemented).

- [ ] **Step 3: Implement** SPEC §7.2 attack branch and §7.3 exactly. A slain monster keeps its entry with `HP <= 0` and is ignored everywhere (drawing, blocking, acting).

- [ ] **Step 4: Run to verify pass**

Run: `go test ./internal/game/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/game
git commit -m "feat(game): add bump combat, monster chase AI, levelling, win and death"
```

---

### Task 9: Items, inventory, stairs, quit

**Files:**
- Modify: `internal/game/game.go`, `internal/game/render.go`, `internal/game/game_test.go`

**Interfaces:**
- Consumes: Tasks 7–8; `ItemKind.Name()` (Task 6).
- Produces: pickup, `use(i int)`, descend, quit behavior per SPEC §7.1, §7.5, §7.6; inventory screen per SPEC §7.7.

- [ ] **Step 1: Write the failing tests**

```go
func TestItemsAndInventory(t *testing.T) {
	d := dungeon.FromArt([]string{"|@....|"})
	d.Levels[0].Items = []dungeon.Item{
		{Kind: dungeon.Potion, Source: "README.md", Pos: dungeon.Point{X: 2, Y: 0}},
		{Kind: dungeon.Scroll, Source: "main.go", Pos: dungeon.Point{X: 3, Y: 0}},
	}
	s := New(d)
	keys(s, "l")
	if len(s.P.Inv) != 1 || s.LootTodo != 1 || !slices.Equal(s.Msgs, []string{"You pick up a TODO potion."}) {
		t.Fatalf("%v", s.Msgs)
	}
	keys(s, "l")
	s.P.HP = 25
	keys(s, "i")
	lines := strings.Split(s.Render(), "\n")
	if s.Mode != Inventory || s.Turn != 2 || lines[1] != "Inventory (a-j to use, esc to close)" ||
		lines[2] != "a) TODO potion (from README.md)" || lines[3] != "b) FIXME scroll (from main.go)" {
		t.Fatalf("inventory screen:\n%s", s.Render())
	}
	keys(s, "a")
	if s.P.HP != 30 || len(s.P.Inv) != 1 || s.Mode != Play || s.Turn != 3 ||
		!slices.Equal(s.Msgs, []string{"You drink a TODO potion. +5 HP."}) {
		t.Fatalf("%+v %v", s.P, s.Msgs)
	}
	keys(s, "i", "b") // no item at index 1: nothing, stay in inventory
	if s.Mode != Inventory || s.Turn != 3 {
		t.Fatal("empty slot must do nothing")
	}
	keys(s, "a")
	if s.P.ATK != 4 || len(s.P.Inv) != 0 || s.Turn != 4 || !slices.Equal(s.Msgs, []string{"You read a FIXME scroll. ATK +1."}) {
		t.Fatalf("%v", s.Msgs)
	}
	keys(s, "i")
	if strings.Split(s.Render(), "\n")[2] != "(empty)" {
		t.Fatal("empty inventory line")
	}
	keys(s, "esc")
	if s.Mode != Play {
		t.Fatal("esc must close inventory")
	}
}

func TestPackFull(t *testing.T) {
	d := dungeon.FromArt([]string{"|@..|"})
	d.Levels[0].Items = []dungeon.Item{{Kind: dungeon.Potion, Source: "x", Pos: dungeon.Point{X: 2, Y: 0}}}
	s := New(d)
	for i := 0; i < 10; i++ {
		s.P.Inv = append(s.P.Inv, dungeon.Item{Kind: dungeon.Scroll})
	}
	keys(s, "l")
	if len(s.P.Inv) != 10 || len(s.level().Items) != 1 || !slices.Equal(s.Msgs, []string{"Your pack is full."}) {
		t.Fatalf("%v", s.Msgs)
	}
}

func TestStairs(t *testing.T) {
	s := New(dungeon.FromArt([]string{"-----", "|@.>|", "-----"}, []string{"-----", "|.@.|", "-----"}))
	keys(s, ">")
	if s.Turn != 0 || !slices.Equal(s.Msgs, []string{"There are no stairs here."}) {
		t.Fatalf("%v", s.Msgs)
	}
	keys(s, "l", "l", ">")
	if s.Depth != 1 || s.P.Pos != (dungeon.Point{X: 2, Y: 1}) || s.Turn != 3 ||
		!slices.Equal(s.Msgs, []string{"You descend to depth 2."}) || !strings.Contains(s.Render(), "Depth 2/2") {
		t.Fatalf("depth %d pos %v msgs %v", s.Depth, s.P.Pos, s.Msgs)
	}
}

func TestQuit(t *testing.T) {
	for _, k := range []string{"q", "ctrl+c"} {
		s := New(dungeon.FromArt([]string{"|@..|"}))
		keys(s, k, "l")
		if s.Outcome != Quit || !s.Done() || s.P.Pos.X != 1 || s.Turn != 0 {
			t.Fatalf("%s: %v", k, s.Outcome)
		}
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/game/`
Expected: FAIL (items, inventory use, and stairs not implemented).

- [ ] **Step 3: Implement** per SPEC §7.1, §7.5, §7.6, §7.7 (inventory rows).

- [ ] **Step 4: Run to verify pass**

Run: `go test ./internal/game/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/game
git commit -m "feat(game): add loot pickup, inventory, stairs and quitting"
```

---

### Task 10: Summary card

**Files:**
- Modify: `internal/game/render.go`, `internal/game/game_test.go`

**Interfaces:**
- Produces: `func (s *State) Card(now time.Time) string`.

- [ ] **Step 1: Write the failing test**

```go
func TestCard(t *testing.T) {
	d := dungeon.FromArt([]string{"|@...|"})
	d.Levels[0].Monsters = []dungeon.Monster{mon("main.go", 2, 0, 10, 50)}
	s := New(d)
	keys(s, ".")
	now := time.Date(2026, 10, 23, 9, 0, 0, 0, time.UTC)
	want := "+------------------------------------------+\n" +
		"| repodungeon: demo @ abcdef0              |\n" +
		"| slain by main.go on depth 1              |\n" +
		"| depth 1/1  level 1  turns 1              |\n" +
		"| slain: 0 files                           |\n" +
		"| loot: 0 TODO, 0 FIXME                    |\n" +
		"| boss: server.go, Devourer of Weekends    |\n" +
		"| 2026-10-23                               |\n" +
		"+------------------------------------------+\n"
	if got := s.Card(now); got != want {
		t.Fatalf("got\n%s", got)
	}
	q := New(dungeon.FromArt([]string{"|@..|"}))
	keys(q, "q")
	if !strings.Contains(q.Card(now), "| fled the dungeon on depth 1") {
		t.Fatal("quit outcome")
	}
	w := New(dungeon.FromArt([]string{"|@..|"}))
	w.Outcome = Won
	if !strings.Contains(w.Card(now), "| VICTORY! the boss is slain") {
		t.Fatal("win outcome")
	}
	long := New(dungeon.FromArt([]string{"|@..|"}))
	long.D.BossName = strings.Repeat("x", 100)
	for _, line := range strings.Split(strings.TrimSuffix(long.Card(now), "\n"), "\n") {
		if len(line) != 44 {
			t.Fatalf("line width %d: %q", len(line), line)
		}
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/game/ -run Card`
Expected: FAIL, `s.Card undefined`.

- [ ] **Step 3: Implement `Card`** per SPEC §7.8. Cut text to 40 runes before padding.

- [ ] **Step 4: Run to verify pass**

Run: `go test ./internal/game/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/game
git commit -m "feat(game): add shareable summary card"
```

---

### Task 11: Bubble Tea model

**Files:**
- Create: `internal/tui/tui.go`, `internal/tui/tui_test.go`
- Modify: `go.mod`, `go.sum` (`go get charm.land/bubbletea/v2@v2.1.0`)

**Interfaces:**
- Consumes: `game.State` (Tasks 7–10).
- Produces: `type Model struct{ S *game.State }` with `Init() tea.Cmd`, `Update(tea.Msg) (tea.Model, tea.Cmd)`, `View() tea.View` per SPEC §8.

- [ ] **Step 1: Write the failing test**

```go
func press(m tea.Model, code rune, text string) (tea.Model, tea.Cmd) {
	return m.Update(tea.KeyPressMsg{Code: code, Text: text})
}

func TestModel(t *testing.T) {
	m := Model{S: game.New(dungeon.FromArt([]string{"|@..|"}))}
	if m.Init() != nil {
		t.Fatal("Init must return nil")
	}
	next, cmd := press(m, 'l', "l")
	if cmd != nil || next.(Model).S.P.Pos.X != 2 {
		t.Fatal("l must move right")
	}
	next, _ = next.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	if next.(Model).S.P.Pos.X != 1 {
		t.Fatal("left arrow must move left")
	}
	v := next.View()
	if v.Content != next.(Model).S.Render() || !v.AltScreen {
		t.Fatal("view")
	}
	_, cmd = press(next, 'q', "q")
	if cmd == nil {
		t.Fatal("q must quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("q must return tea.Quit")
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go get charm.land/bubbletea/v2@v2.1.0 && go test ./internal/tui/`
Expected: FAIL, `undefined: Model`.

- [ ] **Step 3: Implement** per SPEC §8.

- [ ] **Step 4: Run to verify pass**

Run: `go mod tidy && go test ./internal/tui/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add go.mod go.sum internal/tui
git commit -m "feat(tui): add Bubble Tea model over the game state"
```

---

### Task 12: CLI and main

**Files:**
- Create: `internal/cli/cli.go`, `internal/cli/cli_test.go`
- Modify: `main.go`

**Interfaces:**
- Consumes: `repo.Scan` (Task 3), `dungeon.Generate` (Task 6), `game.New`, `(*State).Card` (Tasks 7, 10), `tui.Model` (Task 11).
- Produces: `type Env struct{ Version string; IsTTY bool; RunTUI func(tea.Model) (tea.Model, error); Now func() time.Time }`; `func Run(args []string, stdout, stderr io.Writer, env Env) int`; `const usage` (SPEC §3, exact).

- [ ] **Step 1: Write the failing tests**

```go
func env(tty bool) Env {
	return Env{Version: "1.2.3", IsTTY: tty,
		Now: func() time.Time { return time.Date(2026, 10, 23, 0, 0, 0, 0, time.UTC) },
		RunTUI: func(m tea.Model) (tea.Model, error) {
			m, _ = m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
			return m, nil
		}}
}

func run(e Env, args ...string) (string, string, int) {
	var o, er bytes.Buffer
	c := Run(args, &o, &er, e)
	return o.String(), er.String(), c
}

func goodRepo(t *testing.T) *testrepo.Repo {
	r := testrepo.New(t)
	r.Write("main.go", "package main\n// TODO: win\n")
	r.Write("src/util.go", "package src\n")
	r.Commit("Ada", "ada@example.com", "init")
	return r
}

func TestRunPrintsCard(t *testing.T) {
	r := goodRepo(t)
	out, errOut, code := run(env(true), "-C", r.Dir)
	if code != 0 || errOut != "" || !strings.HasPrefix(out, "+------------------------------------------+\n| repodungeon: ") ||
		!strings.Contains(out, "| fled the dungeon on depth 1") || !strings.Contains(out, "| 2026-10-23") {
		t.Fatalf("%d %q %q", code, out, errOut)
	}
}

func TestRunErrors(t *testing.T) {
	notRepo := t.TempDir()
	bin := testrepo.New(t)
	bin.Write("logo.png", "\x00\x01")
	bin.Commit("A", "a@x", "bin")
	cases := []struct {
		e    Env
		args []string
		msg  string
	}{
		{env(true), []string{"-C", notRepo}, "repodungeon: not a git repository: " + notRepo + "\n"},
		{env(true), []string{"-C", testrepo.New(t).Dir}, "repodungeon: repository has no commits\n"},
		{env(true), []string{"-C", bin.Dir}, "repodungeon: no text files to fight\n"},
		{env(false), []string{"-C", goodRepo(t).Dir}, "repodungeon: needs an interactive terminal\n"},
		{env(true), []string{"extra"}, "repodungeon: unexpected argument: extra\n"},
	}
	for _, c := range cases {
		if _, e, code := run(c.e, c.args...); code != 2 || !strings.HasPrefix(e, c.msg) {
			t.Errorf("%v: %d %q", c.args, code, e)
		}
	}
	if _, e, code := run(env(true), "--bogus"); code != 2 || !strings.Contains(e, "usage: repodungeon [-C dir]") {
		t.Errorf("bogus: %d %q", code, e)
	}
}

func TestHelpVersion(t *testing.T) {
	if o, _, c := run(env(true), "-h"); c != 0 || o != usage {
		t.Fatal("help")
	}
	if o, _, c := run(env(true), "--version"); c != 0 || o != "repodungeon 1.2.3\n" {
		t.Fatal("version")
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/cli/`
Expected: FAIL, `undefined: Run`.

- [ ] **Step 3: Implement**

`Run`: FlagSet `repodungeon` (`ContinueOnError`, output stderr, `fs.Usage = func() {}`; `ErrHelp` prints `usage` to stdout and returns 0; other parse errors print `usage` to stderr and return 2). Then the flow of SPEC §3: positional check, `repo.Scan`, `dungeon.Generate`, TTY check, `env.RunTUI(tui.Model{S: game.New(d)})`, then print `final.(tui.Model).S.Card(env.Now())` to stdout and return 0. Map errors to the SPEC §3 table (`<dir>` is the `-C` value as given). A `RunTUI` error prints `repodungeon: <err>` and returns 2.

`main.go`: `var version = "dev"`; `isTTY := charDev(os.Stdin) && charDev(os.Stdout)` where `charDev(f)` is `fi, err := f.Stat(); err == nil && fi.Mode()&os.ModeCharDevice != 0`; `RunTUI: func(m tea.Model) (tea.Model, error) { return tea.NewProgram(m).Run() }`; `Now: time.Now`; `os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr, env))`.

- [ ] **Step 4: Run to verify pass**

Run: `go test ./...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add main.go internal/cli
git commit -m "feat(cli): wire scan, generation, TUI and card output"
```

---

### Task 13: README, release workflow, final verification

**Files:**
- Create: `README.md`, `.github/workflows/release.yml`

- [ ] **Step 1: Write `.github/workflows/release.yml`** (exact)

```yaml
name: release
on:
  push:
    tags: ["v*"]
permissions:
  contents: write
jobs:
  goreleaser:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v7
        with:
          fetch-depth: 0
      - uses: actions/setup-go@v7
        with:
          go-version-file: go.mod
      - uses: goreleaser/goreleaser-action@v7
        with:
          version: "~> v2"
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

- [ ] **Step 2: Write `README.md`**: pitch, install (`go install github.com/dpologdvinity/repodungeon@latest`, GitHub releases), controls (SPEC §7.1), how the repo maps to the dungeon (SPEC §5.2–§5.4 in plain words: directories are rooms, depth is level, files are monsters with HP from lines and ATK from churn, TODO potions, FIXME scrolls, boss = most-changed file), the example card from SPEC §7.8, MIT line.

- [ ] **Step 3: Full verification**

Run: `test -z "$(gofmt -l .)" && go vet ./... && go test ./... && goreleaser check && go build -o /tmp/repodungeon . && /tmp/repodungeon -C . < /dev/null`
Expected: all PASS; the last command prints `repodungeon: needs an interactive terminal` and exits 2. Then run `/tmp/repodungeon` in a real terminal on this repo: reach depth 2, use a potion, finish the run, and check the card prints and the terminal is restored. Run it twice on the same commit and confirm the first level looks identical. Note the result in the final report.

- [ ] **Step 4: Commit**

```bash
git add README.md .github/workflows/release.yml
git commit -m "docs: add README and release workflow"
```
