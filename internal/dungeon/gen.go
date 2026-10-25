package dungeon

import (
	"errors"
	"path"
	"sort"
	"strings"
	"unicode"

	"github.com/dpologdvinity/repodungeon/internal/repo"
)

// ErrNoFiles is returned by Generate when the repo has no text file, so there
// is nothing to fight.
var ErrNoFiles = errors.New("no text files to fight")

// MapW and MapH are the size of every level map in tiles. Rooms sit on a 3x3
// grid of cells, each 26 wide and 7 tall (see buildLevel), plus a 1-column
// left margin: 1 + 3*26 = 79 <= 80 and 3*7 = 21. See docs/SPEC.md section 5.3.
const MapW, MapH = 80, 21

// Point is a map position. X grows to the right and Y grows downwards.
type Point struct{ X, Y int }

// Tile is one map cell kind. The values index tileGlyphs, so their order is
// part of the format.
type Tile uint8

const (
	// Rock is solid, empty stone. It is the zero value of a new map.
	Rock Tile = iota
	// WallH is a horizontal room wall (top or bottom edge).
	WallH
	// WallV is a vertical room wall (left or right edge).
	WallV
	// Floor is a room interior tile.
	Floor
	// Corridor is a carved passage between rooms.
	Corridor
	// Door is a wall tile that a corridor has cut through.
	Door
	// Stairs leads down to the next level.
	Stairs
)

// tileGlyphs maps a Tile value to its display character, by index.
const tileGlyphs = " -|.#+>"

// Glyph returns the display character of t.
func Glyph(t Tile) rune { return rune(tileGlyphs[t]) }

// Walkable reports whether a creature can stand on t. Every tile from Floor
// upwards is walkable; Rock and walls are not.
func Walkable(t Tile) bool { return t >= Floor }

// Room is a walled rectangle: X, Y is the outer top-left, W x H the interior.
// The outer size is therefore (W+2) x (H+2), including the walls.
type Room struct {
	// Name is the room title, copied from its RoomSpec.
	Name string
	// X and Y are the outer top-left corner; W and H are the interior size.
	X, Y, W, H int
}

// Center returns the middle interior tile of r (rounded down and right).
// Corridors start and end here, and so do the start and the stairs.
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

// Monster is a file turned into an enemy.
type Monster struct {
	// Name is the display name: the file base name, or for the boss the base
	// name plus an epithet. Path is the repo path of the source file.
	Name, Path string
	// Glyph is the map character: see monsterGlyph. The boss uses '&'.
	Glyph rune
	// Pos is the tile the monster stands on.
	Pos Point
	// HP is the current hit points; MaxHP is the starting value.
	// ATK is the damage per hit.
	HP, MaxHP, ATK int
	// Boss marks the final boss. Awake is set by the game when the monster
	// notices the player; generation always leaves it false.
	Boss, Awake bool
}

// ItemKind is the kind of a pickup.
type ItemKind uint8

const (
	// Potion comes from a TODO comment.
	Potion ItemKind = iota
	// Scroll comes from a FIXME comment.
	Scroll
	// Weapon comes from tool files or guaranteed floor spawn.
	Weapon
)

// WeaponArchetype is the combat style and attack behavior of a weapon.
type WeaponArchetype uint8

const (
	// Sword is standard melee with reliable bonus damage.
	Sword WeaponArchetype = iota
	// Spear has 2-tile reach without stepping into melee range.
	Spear
	// Hammer deals heavy blunt damage and knocks back enemies.
	Hammer
	// Dagger deals critical damage against unalert or backstabbed enemies.
	Dagger
)

// String returns the name of the weapon archetype.
func (a WeaponArchetype) String() string {
	switch a {
	case Sword:
		return "Sword"
	case Spear:
		return "Spear"
	case Hammer:
		return "Hammer"
	case Dagger:
		return "Dagger"
	default:
		return "Weapon"
	}
}

// Name returns the name of the weapon archetype.
func (a WeaponArchetype) Name() string {
	return a.String()
}

// Item is a pickup lying on a floor tile.
type Item struct {
	// Kind is the item type.
	Kind ItemKind
	// Source is the repo path of the file whose TODO or FIXME or tool made it.
	Source string
	// Pos is the tile the item lies on.
	Pos Point
	// Archetype is the weapon combat archetype when Kind == Weapon.
	Archetype WeaponArchetype
	// BonusATK is the attack bonus provided when equipped.
	BonusATK int
}

// Name returns the display name of the item.
func (it Item) Name() string {
	if it.Kind == Weapon {
		return it.Archetype.String()
	}
	return it.Kind.Name()
}

