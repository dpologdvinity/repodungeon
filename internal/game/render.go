package game

import (
	"fmt"
	"strings"
	"time"

	"github.com/dpologdvinity/repodungeon/internal/dungeon"
)

const (
	// screenW is the screen width in columns (SPEC 7.7).
	screenW = 80
	// mapRows is the number of map rows; the screen is 1 message row + map + status + help = 24.
	mapRows = dungeon.MapH
	// helpLine is the last row while the run is going.
	helpLine = "hjkl/arrows move  . wait  > descend  i inventory  ? help  q quit"
	// overLine replaces helpLine once the run is over.
	overLine = "Game over. Press any key."
	// invHeader is the first row of the inventory screen under the message row.
	invHeader = "Inventory (a-j to use, esc to close)"
)

// cut truncates s to at most n runes. It counts runes, not bytes, so a
// multi-byte path is never split and rows never exceed the screen width.
func cut(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}

// Render draws the 80x24 screen as plain text.
func (s *State) Render() string {
	// rows are built without trailing spaces; the final loop also adds the newline.
	var rows []string
	rows = append(rows, cut(strings.Join(s.Msgs, " "), screenW))
	switch s.Mode {
	case Help:
		rows = append(rows, s.helpRows()...)
		for len(rows) < 1+mapRows {
			rows = append(rows, "")
		}
	case Inventory:
		rows = append(rows, invHeader)
		for i, it := range s.P.Inv {
			rows = append(rows, cut(fmt.Sprintf("%c) %s (from %s)", 'a'+i, it.Kind.Name(), it.Source), screenW))
		}
		if len(s.P.Inv) == 0 {
			rows = append(rows, "(empty)")
		}
		// Pad so the status row stays on screen row 23, as in Play mode.
		for len(rows) < 1+mapRows {
			rows = append(rows, "")
		}
	default:
		rows = append(rows, s.mapRows()...)
	}
	rows = append(rows, cut(s.status(), screenW))
	switch {
	case s.Done():
		rows = append(rows, overLine)
	case s.Mode == Help && s.helpPage < helpPages-1:
		rows = append(rows, fmt.Sprintf("Press any key for page %d of %d.", s.helpPage+2, helpPages))
	case s.Mode == Help:
		rows = append(rows, "Press any key to continue.")
	default:
		rows = append(rows, helpLine)
	}
	var b strings.Builder
	for _, r := range rows {
		b.WriteString(strings.TrimRight(r, " "))
		b.WriteByte('\n')
	}
	return b.String()
}

// mapRows draws the map area: mapRows rows of screenW runes, each cell from cell.
// Part of the screen outside the level is blank.
func (s *State) mapRows() []string {
	l := s.level()
	out := make([]string, mapRows)
	for y := range out {
		row := make([]rune, screenW)
		for x := range row {
			row[x] = s.cell(l, dungeon.Point{X: x, Y: y})
		}
		out[y] = string(row)
	}
	return out
}

// cell returns the glyph for tile p of level l.
// In view, the draw order is player, living monster, item, terrain.
// Out of view, remembered tiles show terrain only, so monsters and items
// vanish when they leave sight. Never-seen tiles and tiles off the level are blank.
func (s *State) cell(l *dungeon.Level, p dungeon.Point) rune {
	if p.X >= l.W || p.Y >= l.H {
		return ' '
	}
	if s.Visible(p) {
		if p == s.P.Pos {
			return '@'
		}
		if m := s.monsterAt(p); m != nil {
			return m.Glyph
		}
		for _, it := range l.Items {
			if it.Pos == p {
				return it.Kind.Glyph()
			}
		}
		return dungeon.Glyph(l.At(p))
	}
	if s.seen[s.Depth][p.Y*l.W+p.X] {
		return dungeon.Glyph(l.At(p))
	}
	return ' '
}

// place names the room the player stands in, or "corridor" outside all rooms.
func (s *State) place() string {
	for _, r := range s.level().Rooms {
		if r.Contains(s.P.Pos) {
			return r.Name
		}
	}
	return "corridor"
}

// status builds the status row. XP shows the threshold for the next level (20*Level).
func (s *State) status() string {
	return fmt.Sprintf("Depth %d/%d  HP %d/%d  ATK %d  LV %d  XP %d/%d  T %d  %s",
		s.Depth+1, len(s.D.Levels), s.P.HP, s.P.MaxHP, s.P.ATK, s.P.Level, s.P.XP, 20*s.P.Level, s.Turn, s.place())
}

// Card is the shareable end-of-run summary, 44 columns wide.
func (s *State) Card(now time.Time) string {
	var outcome string
	switch s.Outcome {
	case Won:
		outcome = "VICTORY! the boss is slain"
	case Died:
		outcome = fmt.Sprintf("slain by %s on depth %d", s.KilledBy, s.Depth+1)
	default:
		outcome = fmt.Sprintf("fled the dungeon on depth %d", s.Depth+1)
	}
	// 42 dashes plus two corners make the 44-column card.
	border := "+" + strings.Repeat("-", 42) + "+\n"
	lines := []string{
		fmt.Sprintf("repodungeon: %s @ %s", s.D.Repo, s.D.Short),
		outcome,
		fmt.Sprintf("depth %d/%d  level %d  turns %d", s.Depth+1, len(s.D.Levels), s.P.Level, s.Turn),
		fmt.Sprintf("slain: %d files", s.Kills),
		fmt.Sprintf("loot: %d TODO, %d FIXME", s.LootTodo, s.LootFixme),
		"boss: " + s.D.BossName,
		now.Format("2006-01-02"),
	}
	var b strings.Builder
	b.WriteString(border)
	for _, l := range lines {
		// Pad or cut each line to exactly 40 runes inside the border.
		c := cut(l, 40)
		r := []rune(c)
		pad := 40 - len(r)
		fmt.Fprintf(&b, "| %s%s |\n", c, strings.Repeat(" ", pad))
	}
	b.WriteString(border)
	return b.String()
}

// helpPages is the number of pages of the help screen.
const helpPages = 2

// helpRows returns the rows of the current help page. They replace the map
// rows (at most mapRows rows, each at most screenW runes). Page 1 gives the goal
// and the keys and names this run's boss and depth; page 2 explains the map.
// The legend lines start with the symbol as it appears on the map.
func (s *State) helpRows() []string {
	if s.helpPage == 0 {
		return []string{
			"HOW TO PLAY",
			"",
			fmt.Sprintf("Your repo is a dungeon. Slay %s", s.D.BossName),
			fmt.Sprintf("on depth %d (the deepest level) to win. If your HP reaches 0, the run is over.", len(s.D.Levels)),
			"Folders are rooms and deeper folders are deeper levels. Files are monsters.",
			"",
			"KEYS",
			"  h j k l or arrows   move. Walk into a monster to attack it.",
			"  .                   wait one turn",
			"  >                   go down (stand on the > tile first)",
			"  i                   open your pack; press a-j to use an item",
			"  ?                   show this help again",
			"  q                   quit",
		}
	}
	return []string{
		"THE MAP",
		"",
		"  @ you",
		"  & the boss",
		"  a-z a monster: a file. More lines = more HP, more commits = harder hits",
		"  ! potion (from a TODO comment): heals up to 10 HP",
		"  ? scroll (from a FIXME comment): +1 attack",
		"  . floor",
		"  # corridor",
		"  + door",
		"  - | walls",
		"  > stairs",
		"",
		"Monsters sleep until you come close. Slaying them gives XP and levels you up.",
		"There is no saving and no going back up.",
	}
}
