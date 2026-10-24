package game

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/dpologdvinity/repodungeon/internal/dungeon"
)

// mon builds a plain test monster named name at (x, y) with full HP hp and attack atk.
func mon(name string, x, y, hp, atk int) dungeon.Monster {
	return dungeon.Monster{Name: name, Glyph: 'm', Pos: dungeon.Point{X: x, Y: y}, HP: hp, MaxHP: hp, ATK: atk}
}

// keys applies the key strings to s in order.
func keys(s *State, ks ...string) {
	for _, k := range ks {
		s.Apply(k)
	}
}

// TestMoveAndWalls proves that moves and waits take a turn, a wall bump does not,
// unknown keys do nothing, and the field-of-view radius is 5.
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

// TestRender proves the exact 80x24 screen text (SPEC 7.7): message row, map with
// field-of-view clipping, status row and help row. It also proves that New wakes
// monsters already in view.
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

// TestCombatSequence proves damage rolls, message order and turn order in a fight:
// the player strikes first, the monster answers, and a kill gives XP and a kill count.
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

// TestLevelUpLoop proves that one kill can give several levels in a row and that
// leftover XP carries over.
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

// TestBossWin proves that slaying the boss sets Outcome to Won and prints the win message last.
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

// TestMonsterChase proves the monster step rule: move along the longer axis first,
// and a monster out of view stays asleep and does not move.
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

// TestDeathIgnoresLaterKeys proves that a killing blow sets Died, HP 0 and KilledBy,
// that later keys change nothing, and that Render shows the game-over line.
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

// TestItemsAndInventory proves pickup, the inventory screen, using a potion (capped
// heal) and a scroll, that a letter with no item does nothing, and that esc closes the menu.
func TestItemsAndInventory(t *testing.T) {
	d := dungeon.FromArt([]string{"|@....|"})
	d.Levels[0].Items = []dungeon.Item{
		{Kind: dungeon.Potion, Source: "README.md", Pos: dungeon.Point{X: 2, Y: 0}},
		{Kind: dungeon.Scroll, Source: "main.go", Pos: dungeon.Point{X: 3, Y: 0}},
	}
	s := New(d)
	keys(s, "l")
	if len(s.P.Inv) != 1 || s.LootTodo != 1 || !slices.Equal(s.Msgs, []string{"You pick up a TODO potion."}) {
		t.Fatalf("%v", s.Msgs)
	}
	keys(s, "l")
	s.P.HP = 25
	keys(s, "i")
	lines := strings.Split(s.Render(), "\n")
	if s.Mode != Inventory || s.Turn != 2 || lines[1] != "Inventory (a-j to use, esc to close)" ||
		lines[2] != "a) TODO potion (from README.md)" || lines[3] != "b) FIXME scroll (from main.go)" {
		t.Fatalf("inventory screen:\n%s", s.Render())
	}
	keys(s, "a")
	if s.P.HP != 30 || len(s.P.Inv) != 1 || s.Mode != Play || s.Turn != 3 ||
		!slices.Equal(s.Msgs, []string{"You drink a TODO potion. +5 HP."}) {
		t.Fatalf("%+v %v", s.P, s.Msgs)
	}
	keys(s, "i", "b") // no item at index 1: nothing, stay in inventory
	if s.Mode != Inventory || s.Turn != 3 {
		t.Fatal("empty slot must do nothing")
	}
	keys(s, "a")
	if s.P.ATK != 4 || len(s.P.Inv) != 0 || s.Turn != 4 || !slices.Equal(s.Msgs, []string{"You read a FIXME scroll. ATK +1."}) {
		t.Fatalf("%v", s.Msgs)
	}
	keys(s, "i")
	if strings.Split(s.Render(), "\n")[2] != "(empty)" {
		t.Fatal("empty inventory line")
	}
	keys(s, "esc")
	if s.Mode != Play {
		t.Fatal("esc must close inventory")
	}
}

// TestPackFull proves that a full pack leaves the item on the floor and says so.
func TestPackFull(t *testing.T) {
	d := dungeon.FromArt([]string{"|@..|"})
	d.Levels[0].Items = []dungeon.Item{{Kind: dungeon.Potion, Source: "x", Pos: dungeon.Point{X: 2, Y: 0}}}
	s := New(d)
	for i := 0; i < 10; i++ {
		s.P.Inv = append(s.P.Inv, dungeon.Item{Kind: dungeon.Scroll})
	}
	keys(s, "l")
	if len(s.P.Inv) != 10 || len(s.level().Items) != 1 || !slices.Equal(s.Msgs, []string{"Your pack is full."}) {
		t.Fatalf("%v", s.Msgs)
	}
}