// Level is one map with its rooms and contents.
type Level struct {
	// W and H are the map size in tiles.
	W, H int
	// Tiles holds W*H tiles; the tile at (x, y) is Tiles[y*W+x].
	Tiles []Tile // row-major
	// Rooms are the placed rooms, in plan order.
	Rooms []Room
	// Monsters lists all enemies. Per room they follow plan order; the boss,
	// if any, is last.
	Monsters []Monster // boss, if any, is last
	// Items lists all pickups.
	Items []Item
	// Start is where the player arrives: the center of the first room.
	Start Point
	// Stairs is the down staircase. It is only meaningful if HasStairs is true.
	Stairs Point
	// HasStairs is false on the last level, which has the boss instead.
	HasStairs bool
}

// Dungeon is the whole generated game world. See docs/SPEC.md section 5.5.
type Dungeon struct {
	// Seed is the RNG seed taken from the head commit hash.
	Seed uint64
	// Repo is the repo name.
	Repo string // repo name
	// Short is the abbreviated head hash (first 7 characters).
	Short string // Head[:7]
	// BossName is the boss display name, for example "main.go, the Unrefactored".
	BossName string
	// Levels are the maps, from the top (root directory) downwards.
	Levels []*Level
}

// At returns the tile at p. Positions outside the map read as Rock.
func (l *Level) At(p Point) Tile {
	if p.X < 0 || p.Y < 0 || p.X >= l.W || p.Y >= l.H {
		return Rock
	}
	return l.Tiles[p.Y*l.W+p.X]
}

// Set stores t at p. Positions outside the map are ignored.
func (l *Level) Set(p Point, t Tile) {
	if p.X >= 0 && p.Y >= 0 && p.X < l.W && p.Y < l.H {
		l.Tiles[p.Y*l.W+p.X] = t
	}
}

// carve opens p for a corridor. Rock becomes Corridor and a wall becomes a
// Door. Other tiles (floor, doors, stairs) are left as they are.
func (l *Level) carve(p Point) {
	switch l.At(p) {
	case Rock:
		l.Set(p, Corridor)
	case WallH, WallV:
		l.Set(p, Door)
	}
}

// step returns the sign of to-from: 1, -1 or 0.
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
//
// The map is a 3x3 grid of cells, each 26 wide and 7 tall, with a 1-column
// left margin. Cell c (0 to 8) has column c%3 and row c/3. Room i goes in cell
// perm[i], so rooms never overlap. Inside the cell the room is shifted by a
// random offset (ox, oy) that keeps its outer box in the cell. Each room is
// then linked to the next one by an L-shaped corridor between their centers;
// a coin flip picks horizontal-first or vertical-first. Rooms are therefore
// chained in plan order and every walkable tile is reachable from Start.
// Start is the center of the first room. If this is not the last level, the
// stairs go at the center of the last room, or at the bottom-right interior
// corner when there is only one room (that is not the start tile). The RNG
// call order is part of the format; do not change it. See docs/SPEC.md
// section 5.3.
func buildLevel(rooms []RoomSpec, last bool, rng *RNG) *Level {
	l := &Level{W: MapW, H: MapH, Tiles: make([]Tile, MapW*MapH)}
	n := len(rooms)
	// Always draw a full permutation, even for fewer rooms, so the call count
	// does not depend on n.
	perm := rng.Perm(MaxRooms)
	for i, rs := range rooms {
		w, h := RoomSize(len(rs.Files))
		c := perm[i]
		// Offsets: free slack inside the 26 x 7 cell after the outer box.
		ox, oy := rng.IntN(26-(w+2)+1), rng.IntN(7-(h+2)+1)
		// Cell origin is (1 + col*26, row*7).
		r := Room{Name: rs.Name, X: 1 + (c%3)*26 + ox, Y: (c/3)*7 + oy, W: w, H: h}
		// Draw walls on the border and floor inside.
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
	// Link each room to the next. Carving through walls makes doors.
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
			// Single room: Start is the center, so use another tile.
			r := l.Rooms[0]
			l.Stairs = Point{r.X + r.W, r.Y + r.H}
		}
		l.Set(l.Stairs, Stairs)
	}
	return l
}

// Ascii draws tiles, items, living monsters and the start (@), rows trimmed.
//
// Later layers overwrite earlier ones: tiles, then items, then monsters, then
// the start marker. Dead monsters (HP <= 0) are skipped. Trailing spaces are
// cut from each row and every row ends with a newline. It is used for golden
// tests and debugging.
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
//
// Each argument is one level. Characters use the tileGlyphs table. A '>' sets
// the level stairs. The dungeon has fixed metadata (Seed 42, repo "demo").
// Rows may differ in length; missing cells are Rock. It panics on an unknown
// character. See docs/SPEC.md section 5.6.
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

