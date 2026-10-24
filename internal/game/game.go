// Package game is the pure roguelike state machine, driven by key strings.
package game

import (
	"fmt"

	"github.com/dpologdvinity/repodungeon/internal/dungeon"
)

type Mode int

const (
	Play Mode = iota
	Inventory
)

type Outcome int

const (
	Running Outcome = iota
	Won
	Died
	Quit
)

const (
	packSize  = 10
	fovRadius = 5
)

type Player struct {
	Pos                       dungeon.Point
	HP, MaxHP, ATK, Level, XP int
	Inv                       []dungeon.Item
}

type State struct {
	D                   *dungeon.Dungeon
	Depth               int // index into D.Levels
	P                   Player
	Mode                Mode
	Outcome             Outcome
	Turn                int
	Msgs                []string
	Kills               int
	KilledBy            string
	LootTodo, LootFixme int // items picked up
	seen                [][]bool
	rng                 *dungeon.RNG // combat RNG
}

func New(d *dungeon.Dungeon) *State {
	s := &State{D: d, rng: dungeon.NewRNG(d.Seed + 1)}
	s.P = Player{Pos: d.Levels[0].Start, HP: 30, MaxHP: 30, ATK: 3, Level: 1}
	for _, l := range d.Levels {
		s.seen = append(s.seen, make([]bool, l.W*l.H))
	}
	s.updateFOV()
	s.Msgs = []string{fmt.Sprintf("Welcome to %s! Slay %s on depth %d.", d.Repo, d.BossName, len(d.Levels))}
	return s
}

func (s *State) level() *dungeon.Level { return s.D.Levels[s.Depth] }

func (s *State) Done() bool { return s.Outcome != Running }

// Visible reports whether p is within the field-of-view radius (walls do not block).
func (s *State) Visible(p dungeon.Point) bool {
	dx, dy := p.X-s.P.Pos.X, p.Y-s.P.Pos.Y
	return dx*dx+dy*dy <= fovRadius*fovRadius
}

func (s *State) updateFOV() {
	l := s.level()
	for y := max(0, s.P.Pos.Y-fovRadius); y <= min(l.H-1, s.P.Pos.Y+fovRadius); y++ {
		for x := max(0, s.P.Pos.X-fovRadius); x <= min(l.W-1, s.P.Pos.X+fovRadius); x++ {
			if s.Visible(dungeon.Point{X: x, Y: y}) {
				s.seen[s.Depth][y*l.W+x] = true
			}
		}
	}
	for i := range l.Monsters {
		if m := &l.Monsters[i]; m.HP > 0 && s.Visible(m.Pos) {
			m.Awake = true
		}
	}
}

func (s *State) say(format string, a ...any) { s.Msgs = append(s.Msgs, fmt.Sprintf(format, a...)) }

var moves = map[string]dungeon.Point{
	"h": {X: -1}, "left": {X: -1}, "l": {X: 1}, "right": {X: 1},
	"k": {Y: -1}, "up": {Y: -1}, "j": {Y: 1}, "down": {Y: 1},
}

// Apply handles one key press. Keys after the game is over do nothing.
func (s *State) Apply(key string) {
	if s.Done() {
		return
	}
	s.Msgs = nil
	if key == "q" || key == "ctrl+c" {
		s.Outcome = Quit
		return
	}
	var turn bool
	if s.Mode == Inventory {
		turn = s.inventoryKey(key)
	} else {
		turn = s.playKey(key)
	}
	if !turn {
		return
	}
	if s.Outcome == Running {
		s.monsterPhase()
	}
	s.Turn++
	s.updateFOV()
}

// playKey runs a Play-mode key and reports whether it took a turn.
func (s *State) playKey(key string) bool {
	if d, ok := moves[key]; ok {
		return s.move(d)
	}
	switch key {
	case ".":
		return true
	case "i":
		s.Mode = Inventory
	}
	return false
}

func (s *State) inventoryKey(key string) bool {
	switch key {
	case "esc", "i":
		s.Mode = Play
	}
	return false
}

func (s *State) monsterAt(p dungeon.Point) *dungeon.Monster {
	l := s.level()
	for i := range l.Monsters {
		if m := &l.Monsters[i]; m.HP > 0 && m.Pos == p {
			return m
		}
	}
	return nil
}

func (s *State) move(d dungeon.Point) bool {
	t := dungeon.Point{X: s.P.Pos.X + d.X, Y: s.P.Pos.Y + d.Y}
	if m := s.monsterAt(t); m != nil {
		s.attack(m)
		return true
	}
	if !dungeon.Walkable(s.level().At(t)) {
		return false
	}
	s.P.Pos = t
	return true
}

func (s *State) attack(m *dungeon.Monster) {}

func (s *State) monsterPhase() {}
