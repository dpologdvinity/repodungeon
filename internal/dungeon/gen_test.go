package dungeon

import (
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/dpologdvinity/repodungeon/internal/golden"
	"github.com/dpologdvinity/repodungeon/internal/repo"
)

// specs builds room specs with the given file counts, named r0/, r1/, and so
// on. Each file has one line.
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

// TestFromArtAsciiRoundTrip proves FromArt and Ascii are inverses on a small
// map, and that FromArt sets the start, the stairs and the fixed metadata.
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

// TestBuildLevelInvariants proves for 1 to 9 rooms and many seeds that a
// level has the right size and room count, that every walkable tile is
// reachable from the start, and that the start and stairs sit where the spec
// says (no stairs on the last level).
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

// TestBuildLevelGolden pins the exact map of a fixed level against a golden
// file, so any change to the layout or the RNG call order is caught.
func TestBuildLevelGolden(t *testing.T) {
	golden.Assert(t, "level_4_rooms", Ascii(buildLevel(specs(0, 9, 30, 100), false, NewRNG(1))))
}

// sampleRepo returns a small fixed repo used by the Generate tests. It has a
// binary file, a deep directory and a clear boss (main.go).
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

// TestGenerateDeterministicAndBoss proves Generate gives equal results for
// equal input, picks the right boss with the right stats on the last level
// only, adds stairs on every other level, and changes the map with the seed.
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

// TestGenerateEntities proves the monsters and items of level 0 are the
// expected ones, and that on every level each entity lies on its own reachable
// floor tile, away from the start and the stairs. It also pins all levels
// against a golden file.
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

// asciiAll renders every level of d, each under a "=== level N ===" header.
func asciiAll(d *Dungeon) string {
	var b strings.Builder
	for i, l := range d.Levels {
		fmt.Fprintf(&b, "=== level %d ===\n%s", i, Ascii(l))
	}
	return b.String()
}

// TestGenerateSingleRoom proves a one-room repo gives one level with no
// stairs, with the boss placed after the other monster.
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

// TestGenerateNoFiles proves a repo with only binary files returns
// ErrNoFiles.
func TestGenerateNoFiles(t *testing.T) {
	_, err := Generate(&repo.Repo{Head: strings.Repeat("0", 40), Files: []repo.File{{Path: "x.png", Binary: true}}})
	if !errors.Is(err, ErrNoFiles) {
		t.Fatalf("%v", err)
	}
}