// Name returns the display name of k.
func (k ItemKind) Name() string {
	switch k {
	case Potion:
		return "TODO potion"
	case Scroll:
		return "FIXME scroll"
	case Weapon:
		return "weapon"
	default:
		return "item"
	}
}

// Glyph returns the map character of k: '!' for potions, '?' for scrolls, ')' for weapons.
func (k ItemKind) Glyph() rune {
	switch k {
	case Potion:
		return '!'
	case Scroll:
		return '?'
	case Weapon:
		return ')'
	default:
		return '?'
	}
}

// Generate builds the whole dungeon from a scanned repo.
//
// The boss is the text file with the most churn; ties go to more lines, then
// to the smaller path. The seed comes from the head hash. Levels come from
// Plan, and each level is built and then populated using one shared RNG, so
// the call order (all of level 0, then level 1, and so on; within a level the
// layout first, then each room in order) is part of the format. The boss sits
// in the last room of the last level. It returns ErrNoFiles if the repo has no
// text file. See docs/SPEC.md sections 5.1 to 5.4.
func Generate(r *repo.Repo) (*Dungeon, error) {
	// Pick the boss: highest churn, then lines, then smallest path.
	var boss *repo.File
	for i := range r.Files {
		f := &r.Files[i]
		if !f.Text() {
			continue
		}
		if boss == nil || f.Churn > boss.Churn ||
			f.Churn == boss.Churn && (f.Lines > boss.Lines || f.Lines == boss.Lines && f.Path < boss.Path) {
			boss = f
		}
	}
	if boss == nil {
		return nil, ErrNoFiles
	}
	seed := SeedFromHash(r.Head)
	d := &Dungeon{Seed: seed, Repo: r.Name, Short: r.Head[:7],
		BossName: path.Base(boss.Path) + ", " + Epithet(seed)}
	rng := NewRNG(seed)
	plan := Plan(r.Files)
	for i, rooms := range plan {
		last := i == len(plan)-1
		l := buildLevel(rooms, last, rng)
		for ri, rs := range rooms {
			populate(l, l.Rooms[ri], rs, boss, last && ri == len(rooms)-1, d.BossName, rng)
		}
		ensureLevelWeapon(l, rooms, rng)
		d.Levels = append(d.Levels, l)
	}
	return d, nil
}

// populate fills one room of l with monsters, items and (maybe) the boss.
//
// Entities take tiles from a free list: the room interior minus the start
// tile and the stairs. Each placement picks a random index in the free list
// and removes that tile, so no two things share a tile. Order matters for the
// RNG: file monsters first, then potions, then scrolls, then the boss.
//
//   - Monsters: up to 6 files per room, the boss file excluded, ranked by
//     churn, then lines, then path.
//   - Potions: up to 3, one per TODO, in file order.
//   - Scrolls: up to 2, one per FIXME, in file order.
//   - Boss: only when hasBoss is set; it is appended last.
//
// See docs/SPEC.md section 5.4.
func populate(l *Level, room Room, rs RoomSpec, boss *repo.File, hasBoss bool, bossName string, rng *RNG) {
	// Never place anything on the start or the stairs.
	used := map[Point]bool{l.Start: true}
	if l.HasStairs {
		used[l.Stairs] = true
	}
	var free []Point
	for _, p := range room.Interior() {
		if !used[p] {
			free = append(free, p)
		}
	}
	// take removes and returns a random free tile.
	take := func() Point {
		if len(free) == 0 {
			return room.Center()
		}
		i := rng.IntN(len(free))
		p := free[i]
		free = append(free[:i], free[i+1:]...)
		return p
	}

	// The boss is not also a normal monster.
	var fs []repo.File
	for _, f := range rs.Files {
		if f.Path != boss.Path {
			fs = append(fs, f)
		}
	}
	sort.SliceStable(fs, func(i, j int) bool {
		a, b := fs[i], fs[j]
		if a.Churn != b.Churn {
			return a.Churn > b.Churn
		}
		if a.Lines != b.Lines {
			return a.Lines > b.Lines
		}
		return a.Path < b.Path
	})
	for _, f := range fs[:min(6, len(fs))] {
		hp, atk := MonsterStats(f.Lines, f.Churn)
		name := path.Base(f.Path)
		l.Monsters = append(l.Monsters, Monster{Name: name, Path: f.Path, Glyph: monsterGlyph(name),
			Pos: take(), HP: hp, MaxHP: hp, ATK: atk})
	}

	// One entry per TODO/FIXME occurrence, so a file can give several items.
	var todos, fixmes []string
	for _, f := range rs.Files {
		for i := 0; i < f.Todos; i++ {
			todos = append(todos, f.Path)
		}
		for i := 0; i < f.Fixmes; i++ {
			fixmes = append(fixmes, f.Path)
		}
	}
	for _, src := range todos[:min(3, len(todos))] {
		l.Items = append(l.Items, Item{Kind: Potion, Source: src, Pos: take()})
	}
	for _, src := range fixmes[:min(2, len(fixmes))] {
		l.Items = append(l.Items, Item{Kind: Scroll, Source: src, Pos: take()})
	}

	for _, f := range rs.Files {
		minFree := 0
		if hasBoss {
			minFree = 1
		}
		if len(free) <= minFree {
			break
		}
		if isToolFile(f.Path) {
			arch := weaponArchetypeFor(rng)
			bonus := weaponBonusFor(arch, rng)
			l.Items = append(l.Items, Item{
				Kind:      Weapon,
				Source:    f.Path,
				Pos:       take(),
				Archetype: arch,
				BonusATK:  bonus,
			})
		}
	}

	if hasBoss {
		hp, atk := BossStats(boss.Lines, boss.Churn)
		l.Monsters = append(l.Monsters, Monster{Name: bossName, Path: boss.Path, Glyph: '&',
			Pos: take(), HP: hp, MaxHP: hp, ATK: atk, Boss: true})
	}
}

