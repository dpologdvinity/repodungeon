package dungeon

import (
	"fmt"
	"strings"
	"testing"

	"github.com/dpologdvinity/repodungeon/internal/golden"
	"github.com/dpologdvinity/repodungeon/internal/repo"
)

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
