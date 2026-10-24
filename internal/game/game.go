// Package game is the pure roguelike state machine, driven by key strings.
package game

import (
	"fmt"
	"slices"

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
	case ">":
		s.descend()
	}
	return false
}

func (s *State) inventoryKey(key string) bool {
	switch {
	case key == "esc" || key == "i":
		s.Mode = Play
	case len(key) == 1 && key[0] >= 'a' && key[0] <= 'j' && int(key[0]-'a') < len(s.P.Inv):
		s.use(int(key[0] - 'a'))
		s.Mode = Play
		return true
	}
	return false
}

func (s *State) use(i int) {
	it := s.P.Inv[i]
	s.P.Inv = slices.Delete(s.P.Inv, i, i+1)
	if it.Kind == dungeon.Potion {
		heal := min(10, s.P.MaxHP-s.P.HP)
		s.P.HP += heal
		s.say("You drink a TODO potion. +%d HP.", heal)
		return
	}
	s.P.ATK++
	s.say("You read a FIXME scroll. ATK +1.")
}

// descend takes the stairs; it costs a turn but monsters do not act.
func (s *State) descend() {
	if s.level().At(s.P.Pos) != dungeon.Stairs || s.Depth+1 >= len(s.D.Levels) {
		s.say("There are no stairs here.")
		return
	}
	s.Depth++
	s.P.Pos = s.level().Start
	s.say("You descend to depth %d.", s.Depth+1)
	s.Turn++
	s.updateFOV()
}

func (s *State) pickUp(t dungeon.Point) {
	l := s.level()
	for i, it := range l.Items {
		if it.Pos != t {
			continue
		}
		if len(s.P.Inv) >= packSize {
			s.say("Your pack is full.")
			return
		}
		l.Items = slices.Delete(l.Items, i, i+1)
		s.P.Inv = append(s.P.Inv, it)
		s.say("You pick up a %s.", it.Kind.Name())
		if it.Kind == dungeon.Potion {
			s.LootTodo++
		} else {
			s.LootFixme++
		}
		return
	}
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
	s.pickUp(t)
	return true
}

// roll is the damage of an attack with strength atk: atk-1..atk+1, at least 1.
func (s *State) roll(atk int) int { return max(1, atk+s.rng.IntN(3)-1) }

func (s *State) attack(m *dungeon.Monster) {
	dmg := s.roll(s.P.ATK)
	m.HP -= dmg
	s.say("You hit %s for %d.", m.Name, dmg)
	if m.HP > 0 {
		return
	}
	s.say("You slay %s!", m.Name)
	s.Kills++
	s.gainXP(m.MaxHP)
	if m.Boss {
		s.say("%s is defeated! You win!", m.Name)
		s.Outcome = Won
	}
}

func (s *State) gainXP(n int) {
	p := &s.P
	p.XP += n
	for p.XP >= 20*p.Level {
		p.XP -= 20 * p.Level
		p.Level++
		p.MaxHP += 5
		p.HP = min(p.MaxHP, p.HP+5)
		p.ATK++
		s.say("You feel stronger! Level %d.", p.Level)
	}
}

func sign(n int) int {
	switch {
	case n > 0:
		return 1
	case n < 0:
		return -1
	}
	return 0
}

func abs(n int) int { return max(n, -n) }

func (s *State) monsterPhase() {
	l := s.level()
	for i := range l.Monsters {
		m := &l.Monsters[i]
		if m.HP <= 0 || !m.Awake {
			continue
		}
		dx, dy := s.P.Pos.X-m.Pos.X, s.P.Pos.Y-m.Pos.Y
		if abs(dx)+abs(dy) == 1 {
			dmg := s.roll(m.ATK)
			s.P.HP -= dmg
			s.say("%s hits you for %d.", m.Name, dmg)
			if s.P.HP <= 0 {
				s.P.HP = 0
				s.Outcome = Died
				s.KilledBy = m.Name
				s.say("You die...")
				return
			}
			continue
		}
		tries := [2]dungeon.Point{{X: sign(dx)}, {Y: sign(dy)}}
		if abs(dx) < abs(dy) {
			tries[0], tries[1] = tries[1], tries[0]
		}
		for _, d := range tries {
			t := dungeon.Point{X: m.Pos.X + d.X, Y: m.Pos.Y + d.Y}
			if d != (dungeon.Point{}) && dungeon.Walkable(l.At(t)) && t != s.P.Pos && s.monsterAt(t) == nil {
				m.Pos = t
				break
			}
		}
	}
}
