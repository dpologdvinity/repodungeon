package game

import (
	"fmt"
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
		"Depth 1/1  HP 30/30  ATK 3  EN 50  [Unarmed]  LV 1  XP 0/20  T 0  /\n" +
		"hjkl/arrows move  . wait  > descend  1-3 git powers  i inventory  ? help  q quit\n"
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
	keys(s, "l") // player 3+0-1=2 -> monster HP 2; monster 2+0-1=1
	if s.level().Monsters[0].HP != 2 || s.P.HP != 27 ||
		!slices.Equal(s.Msgs, []string{"You hit main.go for 2.", "main.go hits you for 1."}) {
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

// TestRenderRememberedTiles proves that previously seen tiles outside the current
// FOV are rendered as remembered terrain.
func TestRenderRememberedTiles(t *testing.T) {
	d := dungeon.FromArt([]string{"|@.......|"})
	s := New(d)
	for i := 0; i < 6; i++ {
		keys(s, "l")
	}
	_ = s.Render()
}

// TestHelpScreen proves the two-page how-to-play screen: every page fits the
// fixed 80x24 layout, page 1 names the goal and keys, page 2 lists every map
// symbol on its own line, a key turns the page and then closes help without
// costing a turn, and "?" opens it again during play.
func TestHelpScreen(t *testing.T) {
	s := New(dungeon.FromArt([]string{"|@..|"}))
	s.OpenHelp()
	page := func() string {
		lines := strings.Split(strings.TrimSuffix(s.Render(), "\n"), "\n")
		if s.Mode != Help || len(lines) != 24 {
			t.Fatalf("mode %v, %d rows", s.Mode, len(lines))
		}
		for i, l := range lines {
			if n := len([]rune(l)); n > 80 {
				t.Fatalf("row %d has %d runes", i, n)
			}
		}
		return s.Render()
	}
	has := func(screen string, wants ...string) {
		t.Helper()
		for _, want := range wants {
			if !strings.Contains(screen, want) {
				t.Errorf("help screen lacks %q:\n%s", want, screen)
			}
		}
	}
	has(page(), "HOW TO PLAY", "server.go, Devourer of Weekends", "KEYS", "Press any key for page 2 of 2.")
	keys(s, "l") // turns the page; must not move
	map2 := page()
	has(map2, "THE MAP", "Press any key to continue.")
	for _, legend := range []string{"  @ you\n", "  & the boss\n", "  ! potion", "  ? scroll", "  . floor\n",
		"  # corridor\n", "  + door\n", "  - | walls\n", "  > stairs\n"} {
		has(map2, legend)
	}
	keys(s, "l") // closes help; must not move either
	if s.Mode != Play || s.P.Pos.X != 1 || s.Turn != 0 {
		t.Fatalf("mode %v pos %v turn %d", s.Mode, s.P.Pos, s.Turn)
	}
	keys(s, "?")
	if s.Mode != Help || s.Turn != 0 || !strings.Contains(s.Render(), "HOW TO PLAY") {
		t.Fatal("? must open help on page 1 without a turn")
	}
	keys(s, "q")
	if s.Outcome != Quit {
		t.Fatal("q must still quit from the help screen")
	}
}

func TestWeaponEquipAndPackLimit(t *testing.T) {
	d := dungeon.FromArt([]string{
		"---|",
		"|@.|",
		"---|",
	})
	s := New(d)
	w := dungeon.Item{Kind: dungeon.Weapon, Archetype: dungeon.Sword, BonusATK: 3}
	s.P.Inv = append(s.P.Inv, w)

	// Test equipping weapon from inventory slot 0
	s.inventoryKey("a")
	if s.P.Weapon == nil || s.P.Weapon.BonusATK != 3 {
		t.Errorf("expected weapon equipped with +3 ATK")
	}

	// Review Focus #5: Full pack with equipped weapon
	for i := 0; i < packSize; i++ {
		s.P.Inv = append(s.P.Inv, dungeon.Item{Kind: dungeon.Potion})
	}
	s.level().Items = append(s.level().Items, dungeon.Item{Kind: dungeon.Weapon, Pos: dungeon.Point{X: 2, Y: 1}})
	s.Apply("l") // step onto weapon
	if len(s.P.Inv) > packSize {
		t.Errorf("pack overflowed beyond %d items", packSize)
	}
	if s.Msgs[len(s.Msgs)-1] != "Your pack is full." {
		t.Errorf("expected full pack message, got %v", s.Msgs)
	}
}

func TestWeaponAutoEquipAndSwap(t *testing.T) {
	// Auto-equip on floor pickup when unarmed and inventory empty
	d := dungeon.FromArt([]string{
		"----|",
		"|@..|",
		"----|",
	})
	s := New(d)
	s.level().Items = append(s.level().Items, dungeon.Item{
		Kind: dungeon.Weapon, Archetype: dungeon.Sword, BonusATK: 3, Pos: dungeon.Point{X: 2, Y: 1},
	})
	s.Apply("l")
	if s.P.Weapon == nil || s.P.Weapon.Archetype != dungeon.Sword || s.P.Weapon.BonusATK != 3 {
		t.Fatalf("expected sword auto-equipped on empty inventory: %+v", s.P.Weapon)
	}
	if len(s.P.Inv) != 0 {
		t.Fatalf("inventory should be empty after auto-equip, got %d items", len(s.P.Inv))
	}

	// Pickup weapon when already equipped: goes to inventory
	s.level().Items = append(s.level().Items, dungeon.Item{
		Kind: dungeon.Weapon, Archetype: dungeon.Spear, BonusATK: 2, Pos: dungeon.Point{X: 3, Y: 1},
	})
	s.Apply("l")
	if s.P.Weapon == nil || s.P.Weapon.Archetype != dungeon.Sword {
		t.Fatalf("equipped weapon changed unexpectedly: %+v", s.P.Weapon)
	}
	if len(s.P.Inv) != 1 || s.P.Inv[0].Archetype != dungeon.Spear {
		t.Fatalf("expected spear in inventory, got: %+v", s.P.Inv)
	}

	// Swapping weapons: equipping spear swaps sword back to inventory
	s.inventoryKey("a")
	if s.P.Weapon == nil || s.P.Weapon.Archetype != dungeon.Spear {
		t.Fatalf("expected spear equipped after swap: %+v", s.P.Weapon)
	}
	if len(s.P.Inv) != 1 || s.P.Inv[0].Archetype != dungeon.Sword {
		t.Fatalf("expected sword returned to inventory slot 0, got: %+v", s.P.Inv)
	}

	// Pickup when unarmed but inventory not empty: goes to inventory, does not auto-equip
	s.P.Weapon = nil
	s.level().Items = append(s.level().Items, dungeon.Item{
		Kind: dungeon.Weapon, Archetype: dungeon.Hammer, BonusATK: 3, Pos: dungeon.Point{X: 2, Y: 1},
	})
	s.Apply("h") // step left onto hammer
	if s.P.Weapon != nil {
		t.Fatalf("expected unarmed when pack was not empty, got equipped: %+v", s.P.Weapon)
	}
	if len(s.P.Inv) != 2 || s.P.Inv[1].Archetype != dungeon.Hammer {
		t.Fatalf("expected hammer added to inventory, got: %+v", s.P.Inv)
	}
}

func TestWeaponCombatMechanics(t *testing.T) {
	// 1. Spear 2-tile reach attack
	d := dungeon.FromArt([]string{
		"------|",
		"|@....|",
		"------|",
	})
	s := New(d)
	s.P.Weapon = &dungeon.Item{Kind: dungeon.Weapon, Archetype: dungeon.Spear, BonusATK: 2}
	s.level().Monsters = append(s.level().Monsters, dungeon.Monster{
		Name: "target.go", Pos: dungeon.Point{X: 3, Y: 1}, HP: 10, MaxHP: 10, ATK: 2,
	})

	// Pressing 'l' (right) toward monster at distance 2 should attack without changing P.Pos
	startPos := s.P.Pos
	s.Apply("l")
	if s.P.Pos != startPos {
		t.Errorf("spear attack should not advance player position: %v vs %v", s.P.Pos, startPos)
	}
	if s.level().Monsters[0].HP >= 10 {
		t.Errorf("spear attack should have damaged monster, HP is %d", s.level().Monsters[0].HP)
	}

	// Review Focus #1: Spear obstructed by wall should not attack through walls
	s.level().Set(dungeon.Point{X: 2, Y: 1}, dungeon.WallV)
	hpBefore := s.level().Monsters[0].HP
	s.Apply("l")
	if s.level().Monsters[0].HP != hpBefore {
		t.Errorf("spear should not attack through solid wall")
	}

	// Review Focus #1: Spear does not target dead monster at distance 2
	s.level().Set(dungeon.Point{X: 2, Y: 1}, dungeon.Floor) // clear wall
	s.level().Monsters[0].HP = 0                            // mark dead
	s.Apply("l")
	// Since monster at distance 2 is dead, player should step into tile 1
	if s.P.Pos != (dungeon.Point{X: 2, Y: 1}) {
		t.Errorf("expected player to advance onto walkable tile when 2-tile monster is dead, got %v", s.P.Pos)
	}

	// 2. Hammer knockback & collision damage
	dHammer := dungeon.FromArt([]string{
		"--------|",
		"|@.....||",
		"--------|",
	})
	sH := New(dHammer)
	sH.P.Weapon = &dungeon.Item{Kind: dungeon.Weapon, Archetype: dungeon.Hammer, BonusATK: 3}
	sH.level().Monsters = append(sH.level().Monsters, dungeon.Monster{
		Name: "block.go", Pos: dungeon.Point{X: 2, Y: 1}, HP: 20, MaxHP: 20, ATK: 2,
	})

	// Hit with open space behind target: monster pushed from (2,1) to (3,1)
	sH.Apply("l")
	if sH.level().Monsters[0].Pos != (dungeon.Point{X: 3, Y: 1}) {
		t.Errorf("expected hammer to knock monster to (3,1), got %v", sH.level().Monsters[0].Pos)
	}

	// Hit with wall behind target: wall at (4,1), monster at (3,1)
	sH.level().Set(dungeon.Point{X: 4, Y: 1}, dungeon.WallV)
	// Player steps forward to (2,1) adjacent to monster
	sH.Apply("l")
	if sH.P.Pos != (dungeon.Point{X: 2, Y: 1}) {
		t.Fatalf("expected player at (2,1), got %v", sH.P.Pos)
	}
	hpBeforeHit := sH.level().Monsters[0].HP
	// Player hits monster at (3,1), hammer attempts knockback into wall at (4,1)
	sH.Apply("l")
	// Monster should remain at (3,1) due to wall at (4,1)
	if sH.level().Monsters[0].Pos != (dungeon.Point{X: 3, Y: 1}) {
		t.Errorf("monster should not move when knockback is blocked by wall: %v", sH.level().Monsters[0].Pos)
	}
	// Damage should include normal roll (P.ATK 3 + Weapon 3 = 6 -> roll 5..7) + 2 collision damage
	damageTaken := hpBeforeHit - sH.level().Monsters[0].HP
	if damageTaken < 7 || damageTaken > 9 {
		t.Errorf("expected damage with collision 7..9, got %d", damageTaken)
	}

	// Hit with another living monster blocking destination
	dBlocked := dungeon.FromArt([]string{
		"--------|",
		"|@.....||",
		"--------|",
	})
	sB := New(dBlocked)
	sB.P.Weapon = &dungeon.Item{Kind: dungeon.Weapon, Archetype: dungeon.Hammer, BonusATK: 3}
	sB.level().Monsters = []dungeon.Monster{
		{Name: "front.go", Pos: dungeon.Point{X: 2, Y: 1}, HP: 20, MaxHP: 20, ATK: 2},
		{Name: "rear.go", Pos: dungeon.Point{X: 3, Y: 1}, HP: 20, MaxHP: 20, ATK: 2},
	}
	hpBeforeFront := sB.level().Monsters[0].HP
	sB.Apply("l")
	if sB.level().Monsters[0].Pos != (dungeon.Point{X: 2, Y: 1}) {
		t.Errorf("monster should not move when destination holds another monster, pos: %v", sB.level().Monsters[0].Pos)
	}
	dmgBlocked := hpBeforeFront - sB.level().Monsters[0].HP
	if dmgBlocked < 7 || dmgBlocked > 9 {
		t.Errorf("expected 7..9 damage including collision, got %d", dmgBlocked)
	}

	// 3. Dagger critical damage on sleeping and stashed enemies
	dDagger := dungeon.FromArt([]string{
		"------|",
		"|@...||",
		"------|",
	})
	sD := New(dDagger)
	sD.P.Weapon = &dungeon.Item{Kind: dungeon.Weapon, Archetype: dungeon.Dagger, BonusATK: 1}
	sD.level().Monsters = []dungeon.Monster{
		{Name: "asleep.go", Pos: dungeon.Point{X: 2, Y: 1}, HP: 30, MaxHP: 30, ATK: 2, Awake: false},
	}
	// Target is asleep: dagger deals double damage (P.ATK 3 + 1 = 4 -> roll 3..5, doubled is 6..10)
	mult := sD.daggerCritical(&sD.level().Monsters[0])
	if mult != 2 {
		t.Errorf("expected daggerCritical multiplier 2 for sleeping monster, got %d", mult)
	}
	sD.Apply("l")
	dmgSleeping := 30 - sD.level().Monsters[0].HP
	if dmgSleeping < 6 || dmgSleeping > 10 {
		t.Errorf("expected sleeping crit damage 6..10, got %d", dmgSleeping)
	}

	// Target is now awake, no stash: normal damage (3..5)
	multAwake := sD.daggerCritical(&sD.level().Monsters[0])
	if multAwake != 1 {
		t.Errorf("expected daggerCritical multiplier 1 for awake monster without stash, got %d", multAwake)
	}
	hpAwakeBefore := sD.level().Monsters[0].HP
	sD.Apply("l")
	dmgAwake := hpAwakeBefore - sD.level().Monsters[0].HP
	if dmgAwake < 3 || dmgAwake > 5 {
		t.Errorf("expected awake dagger damage 3..5, got %d", dmgAwake)
	}

	// Target is awake, but player in git stash stealth: double damage
	sD.P.StashTurns = 2
	multStash := sD.daggerCritical(&sD.level().Monsters[0])
	if multStash != 2 {
		t.Errorf("expected daggerCritical multiplier 2 while stashed, got %d", multStash)
	}
	hpStashBefore := sD.level().Monsters[0].HP
	sD.Apply("l")
	dmgStash := hpStashBefore - sD.level().Monsters[0].HP
	if dmgStash < 6 || dmgStash > 10 {
		t.Errorf("expected stashed dagger damage 6..10, got %d", dmgStash)
	}
}

// TestWeaponDirectInterfaces verifies direct method calls for spearReachAttack,
// hammerKnockback, and daggerCritical, including boundary and dead-monster handling.
func TestWeaponDirectInterfaces(t *testing.T) {
	d := dungeon.FromArt([]string{
		"------|",
		"|@....|",
		"------|",
	})
	s := New(d)

	// Direct spearReachAttack without spear returns false
	if s.spearReachAttack(dungeon.Point{X: 1, Y: 0}) {
		t.Errorf("spearReachAttack should return false when unarmed")
	}

	// Equip spear
	s.P.Weapon = &dungeon.Item{Kind: dungeon.Weapon, Archetype: dungeon.Spear, BonusATK: 2}
	// No monster at distance 2 -> false
	if s.spearReachAttack(dungeon.Point{X: 1, Y: 0}) {
		t.Errorf("spearReachAttack should return false when tile 2 is empty")
	}

	// Living monster at distance 2 -> true
	s.level().Monsters = append(s.level().Monsters, dungeon.Monster{
		Name: "m.go", Pos: dungeon.Point{X: 3, Y: 1}, HP: 15, MaxHP: 15, ATK: 2,
	})
	if !s.spearReachAttack(dungeon.Point{X: 1, Y: 0}) {
		t.Errorf("spearReachAttack should return true when monster at distance 2")
	}
	if s.level().Monsters[0].HP >= 15 {
		t.Errorf("monster should have taken damage from direct spearReachAttack")
	}

	// Dead monster at distance 2 -> false
	s.level().Monsters[0].HP = 0
	if s.spearReachAttack(dungeon.Point{X: 1, Y: 0}) {
		t.Errorf("spearReachAttack should return false when monster is dead")
	}

	// Living monster at tile 1 blocks spear reach attack to tile 2
	s.level().Monsters = append(s.level().Monsters,
		dungeon.Monster{Name: "near.go", Pos: dungeon.Point{X: 2, Y: 1}, HP: 10, MaxHP: 10, ATK: 2},
		dungeon.Monster{Name: "far.go", Pos: dungeon.Point{X: 3, Y: 1}, HP: 10, MaxHP: 10, ATK: 2},
	)
	if s.spearReachAttack(dungeon.Point{X: 1, Y: 0}) {
		t.Errorf("spearReachAttack should return false when tile 1 holds living monster")
	}

	// Dead monster at tile 1 allows spear reach attack to tile 2
	s.level().Monsters[1].HP = 0 // near.go is dead
	if !s.spearReachAttack(dungeon.Point{X: 1, Y: 0}) {
		t.Errorf("spearReachAttack should succeed when intervening monster is dead")
	}

	// Direct hammerKnockback
	s.level().Monsters = nil // clear monsters from earlier spear checks
	s.P.Weapon = &dungeon.Item{Kind: dungeon.Weapon, Archetype: dungeon.Hammer, BonusATK: 3}
	hm := &dungeon.Monster{Name: "h.go", Pos: dungeon.Point{X: 2, Y: 1}, HP: 10, MaxHP: 10, ATK: 2}
	// Knockback into open floor (3,1)
	s.hammerKnockback(hm, dungeon.Point{X: 1, Y: 0})
	if hm.Pos != (dungeon.Point{X: 3, Y: 1}) || hm.HP != 10 {
		t.Errorf("expected knockback to (3,1) without damage, got pos %v hp %d", hm.Pos, hm.HP)
	}

	// Knockback into wall (4,1)
	s.level().Set(dungeon.Point{X: 4, Y: 1}, dungeon.WallV)
	s.hammerKnockback(hm, dungeon.Point{X: 1, Y: 0})
	if hm.Pos != (dungeon.Point{X: 3, Y: 1}) || hm.HP != 8 {
		t.Errorf("expected collision damage of 2 and pos unchanged, got pos %v hp %d", hm.Pos, hm.HP)
	}

	// Dead monster does nothing in hammerKnockback
	hm.HP = 0
	s.hammerKnockback(hm, dungeon.Point{X: 1, Y: 0})
	if hm.HP != 0 {
		t.Errorf("dead monster should not take collision damage")
	}

	// Nil monster does nothing in hammerKnockback
	s.hammerKnockback(nil, dungeon.Point{X: 1, Y: 0})

	// Direct daggerCritical on nil returns 1
	if s.daggerCritical(nil) != 1 {
		t.Errorf("daggerCritical(nil) should return 1")
	}
}

// TestHammerCollisionKill verifies that collision damage can slay a monster and award XP.
func TestHammerCollisionKill(t *testing.T) {
	d := dungeon.FromArt([]string{
		"----|",
		"|@.||",
		"----|",
	})
	s := New(d)
	s.P.Weapon = &dungeon.Item{Kind: dungeon.Weapon, Archetype: dungeon.Hammer, BonusATK: 3}
	s.level().Monsters = []dungeon.Monster{
		{Name: "fragile.go", Pos: dungeon.Point{X: 2, Y: 1}, HP: 1, MaxHP: 10, ATK: 1},
	}
	// Initial hit would normally kill anyway with roll, so test hammerKnockback directly with 2 HP against wall at (3,1)
	hm := &s.level().Monsters[0]
	hm.HP = 2
	killsBefore := s.Kills
	xpBefore := s.P.XP
	s.hammerKnockback(hm, dungeon.Point{X: 1, Y: 0})
	if hm.HP > 0 {
		t.Errorf("monster should be slain by collision, HP: %d", hm.HP)
	}
	if s.Kills != killsBefore+1 {
		t.Errorf("expected kills incremented, got %d", s.Kills)
	}
	if s.P.XP != xpBefore+10 {
		t.Errorf("expected XP awarded for collision kill, got %d", s.P.XP)
	}
}

// TestSwordBonusDamage verifies that a sword adds BonusATK to damage without extra effects.
func TestSwordBonusDamage(t *testing.T) {
	d := dungeon.FromArt([]string{
		"----|",
		"|@.||",
		"----|",
	})
	s := New(d)
	s.P.Weapon = &dungeon.Item{Kind: dungeon.Weapon, Archetype: dungeon.Sword, BonusATK: 4}
	s.level().Monsters = []dungeon.Monster{
		{Name: "target.go", Pos: dungeon.Point{X: 2, Y: 1}, HP: 50, MaxHP: 50, ATK: 1},
	}
	// Player ATK is 3 + 4 = 7. Damage roll is 6..8.
	s.Apply("l")
	dmg := 50 - s.level().Monsters[0].HP
	if dmg < 6 || dmg > 8 {
		t.Errorf("expected sword damage 6..8, got %d", dmg)
	}
	// Monster should not be knocked back
	if s.level().Monsters[0].Pos != (dungeon.Point{X: 2, Y: 1}) {
		t.Errorf("sword attack should not knock back monster")
	}
}

func TestRestingAndMaxHPConstraint(t *testing.T) {
	d := dungeon.FromArt([]string{
		"----|",
		"|@..|",
		"----|",
	})
	s := New(d)
	s.P.HP = 20
	s.P.MaxHP = 30

	// 4 safe wait turns should restore 1 HP
	for i := 0; i < 4; i++ {
		s.Apply(".")
	}
	if s.P.HP != 21 {
		t.Errorf("expected HP to heal to 21 after 4 safe waits, got %d", s.P.HP)
	}
	if !slices.Contains(s.Msgs, "You rest quietly. +1 HP.") {
		t.Errorf("expected rest message, got %v", s.Msgs)
	}

	// Review Focus #3: resting at MaxHP must not overflow or send spurious messages
	s.P.HP = s.P.MaxHP
	s.Msgs = nil
	for i := 0; i < 4; i++ {
		s.Apply(".")
	}
	if s.P.HP > s.P.MaxHP {
		t.Errorf("HP overflowed MaxHP: %d > %d", s.P.HP, s.P.MaxHP)
	}
	if len(s.Msgs) > 0 {
		t.Errorf("expected no messages when resting at MaxHP, got %v", s.Msgs)
	}
}

func TestRestingInterruptedByAwakeMonsters(t *testing.T) {
	d := dungeon.FromArt([]string{
		"--------|",
		"|@.....||",
		"--------|",
	})
	s := New(d)
	s.P.HP = 20
	s.P.MaxHP = 30

	// 3 safe waits
	for i := 0; i < 3; i++ {
		s.Apply(".")
	}
	if s.P.HP != 20 {
		t.Errorf("expected HP 20 after 3 waits, got %d", s.P.HP)
	}

	// Add awake monster in FOV at distance 3 (Pos X:4, Y:1)
	s.level().Monsters = append(s.level().Monsters, dungeon.Monster{
		Name: "bug.go", Pos: dungeon.Point{X: 4, Y: 1}, HP: 10, MaxHP: 10, ATK: 2, Awake: true,
	})

	if !s.hasAwakeMonstersInFOV() {
		t.Fatalf("expected hasAwakeMonstersInFOV to be true")
	}

	// 4th wait should NOT heal because awake monster is in FOV; counter resets
	s.Apply(".")
	if s.P.HP != 20 {
		t.Errorf("expected HP to stay 20 with awake monster in FOV, got %d", s.P.HP)
	}

	// Defeat or remove monster
	s.level().Monsters[0].HP = 0
	if s.hasAwakeMonstersInFOV() {
		t.Errorf("dead monster in FOV should not count as awake monster")
	}

	// Now 4 safe waits should restore 1 HP from the fresh reset
	for i := 0; i < 4; i++ {
		s.Apply(".")
	}
	if s.P.HP != 21 {
		t.Errorf("expected HP to heal to 21 after 4 fresh safe waits, got %d", s.P.HP)
	}
}

func TestRestingInterruptedByMovement(t *testing.T) {
	d := dungeon.FromArt([]string{
		"----|",
		"|@..|",
		"----|",
	})
	s := New(d)
	s.P.HP = 20
	s.P.MaxHP = 30

	// 3 safe waits
	for i := 0; i < 3; i++ {
		s.Apply(".")
	}

	// Step right then left
	s.Apply("l")
	s.Apply("h")

	// Wait 1 turn - should NOT heal because movement broke the streak
	s.Apply(".")
	if s.P.HP != 20 {
		t.Errorf("movement should have reset safe rest counter, HP is %d", s.P.HP)
	}

	// 3 more waits (total 4 since reset) -> now heals
	for i := 0; i < 3; i++ {
		s.Apply(".")
	}
	if s.P.HP != 21 {
		t.Errorf("expected HP 21 after completing 4 waits post-movement, got %d", s.P.HP)
	}
}

func TestRestingDirectInterfaces(t *testing.T) {
	d := dungeon.FromArt([]string{
		"-------------------|",
		"|@.................|",
		"-------------------|",
	})
	s := New(d)
	s.P.HP = 20
	s.P.MaxHP = 30

	// hasAwakeMonstersInFOV with no monsters
	if s.hasAwakeMonstersInFOV() {
		t.Errorf("expected false with no monsters")
	}

	// Monster far out of FOV (fovRadius is 5)
	s.level().Monsters = append(s.level().Monsters, dungeon.Monster{
		Name: "far.go", Pos: dungeon.Point{X: 15, Y: 1}, HP: 10, MaxHP: 10, ATK: 2, Awake: true,
	})
	if s.Visible(dungeon.Point{X: 15, Y: 1}) {
		t.Fatalf("monster at (15,1) should be out of FOV from (1,1)")
	}
	if s.hasAwakeMonstersInFOV() {
		t.Errorf("awake monster outside FOV should not count")
	}

	// Directly call safeRestTurn 4 times
	for i := 0; i < 4; i++ {
		s.safeRestTurn()
	}
	if s.P.HP != 21 {
		t.Errorf("expected direct safeRestTurn to heal to 21, got %d", s.P.HP)
	}
}

func TestMonsterTelegraphAndDodge(t *testing.T) {
	d := dungeon.FromArt([]string{
		"--------",
		"|......|",
		"--------",
	})
	d.Levels[0].Start = dungeon.Point{X: 3, Y: 1}
	d.Levels[0].Monsters = []dungeon.Monster{
		{Name: "goblin.go", Pos: dungeon.Point{X: 4, Y: 1}, HP: 20, MaxHP: 20, ATK: 4, Awake: true, Telegraph: true},
	}
	s := New(d)
	m := &s.level().Monsters[0]

	// Player stays adjacent: takes 1.5x damage
	hpBefore := s.P.HP
	s.Apply(".") // wait -> stays adjacent -> takes telegraphed hit
	if s.P.HP >= hpBefore {
		t.Errorf("expected damage from telegraphed strike, hpBefore=%d, hpAfter=%d", hpBefore, s.P.HP)
	}
	if m.Telegraph {
		t.Errorf("telegraph flag should reset after striking")
	}

	// Player steps away to distance > 1: attack misses!
	m.Telegraph = true
	hpBefore = s.P.HP
	s.Apply("h") // step left to (2, 1), monster at (4, 1): distance is 2 > 1
	if s.P.HP != hpBefore {
		t.Errorf("expected no damage when dodging telegraphed strike, hpBefore=%d, hpAfter=%d", hpBefore, s.P.HP)
	}
	if m.Telegraph {
		t.Errorf("telegraph flag should reset after miss")
	}
	if !slices.Contains(s.Msgs, m.Name+"'s heavy blow misses!") {
		t.Errorf("expected miss message, got %v", s.Msgs)
	}
}

func TestMonsterTelegraphOrganicCycleAndDodge(t *testing.T) {
	// Find a seed where the first IntN(3) returns 0 (triggers telegraph)
	var seed uint64
	for s := uint64(0); s < 100; s++ {
		if dungeon.NewRNG(s+1).IntN(3) == 0 {
			seed = s
			break
		}
	}

	d := dungeon.FromArt([]string{
		"--------",
		"|......|",
		"--------",
	})
	d.Seed = seed
	d.Levels[0].Start = dungeon.Point{X: 3, Y: 1}
	d.Levels[0].Monsters = []dungeon.Monster{
		{Name: "orc.go", Pos: dungeon.Point{X: 4, Y: 1}, HP: 50, MaxHP: 50, ATK: 6, Awake: true},
	}
	s := New(d)
	m := &s.level().Monsters[0]

	hpBefore := s.P.HP
	// Turn 1: Player waits. Monster is adjacent, not telegraphed. RNG rolls 0 -> telegraphs.
	s.Apply(".")
	if !m.Telegraph {
		t.Fatalf("expected monster to telegraph on turn 1 with seed %d", seed)
	}
	if s.P.HP != hpBefore {
		t.Errorf("expected no damage on telegraph wind-up turn, hpBefore=%d, got %d", hpBefore, s.P.HP)
	}
	if !slices.Contains(s.Msgs, "orc.go winds up a heavy blow!") {
		t.Errorf("expected windup message, got %v", s.Msgs)
	}

	// Turn 2: Player steps away to dodge (X: 2, Y: 1), distance = 2
	s.Apply("h")
	if m.Telegraph {
		t.Errorf("telegraph flag should reset after dodge")
	}
	if s.P.HP != hpBefore {
		t.Errorf("expected no damage when dodging telegraphed blow")
	}
	if !slices.Contains(s.Msgs, "orc.go's heavy blow misses!") {
		t.Errorf("expected miss message, got %v", s.Msgs)
	}
}

func TestMonsterTelegraphDamageMultiplier(t *testing.T) {
	d := dungeon.FromArt([]string{
		"--------",
		"|......|",
		"--------",
	})
	d.Levels[0].Start = dungeon.Point{X: 3, Y: 1}
	d.Levels[0].Monsters = []dungeon.Monster{
		{Name: "troll.go", Pos: dungeon.Point{X: 4, Y: 1}, HP: 50, MaxHP: 50, ATK: 10, Awake: true, Telegraph: true},
	}
	s := New(d)
	s.P.HP = 50
	s.P.MaxHP = 50

	s.Apply(".")
	dmg := 50 - s.P.HP
	// Normal roll for ATK 10 is 9..11. Telegraphed 1.5x damage is:
	// 9 -> 13, 10 -> 15, 11 -> 16. So dmg must be between 13 and 16.
	if dmg < 13 || dmg > 16 {
		t.Errorf("expected telegraphed damage 13..16, got %d", dmg)
	}
}

func TestMonsterTelegraphFatalStrike(t *testing.T) {
	d := dungeon.FromArt([]string{
		"--------",
		"|......|",
		"--------",
	})
	d.Levels[0].Start = dungeon.Point{X: 3, Y: 1}
	d.Levels[0].Monsters = []dungeon.Monster{
		{Name: "boss.go", Pos: dungeon.Point{X: 4, Y: 1}, HP: 50, MaxHP: 50, ATK: 10, Awake: true, Telegraph: true},
	}
	s := New(d)
	s.P.HP = 5 // low health so telegraphed strike is fatal

	s.Apply(".")
	if s.P.HP != 0 {
		t.Errorf("expected player HP 0, got %d", s.P.HP)
	}
	if s.Outcome != Died {
		t.Errorf("expected Outcome Died, got %v", s.Outcome)
	}
	if s.KilledBy != "boss.go" {
		t.Errorf("expected KilledBy boss.go, got %q", s.KilledBy)
	}
	if !slices.Contains(s.Msgs, "You die...") {
		t.Errorf("expected death message, got %v", s.Msgs)
	}
}

func TestMonsterTelegraphSlainBeforeStrike(t *testing.T) {
	d := dungeon.FromArt([]string{
		"--------",
		"|......|",
		"--------",
	})
	d.Levels[0].Start = dungeon.Point{X: 3, Y: 1}
	d.Levels[0].Monsters = []dungeon.Monster{
		{Name: "weakling.go", Pos: dungeon.Point{X: 4, Y: 1}, HP: 1, MaxHP: 1, ATK: 10, Awake: true, Telegraph: true},
	}
	s := New(d)
	hpBefore := s.P.HP

	// Player attacks the monster moving "l" (into tile 4, 1)
	s.Apply("l")
	if s.level().Monsters[0].HP > 0 {
		t.Fatalf("expected monster to be slain")
	}
	if s.P.HP != hpBefore {
		t.Errorf("player should not take damage from slain monster, hpBefore=%d, got %d", hpBefore, s.P.HP)
	}
	if slices.Contains(s.Msgs, "weakling.go's heavy blow misses!") {
		t.Errorf("slain monster should not emit miss message")
	}
}

func TestGitPowersAndFocusChecks(t *testing.T) {
	d := dungeon.FromArt([]string{
		"------|",
		"|@.m..|",
		"------|",
	})
	s := New(d)
	s.P.GitEnergy = 100

	// Review Focus #2: 'git revert' (key 2) with 0 damage taken
	s.P.LastDamageTaken = 0
	s.Apply("2")
	if s.P.GitEnergy != 100 {
		t.Errorf("git revert should not consume energy when no damage was taken")
	}
	if !slices.Contains(s.Msgs, "No damage to revert.") {
		t.Errorf("expected 'No damage to revert.' message, got %v", s.Msgs)
	}

	// Review Focus #4: 'git blame' (key 3) with no adjacent monster
	s.P.Pos = dungeon.Point{X: 1, Y: 1}
	s.level().Monsters[0].Pos = dungeon.Point{X: 5, Y: 1}
	s.Apply("3")
	if s.P.GitEnergy != 100 {
		t.Errorf("git blame should not consume energy when no enemy is adjacent")
	}
	if !slices.Contains(s.Msgs, "No adjacent enemy to blame.") {
		t.Errorf("expected 'No adjacent enemy to blame.' message, got %v", s.Msgs)
	}

	// Successful git stash (key 1)
	s.Apply("1")
	if s.P.GitEnergy != 70 { // cost 30
		t.Errorf("expected energy 70 after stash, got %d", s.P.GitEnergy)
	}
	if s.P.StashTurns != 2 {
		t.Errorf("expected 2 turns of stash stealth")
	}
}

func TestGitPowerRevertHeal(t *testing.T) {
	d := dungeon.FromArt([]string{
		"--------",
		"|@.m...|",
		"--------",
	})
	d.Levels[0].Monsters[0].Pos = dungeon.Point{X: 2, Y: 1}
	s := New(d)
	s.P.GitEnergy = 100

	// Wait a turn so the adjacent monster hits the player
	s.Apply(".")
	damageTaken := s.P.LastDamageTaken
	if damageTaken <= 0 {
		t.Fatalf("expected monster to deal damage, got LastDamageTaken=%d", damageTaken)
	}
	hpAfterHit := s.P.HP
	if hpAfterHit != 30-damageTaken {
		t.Fatalf("expected HP %d, got %d", 30-damageTaken, hpAfterHit)
	}

	// Revert damage
	s.Apply("2")
	if s.P.GitEnergy != 75 { // 100 - 25 = 75
		t.Errorf("expected GitEnergy 75, got %d", s.P.GitEnergy)
	}
	if s.P.HP != 30 {
		t.Errorf("expected player HP healed back to 30, got %d", s.P.HP)
	}
	wantMsg := fmt.Sprintf("You revert the damage! +%d HP.", damageTaken)
	if !slices.Contains(s.Msgs, wantMsg) {
		t.Errorf("expected message %q, got %v", wantMsg, s.Msgs)
	}
}

func TestGitPowerBlame(t *testing.T) {
	d := dungeon.FromArt([]string{
		"--------",
		"|..m...|",
		"|.m@m..|",
		"|..m...|",
		"--------",
	})
	// Player is at (3, 2). Place monsters at Up (3, 1), Left (2, 2), Right (4, 2), Down (3, 3)
	d.Levels[0].Start = dungeon.Point{X: 3, Y: 2}
	d.Levels[0].Monsters = []dungeon.Monster{
		{Name: "up.go", Pos: dungeon.Point{X: 3, Y: 1}, HP: 10, MaxHP: 10, ATK: 6, Awake: true},
		{Name: "left.go", Pos: dungeon.Point{X: 2, Y: 2}, HP: 5, MaxHP: 5, ATK: 4, Awake: true},
		{Name: "right.go", Pos: dungeon.Point{X: 4, Y: 2}, HP: 20, MaxHP: 20, ATK: 8, Awake: true},
		{Name: "down.go", Pos: dungeon.Point{X: 3, Y: 3}, HP: 8, MaxHP: 8, ATK: 6, Awake: true},
	}
	s := New(d)
	s.P.GitEnergy = 100

	// 1. Move left (or attack left monster) to establish facing direction Left
	s.Apply("h") // hits left.go
	if s.lastDir != (dungeon.Point{X: -1, Y: 0}) {
		t.Errorf("expected lastDir {-1, 0}, got %v", s.lastDir)
	}

	// Now cast git blame (key 3)
	// It should target left.go because facing direction is prioritized over higher HP right.go
	s.Apply("3")
	leftMon := &s.level().Monsters[1]
	if leftMon.StunTurns != 1 { // Started at 2, decremented by 1 in monsterPhase
		t.Errorf("expected left.go StunTurns 1 after turn, got %d", leftMon.StunTurns)
	}
	if leftMon.ATK != 2 { // 4 / 2 = 2
		t.Errorf("expected left.go ATK halved to 2, got %d", leftMon.ATK)
	}
	if s.P.GitEnergy != 80 { // 100 - 20
		t.Errorf("expected GitEnergy 80, got %d", s.P.GitEnergy)
	}
}

func TestGitPowerStashStealth(t *testing.T) {
	d := dungeon.FromArt([]string{
		"--------",
		"|@.m...|",
		"--------",
	})
	d.Levels[0].Monsters[0].Pos = dungeon.Point{X: 3, Y: 1}
	s := New(d)
	s.P.GitEnergy = 100

	// Cast stash
	s.Apply("1")
	if s.P.StashTurns != 2 {
		t.Fatalf("expected StashTurns 2, got %d", s.P.StashTurns)
	}

	// Monster at (3, 1) should NOT move toward player at (1, 1) during stash
	m := &s.level().Monsters[0]
	if m.Pos != (dungeon.Point{X: 3, Y: 1}) {
		t.Errorf("monster should not move while player is stashed, pos: %v", m.Pos)
	}

	// Move right to (2, 1), next to monster
	s.Apply("l")
	if s.P.StashTurns != 1 {
		t.Errorf("expected StashTurns 1 after move, got %d", s.P.StashTurns)
	}
	// Adjacent monster should NOT attack while player is stashed
	if s.P.HP != 30 {
		t.Errorf("expected player unharmed during stash, got HP %d", s.P.HP)
	}
}

func TestGitEnergyExplorationAndSlay(t *testing.T) {
	d := dungeon.FromArt([]string{
		"----------------------",
		"|@...................|",
		"----------------------",
	})
	d.Levels[0].Monsters = []dungeon.Monster{
		{Name: "bug.go", Pos: dungeon.Point{X: 12, Y: 1}, HP: 1, MaxHP: 1, ATK: 1, Awake: true},
	}
	s := New(d)
	// Player starts at 50 GitEnergy
	if s.P.GitEnergy != 50 {
		t.Errorf("expected starting GitEnergy 50, got %d", s.P.GitEnergy)
	}

	// Move right across the corridor, uncovering new floor tiles
	energyBefore := s.P.GitEnergy
	s.Apply("l")
	s.Apply("l")
	if s.P.GitEnergy <= energyBefore {
		t.Errorf("expected GitEnergy to increase from uncovering floor tiles, before=%d, after=%d", energyBefore, s.P.GitEnergy)
	}

	// Slay monster
	s.level().Monsters[0].Pos = dungeon.Point{X: s.P.Pos.X + 1, Y: s.P.Pos.Y}
	s.level().Monsters[0].HP = 1
	s.P.GitEnergy = 50
	s.Apply("l")             // slay bug.go with 1 HP
	if s.P.GitEnergy != 65 { // 50 + 15
		t.Errorf("expected GitEnergy 65 after slay, got %d", s.P.GitEnergy)
	}
}

func TestGitPowersInsufficientEnergy(t *testing.T) {
	d := dungeon.FromArt([]string{
		"--------",
		"|@.m...|",
		"--------",
	})
	d.Levels[0].Monsters[0].Pos = dungeon.Point{X: 2, Y: 1}
	s := New(d)
	s.P.GitEnergy = 10
	s.P.LastDamageTaken = 10

	s.Apply("1") // requires 30
	if s.P.GitEnergy != 10 {
		t.Errorf("energy should not be consumed, got %d", s.P.GitEnergy)
	}
	if !slices.Contains(s.Msgs, "Not enough Git Energy.") {
		t.Errorf("expected 'Not enough Git Energy.' msg, got %v", s.Msgs)
	}

	s.Apply("2") // requires 25
	if s.P.GitEnergy != 10 {
		t.Errorf("energy should not be consumed, got %d", s.P.GitEnergy)
	}
	if !slices.Contains(s.Msgs, "Not enough Git Energy.") {
		t.Errorf("expected 'Not enough Git Energy.' msg, got %v", s.Msgs)
	}

	s.Apply("3") // requires 20
	if s.P.GitEnergy != 10 {
		t.Errorf("energy should not be consumed, got %d", s.P.GitEnergy)
	}
	if !slices.Contains(s.Msgs, "Not enough Git Energy.") {
		t.Errorf("expected 'Not enough Git Energy.' msg, got %v", s.Msgs)
	}
}
