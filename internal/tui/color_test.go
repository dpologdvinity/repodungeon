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