// monsterGlyph is the first ASCII letter of name, lowercased; 'm' if none.
func monsterGlyph(name string) rune {
	for _, c := range name {
		if c < unicode.MaxASCII && unicode.IsLetter(c) {
			return unicode.ToLower(c)
		}
	}
	return 'm'
}

// isToolFile reports whether the file at p is a build/tool file that spawns a weapon.
func isToolFile(p string) bool {
	base := path.Base(p)
	lower := strings.ToLower(base)
	if lower == "makefile" || lower == "gnumakefile" || lower == "dockerfile" || lower == "containerfile" ||
		strings.HasPrefix(lower, "dockerfile.") || strings.HasSuffix(lower, ".dockerfile") {
		return true
	}
	if lower == "go.mod" || lower == "package.json" || lower == "cargo.toml" || lower == "pom.xml" || lower == "build.gradle" || lower == "cmakelists.txt" {
		return true
	}
	ext := path.Ext(lower)
	if ext == ".sh" || ext == ".bash" || ext == ".zsh" {
		return true
	}
	if strings.Contains(p, ".github/") || strings.Contains(p, ".circleci/") || lower == ".gitlab-ci.yml" || lower == ".travis.yml" {
		return true
	}
	return false
}

// weaponArchetypeFor picks an archetype using rng.
func weaponArchetypeFor(rng *RNG) WeaponArchetype {
	if rng == nil {
		return Sword
	}
	return WeaponArchetype(rng.IntN(4))
}

// weaponBonusFor returns the bonus ATK for a given archetype.
func weaponBonusFor(arch WeaponArchetype, rng *RNG) int {
	switch arch {
	case Sword:
		if rng != nil {
			return 2 + rng.IntN(3)
		}
		return 3
	case Spear:
		return 2
	case Hammer:
		return 3
	case Dagger:
		return 2
	default:
		return 2
	}
}

// ensureLevelWeapon guarantees that at least one weapon spawns on the level.
func ensureLevelWeapon(l *Level, rooms []RoomSpec, rng *RNG) {
	for _, it := range l.Items {
		if it.Kind == Weapon {
			return
		}
	}
	if len(l.Rooms) == 0 {
		return
	}
	used := map[Point]bool{l.Start: true}
	if l.HasStairs {
		used[l.Stairs] = true
	}
	for _, m := range l.Monsters {
		used[m.Pos] = true
	}
	for _, it := range l.Items {
		used[it.Pos] = true
	}
	for ri, r := range l.Rooms {
		var free []Point
		for _, p := range r.Interior() {
			if !used[p] {
				free = append(free, p)
			}
		}
		if len(free) > 0 {
			var pos Point
			if rng != nil {
				pos = free[rng.IntN(len(free))]
			} else {
				pos = free[0]
			}
			arch := weaponArchetypeFor(rng)
			bonus := weaponBonusFor(arch, rng)
			src := "floor"
			if ri < len(rooms) && len(rooms[ri].Files) > 0 {
				src = rooms[ri].Files[0].Path
			}
			l.Items = append(l.Items, Item{
				Kind:      Weapon,
				Source:    src,
				Pos:       pos,
				Archetype: arch,
				BonusATK:  bonus,
			})
			return
		}
	}
}
