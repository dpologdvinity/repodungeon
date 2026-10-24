package game

import (
	"slices"
	"strings"
	"testing"

	"github.com/dpologdvinity/repodungeon/internal/dungeon"
)

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
