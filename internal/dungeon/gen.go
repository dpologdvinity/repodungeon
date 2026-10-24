package dungeon

import "strings"

const MapW, MapH = 80, 21

type Point struct{ X, Y int }

type Tile uint8

const (
	Rock Tile = iota
	WallH
	WallV
	Floor
	Corridor
	Door
	Stairs
)

const tileGlyphs = " -|.#+>"

func Glyph(t Tile) rune { return rune(tileGlyphs[t]) }

func Walkable(t Tile) bool { return t >= Floor }

// Room is a walled rectangle: X, Y is the outer top-left, W x H the interior.
type Room struct {
	Name       string
	X, Y, W, H int
}

func (r Room) Center() Point { return Point{r.X + 1 + r.W/2, r.Y + 1 + r.H/2} }

// Contains reports whether p lies in the outer rectangle.
func (r Room) Contains(p Point) bool {
	return p.X >= r.X && p.X < r.X+r.W+2 && p.Y >= r.Y && p.Y < r.Y+r.H+2
}

// Interior lists the floor tiles in row-major order.
func (r Room) Interior() []Point {
	pts := make([]Point, 0, r.W*r.H)
	for y := r.Y + 1; y <= r.Y+r.H; y++ {
		for x := r.X + 1; x <= r.X+r.W; x++ {
			pts = append(pts, Point{x, y})
		}
	}
	return pts
}

type Monster struct {
	Name, Path     string
	Glyph          rune
	Pos            Point
	HP, MaxHP, ATK int
	Boss, Awake    bool
}

type ItemKind uint8

const (
	Potion ItemKind = iota
	Scroll
)

type Item struct {
	Kind   ItemKind
	Source string
	Pos    Point
}

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

func (l *Level) At(p Point) Tile {
	if p.X < 0 || p.Y < 0 || p.X >= l.W || p.Y >= l.H {
		return Rock
	}
	return l.Tiles[p.Y*l.W+p.X]
}

func (l *Level) Set(p Point, t Tile) {
	if p.X >= 0 && p.Y >= 0 && p.X < l.W && p.Y < l.H {
		l.Tiles[p.Y*l.W+p.X] = t
	}
}

func (l *Level) carve(p Point) {
	switch l.At(p) {
	case Rock:
		l.Set(p, Corridor)
	case WallH, WallV:
		l.Set(p, Door)
	}
}

func step(from, to int) int {
	switch {
	case to > from:
		return 1
	case to < from:
		return -1
	}
	return 0
}

// line carves the straight run from a to b (inclusive); a and b share an axis.
func (l *Level) line(a, b Point) {
	dx, dy := step(a.X, b.X), step(a.Y, b.Y)
	for p := a; ; p = (Point{p.X + dx, p.Y + dy}) {
		l.carve(p)
		if p == b {
			return
		}
	}
}

// buildLevel carves rooms and corridors. RNG order: Perm(9), per-room
// offsets, then one IntN(2) per corridor.
func buildLevel(rooms []RoomSpec, last bool, rng *RNG) *Level {
	l := &Level{W: MapW, H: MapH, Tiles: make([]Tile, MapW*MapH)}
	n := len(rooms)
	perm := rng.Perm(MaxRooms)
	for i, rs := range rooms {
		w, h := RoomSize(len(rs.Files))
		c := perm[i]
		ox, oy := rng.IntN(26-(w+2)+1), rng.IntN(7-(h+2)+1)
		r := Room{Name: rs.Name, X: 1 + (c%3)*26 + ox, Y: (c/3)*7 + oy, W: w, H: h}
		for y := r.Y; y < r.Y+h+2; y++ {
			for x := r.X; x < r.X+w+2; x++ {
				switch {
				case y == r.Y || y == r.Y+h+1:
					l.Set(Point{x, y}, WallH)
				case x == r.X || x == r.X+w+1:
					l.Set(Point{x, y}, WallV)
				default:
					l.Set(Point{x, y}, Floor)
				}
			}
		}
		l.Rooms = append(l.Rooms, r)
	}
	for i := 0; i < n-1; i++ {
		horizFirst := rng.IntN(2) == 0
		a, b := l.Rooms[i].Center(), l.Rooms[i+1].Center()
		if horizFirst {
			l.line(a, Point{b.X, a.Y})
			l.line(Point{b.X, a.Y}, b)
		} else {
			l.line(a, Point{a.X, b.Y})
			l.line(Point{a.X, b.Y}, b)
		}
	}
	l.Start = l.Rooms[0].Center()
	if !last {
		l.HasStairs = true
		l.Stairs = l.Rooms[n-1].Center()
		if n == 1 {
			r := l.Rooms[0]
			l.Stairs = Point{r.X + r.W, r.Y + r.H}
		}
		l.Set(l.Stairs, Stairs)
	}
	return l
}

// Ascii draws tiles, items, living monsters and the start (@), rows trimmed.
func Ascii(l *Level) string {
	grid := make([][]rune, l.H)
	for y := range grid {
		grid[y] = make([]rune, l.W)
		for x := range grid[y] {
			grid[y][x] = Glyph(l.Tiles[y*l.W+x])
		}
	}
	for _, it := range l.Items {
		grid[it.Pos.Y][it.Pos.X] = it.Kind.Glyph()
	}
	for _, m := range l.Monsters {
		if m.HP > 0 {
			grid[m.Pos.Y][m.Pos.X] = m.Glyph
		}
	}
	grid[l.Start.Y][l.Start.X] = '@'
	var b strings.Builder
	for _, row := range grid {
		b.WriteString(strings.TrimRight(string(row), " "))
		b.WriteByte('\n')
	}
	return b.String()
}

// FromArt builds a dungeon from ASCII rows (test helper). '@' is Floor + Start.
func FromArt(levels ...[]string) *Dungeon {
	d := &Dungeon{Seed: 42, Repo: "demo", Short: "abcdef0", BossName: "server.go, Devourer of Weekends"}
	for _, rows := range levels {
		l := &Level{H: len(rows)}
		for _, row := range rows {
			l.W = max(l.W, len([]rune(row)))
		}
		l.Tiles = make([]Tile, l.W*l.H)
		for y, row := range rows {
			for x, c := range []rune(row) {
				p := Point{x, y}
				if c == '@' {
					l.Start = p
					l.Set(p, Floor)
					continue
				}
				i := strings.IndexRune(tileGlyphs, c)
				if i < 0 {
					panic("FromArt: bad char " + string(c))
				}
				l.Set(p, Tile(i))
				if Tile(i) == Stairs {
					l.Stairs, l.HasStairs = p, true
				}
			}
		}
		d.Levels = append(d.Levels, l)
	}
	return d
}

func (k ItemKind) Name() string {
	if k == Potion {
		return "TODO potion"
	}
	return "FIXME scroll"
}

func (k ItemKind) Glyph() rune {
	if k == Potion {
		return '!'
	}
	return '?'
}
