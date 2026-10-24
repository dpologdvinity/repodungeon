package game

import (
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
