package tui

import (
	"regexp"
	"strings"
	"testing"

	"github.com/dpologdvinity/repodungeon/internal/dungeon"
	"github.com/dpologdvinity/repodungeon/internal/game"
)

// ansi matches one SGR color escape sequence.
var ansi = regexp.MustCompile("\x1b\\[[0-9;]*m")

// strip removes color codes so the text can be compared with game.Render.
func strip(s string) string { return ansi.ReplaceAllString(s, "") }

// colorState builds a small level that shows every kind of map glyph.
func colorState() *game.State {
	d := dungeon.FromArt([]string{"-----", "|@.+#", "|..>.", "-----"})
	l := d.Levels[0]
	l.Items = []dungeon.Item{{Kind: dungeon.Potion, Source: "a", Pos: dungeon.Point{X: 2, Y: 2}}}
	l.Monsters = []dungeon.Monster{
		{Name: "m.go", Glyph: 'm', Pos: dungeon.Point{X: 1, Y: 2}, HP: 3, MaxHP: 3, ATK: 1},
		{Name: "boss.go", Glyph: '&', Pos: dungeon.Point{X: 4, Y: 2}, HP: 9, MaxHP: 9, ATK: 1, Boss: true},
	}
	return game.New(d)
}

// TestColorizeKeepsText proves coloring only adds escape codes: with them
// stripped, the screen is byte-for-byte what game.Render returns, in every mode.
func TestColorizeKeepsText(t *testing.T) {
	s := colorState()
	for _, setup := range []func(){
		func() {},
		func() { s.Apply("i") },
		func() { s.Apply("esc"); s.OpenHelp() },
		func() { s.Apply("x") }, // help page 2
		func() { s.Apply("x"); s.P.HP = 5 },
		func() { s.Apply("q") },
	} {
		setup()
		if got, want := strip(colorize(s)), s.Render(); got != want {
			t.Fatalf("mode %v: text changed\ngot:\n%s\nwant:\n%s", s.Mode, got, want)
		}
	}
}

// TestColorizeMap proves the map glyphs get their colors.
func TestColorizeMap(t *testing.T) {
	out := colorize(colorState())
	for glyph, want := range map[string]string{
		"@": "\x1b[1;92m@", "&": "\x1b[1;91m&", "m": "\x1b[31mm", "!": "\x1b[95m!",
		"+": "\x1b[33m+", ">": "\x1b[1;93m>", "-": "\x1b[37m-",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("glyph %q: missing %q", glyph, want)
		}
	}
}

// TestColorizeHP proves the HP number follows health: green, yellow, red.
func TestColorizeHP(t *testing.T) {
	s := colorState()
	for hp, want := range map[int]string{30: "\x1b[32mHP 30/30", 12: "\x1b[33mHP 12/30", 4: "\x1b[1;91mHP 4/30"} {
		s.P.HP = hp
		if out := colorize(s); !strings.Contains(out, want) {
			t.Errorf("hp %d: missing %q", hp, want)
		}
	}
}

// TestColorizeWeaponAndTelegraph proves weapons and telegraphed monsters get styled.
func TestColorizeWeaponAndTelegraph(t *testing.T) {
	d := dungeon.FromArt([]string{"-----", "|@..|", "-----"})
	l := d.Levels[0]
	l.Items = []dungeon.Item{{Kind: dungeon.Weapon, Archetype: dungeon.Spear, Pos: dungeon.Point{X: 2, Y: 1}}}
	l.Monsters = []dungeon.Monster{
		{Name: "m.go", Glyph: 'm', Pos: dungeon.Point{X: 3, Y: 1}, HP: 5, MaxHP: 5, ATK: 1, Awake: true, Telegraph: true},
	}
	s := game.New(d)
	out := colorize(s)
	// Weapon glyph ')' should be formatted in bold blue/cyan
	if !strings.Contains(out, "\x1b[1;94m)") && !strings.Contains(out, "\x1b[1;36m)") {
		t.Errorf("weapon glyph missing expected blue/cyan color: %q", out)
	}
	// Telegraphed monster alert should be formatted in bold red
	if !strings.Contains(out, "\x1b[1;91m!") && !strings.Contains(out, "\x1b[1;31m!") {
		t.Errorf("telegraphed monster missing expected bold red alert color: %q", out)
	}
}

// TestColorizeKeepsTextWithWeaponsAndTelegraph proves strip(colorize(s)) == s.Render()
// with weapons, telegraphed monsters, and new status HUD.
func TestColorizeKeepsTextWithWeaponsAndTelegraph(t *testing.T) {
	d := dungeon.FromArt([]string{"------", "|@...|", "------"})
	l := d.Levels[0]
	l.Items = []dungeon.Item{
		{Kind: dungeon.Weapon, Archetype: dungeon.Spear, Pos: dungeon.Point{X: 2, Y: 1}},
		{Kind: dungeon.Potion, Pos: dungeon.Point{X: 3, Y: 1}},
	}
	l.Monsters = []dungeon.Monster{
		{Name: "telegraph.go", Glyph: 't', Pos: dungeon.Point{X: 4, Y: 1}, HP: 5, MaxHP: 5, ATK: 2, Awake: true, Telegraph: true},
	}
	s := game.New(d)
	s.P.Weapon = &dungeon.Item{Kind: dungeon.Weapon, Archetype: dungeon.Sword, BonusATK: 3}
	s.P.GitEnergy = 80
	s.P.Inv = append(s.P.Inv, dungeon.Item{Kind: dungeon.Weapon, Archetype: dungeon.Spear, Source: "tools/test.go"})

	for _, setup := range []func(){
		func() {},
		func() { s.Apply("i") },
		func() { s.Apply("esc"); s.OpenHelp() },
		func() { s.Apply(" ") }, // help page 2
		func() { s.Apply(" "); s.P.HP = 5 },
		func() { s.Apply("q") },
	} {
		setup()
		if got, want := strip(colorize(s)), s.Render(); got != want {
			t.Fatalf("mode %v: text changed\ngot:\n%s\nwant:\n%s", s.Mode, got, want)
		}
	}
}

// TestColorizeInventoryWeapons verifies weapons in inventory are styled.
func TestColorizeInventoryWeapons(t *testing.T) {
	d := dungeon.FromArt([]string{"-----", "|@..|", "-----"})
	s := game.New(d)
	s.P.Inv = []dungeon.Item{
		{Kind: dungeon.Weapon, Archetype: dungeon.Hammer, Source: "tools/build.go"},
	}
	s.Apply("i")
	out := colorize(s)
	if !strings.Contains(out, "\x1b[1;94m") && !strings.Contains(out, "\x1b[1;36m") {
		t.Errorf("inventory weapon line missing weapon color: %q", out)
	}
}
