package game

import (
	"fmt"
	"strings"
	"time"

	"github.com/dpologdvinity/repodungeon/internal/dungeon"
)

const (
	screenW   = 80
	mapRows   = dungeon.MapH
	helpLine  = "hjkl/arrows move  . wait  > descend  i inventory  q quit"
	overLine  = "Game over. Press any key."
	invHeader = "Inventory (a-j to use, esc to close)"
)

func cut(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}

// Render draws the 80x24 screen as plain text.
func (s *State) Render() string {
	var rows []string
	rows = append(rows, cut(strings.Join(s.Msgs, " "), screenW))
	if s.Mode == Inventory {
		rows = append(rows, invHeader)
		for i, it := range s.P.Inv {
			rows = append(rows, cut(fmt.Sprintf("%c) %s (from %s)", 'a'+i, it.Kind.Name(), it.Source), screenW))
		}
		if len(s.P.Inv) == 0 {
			rows = append(rows, "(empty)")
		}
		for len(rows) < 1+mapRows {
			rows = append(rows, "")
		}
	} else {
		rows = append(rows, s.mapRows()...)
	}
	rows = append(rows, s.status())
	if s.Done() {
		rows = append(rows, overLine)
	} else {
		rows = append(rows, helpLine)
	}
	var b strings.Builder
	for _, r := range rows {
		b.WriteString(strings.TrimRight(r, " "))
		b.WriteByte('\n')
	}
	return b.String()
}

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

func (s *State) place() string {
	for _, r := range s.level().Rooms {
		if r.Contains(s.P.Pos) {
			return r.Name
		}
	}
	return "corridor"
}

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
		fmt.Fprintf(&b, "| %-40s |\n", cut(l, 40))
	}
	b.WriteString(border)
	return b.String()
}