// TestStairs proves that > without stairs costs nothing, and that descending moves to
// the next level's start and costs exactly one turn.
func TestStairs(t *testing.T) {
	s := New(dungeon.FromArt([]string{"-----", "|@.>|", "-----"}, []string{"-----", "|.@.|", "-----"}))
	keys(s, ">")
	if s.Turn != 0 || !slices.Equal(s.Msgs, []string{"There are no stairs here."}) {
		t.Fatalf("%v", s.Msgs)
	}
	keys(s, "l", "l", ">")
	if s.Depth != 1 || s.P.Pos != (dungeon.Point{X: 2, Y: 1}) || s.Turn != 3 ||
		!slices.Equal(s.Msgs, []string{"You descend to depth 2."}) || !strings.Contains(s.Render(), "Depth 2/2") {
		t.Fatalf("depth %d pos %v msgs %v", s.Depth, s.P.Pos, s.Msgs)
	}
}

// TestQuit proves that q and ctrl+c end the run as Quit and later keys are ignored.
func TestQuit(t *testing.T) {
	for _, k := range []string{"q", "ctrl+c"} {
		s := New(dungeon.FromArt([]string{"|@..|"}))
		keys(s, k, "l")
		if s.Outcome != Quit || !s.Done() || s.P.Pos.X != 1 || s.Turn != 0 {
			t.Fatalf("%s: %v", k, s.Outcome)
		}
	}
}

// TestCard proves the exact card text for death, quit and win, and that long lines
// are cut so every card row is 44 columns wide.
func TestCard(t *testing.T) {
	d := dungeon.FromArt([]string{"|@...|"})
	d.Levels[0].Monsters = []dungeon.Monster{mon("main.go", 2, 0, 10, 50)}
	s := New(d)
	keys(s, ".")
	now := time.Date(2026, 10, 23, 9, 0, 0, 0, time.UTC)
	want := "+------------------------------------------+\n" +
		"| repodungeon: demo @ abcdef0              |\n" +
		"| slain by main.go on depth 1              |\n" +
		"| depth 1/1  level 1  turns 1              |\n" +
		"| slain: 0 files                           |\n" +
		"| loot: 0 TODO, 0 FIXME                    |\n" +
		"| boss: server.go, Devourer of Weekends    |\n" +
		"| 2026-10-23                               |\n" +
		"+------------------------------------------+\n"
	if got := s.Card(now); got != want {
		t.Fatalf("got\n%s", got)
	}
	q := New(dungeon.FromArt([]string{"|@..|"}))
	keys(q, "q")
	if !strings.Contains(q.Card(now), "| fled the dungeon on depth 1") {
		t.Fatal("quit outcome")
	}
	w := New(dungeon.FromArt([]string{"|@..|"}))
	w.Outcome = Won
	if !strings.Contains(w.Card(now), "| VICTORY! the boss is slain") {
		t.Fatal("win outcome")
	}
	long := New(dungeon.FromArt([]string{"|@..|"}))
	long.D.BossName = strings.Repeat("x", 100)
	for _, line := range strings.Split(strings.TrimSuffix(long.Card(now), "\n"), "\n") {
		if len(line) != 44 {
			t.Fatalf("line width %d: %q", len(line), line)
		}
	}
	unicodeBoss := New(dungeon.FromArt([]string{"|@..|"}))
	unicodeBoss.D.Repo = "日本語リポジトリ"
	unicodeBoss.D.BossName = "龍, Dragon of Code"
	for _, line := range strings.Split(strings.TrimSuffix(unicodeBoss.Card(now), "\n"), "\n") {
		if n := len([]rune(line)); n != 44 {
			t.Fatalf("unicode card line rune width %d != 44: %q", n, line)
		}
	}
}

// TestStatusRowCappedAt80Runes proves that a very long room path does not cause
// the status row to exceed 80 runes in Render().
func TestStatusRowCappedAt80Runes(t *testing.T) {
	d := dungeon.FromArt([]string{"|@..|"})
	d.Levels[0].Rooms = []dungeon.Room{{Name: strings.Repeat("sub/dir/", 20), X: 0, Y: 0, W: 4, H: 1}}
	s := New(d)
	for i, line := range strings.Split(s.Render(), "\n") {
		if n := len([]rune(line)); n > 80 {
			t.Fatalf("row %d is %d runes: %q", i, n, line)
		}
	}
}

// TestInventoryRowsFitScreen proves that a very long item source path is cut so no
// inventory row exceeds 80 runes.
func TestInventoryRowsFitScreen(t *testing.T) {
	s := New(dungeon.FromArt([]string{"|@..|"}))
	s.P.Inv = []dungeon.Item{{Kind: dungeon.Potion, Source: strings.Repeat("d/", 60) + "x.go"}}
	keys(s, "i")
	for i, line := range strings.Split(s.Render(), "\n") {
		if n := len([]rune(line)); n > 80 {
			t.Fatalf("row %d is %d runes", i, n)
		}
	}
}
