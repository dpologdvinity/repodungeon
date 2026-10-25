package game

import (
	"strings"
	"testing"

	"github.com/dpologdvinity/repodungeon/internal/dungeon"
)

func TestRenderNewSystems(t *testing.T) {
	d := dungeon.FromArt([]string{
		"---|",
		"|@.|",
		"---|",
	})
	s := New(d)
	s.P.Weapon = &dungeon.Item{Kind: dungeon.Weapon, Archetype: dungeon.Spear, BonusATK: 2}
	s.P.GitEnergy = 75

	out := s.Render()
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 24 {
		t.Fatalf("expected exactly 24 screen rows, got %d", len(lines))
	}
	for i, line := range lines {
		if len([]rune(line)) > 80 {
			t.Errorf("line %d exceeds 80 runes: %q", i, line)
		}
	}
	if !strings.Contains(lines[22], "EN 75") && !strings.Contains(lines[22], "Spear") {
		t.Errorf("status line missing energy or weapon info: %q", lines[22])
	}
}

func TestRenderCellWeaponAndTelegraph(t *testing.T) {
	d := dungeon.FromArt([]string{
		"-----|",
		"|@...|",
		"-----|",
	})
	s := New(d)
	// Place weapon at (2,1)
	s.level().Items = append(s.level().Items, dungeon.Item{
		Kind:      dungeon.Weapon,
		Archetype: dungeon.Sword,
		BonusATK:  3,
		Pos:       dungeon.Point{X: 2, Y: 1},
	})
	// Place telegraphed monster at (3,1)
	s.level().Monsters = []dungeon.Monster{
		{
			Name:      "goblin.go",
			Glyph:     'g',
			Pos:       dungeon.Point{X: 3, Y: 1},
			HP:        10,
			MaxHP:     10,
			ATK:       2,
			Awake:     true,
			Telegraph: true,
		},
	}

	out := s.Render()
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	// Map row 1 (screen row 2, index 2) should show:
	// @ at (1,1), ) at (2,1), ! at (3,1)
	mapRow := lines[2]
	if !strings.Contains(mapRow, "|@)!.|") {
		t.Errorf("expected map row to contain |@)!.|, got %q", mapRow)
	}
}

func TestRenderHelpLineGitPowers(t *testing.T) {
	d := dungeon.FromArt([]string{
		"---|",
		"|@.|",
		"---|",
	})
	s := New(d)
	out := s.Render()
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if !strings.Contains(lines[23], "1-3 git powers") {
		t.Errorf("help line missing git powers: %q", lines[23])
	}
}

func TestRenderHelpRowsContent(t *testing.T) {
	d := dungeon.FromArt([]string{
		"---|",
		"|@.|",
		"---|",
	})
	s := New(d)
	s.OpenHelp() // page 1
	p1 := s.Render()
	if !strings.Contains(p1, "git stash") || !strings.Contains(p1, "git revert") || !strings.Contains(p1, "git blame") {
		t.Errorf("help page 1 missing git powers info:\n%s", p1)
	}
	if !strings.Contains(p1, "rest") || !strings.Contains(p1, "(1 HP per 4 turns)") {
		t.Errorf("help page 1 missing rest explanation or correct rate (1 HP per 4 turns):\n%s", p1)
	}

	s.Apply(" ") // next page (page 2)
	p2 := s.Render()
	if !strings.Contains(p2, "weapon") || !strings.Contains(p2, "Spear") || !strings.Contains(p2, "Hammer") {
		t.Errorf("help page 2 missing weapon archetypes:\n%s", p2)
	}
	if !strings.Contains(p2, "telegraph") && !strings.Contains(p2, "Telegraph") {
		t.Errorf("help page 2 missing monster telegraph info:\n%s", p2)
	}
}

func TestRenderStatusRowUnarmedAndEquipped(t *testing.T) {
	d := dungeon.FromArt([]string{
		"---|",
		"|@.|",
		"---|",
	})
	s := New(d)
	// Default: unarmed
	out := s.Render()
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if !strings.Contains(lines[22], "[Unarmed]") {
		t.Errorf("expected [Unarmed] in status row: %q", lines[22])
	}

	// Equip sword
	s.P.Weapon = &dungeon.Item{Kind: dungeon.Weapon, Archetype: dungeon.Sword, BonusATK: 3}
	out = s.Render()
	lines = strings.Split(strings.TrimRight(out, "\n"), "\n")
	if !strings.Contains(lines[22], "[Wpn: Sword]") {
		t.Errorf("expected [Wpn: Sword] in status row: %q", lines[22])
	}

	// Equip hammer
	s.P.Weapon = &dungeon.Item{Kind: dungeon.Weapon, Archetype: dungeon.Hammer, BonusATK: 4}
	out = s.Render()
	lines = strings.Split(strings.TrimRight(out, "\n"), "\n")
	if !strings.Contains(lines[22], "[Wpn: Hammer]") {
		t.Errorf("expected [Wpn: Hammer] in status row: %q", lines[22])
	}
}

func TestRenderStatusRowCappedAt80WithExtremeValues(t *testing.T) {
	d := dungeon.FromArt([]string{
		"---|",
		"|@.|",
		"---|",
	})
	s := New(d)
	s.P.HP = 9999
	s.P.MaxHP = 9999
	s.P.ATK = 999
	s.P.GitEnergy = 100
	s.P.Level = 99
	s.P.XP = 9999
	s.Turn = 99999
	s.P.Weapon = &dungeon.Item{Kind: dungeon.Weapon, Archetype: dungeon.Hammer}
	s.D.Levels[0].Rooms = []dungeon.Room{{Name: strings.Repeat("a_very_deep_directory_path/", 10), X: 0, Y: 0, W: 3, H: 1}}

	out := s.Render()
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	for i, line := range lines {
		if runes := len([]rune(line)); runes > 80 {
			t.Errorf("line %d has %d runes > 80: %q", i, runes, line)
		}
	}
}

func TestRenderDimensionsAcrossAllModes(t *testing.T) {
	d := dungeon.FromArt([]string{
		"---|",
		"|@.|",
		"---|",
	})
	s := New(d)
	s.P.Inv = []dungeon.Item{
		{Kind: dungeon.Weapon, Archetype: dungeon.Sword, Source: "tools/build.go"},
		{Kind: dungeon.Potion, Source: "TODO.md"},
	}

	verifyScreen := func(modeName string) {
		out := s.Render()
		lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
		if len(lines) != 24 {
			t.Fatalf("%s mode: expected 24 rows, got %d", modeName, len(lines))
		}
		for i, line := range lines {
			if runes := len([]rune(line)); runes > 80 {
				t.Errorf("%s mode line %d has %d runes > 80: %q", modeName, i, runes, line)
			}
		}
	}

	verifyScreen("Play")

	s.Apply("i") // Inventory mode
	verifyScreen("Inventory")

	s.Apply("esc")
	s.OpenHelp() // Help mode page 1
	verifyScreen("Help Page 1")

	s.Apply(" ") // Help mode page 2
	verifyScreen("Help Page 2")

	s.Apply(" ") // Back to Play
	s.Outcome = Died
	verifyScreen("Game Over (Died)")

	s.Outcome = Won
	verifyScreen("Game Over (Won)")
}

func TestRenderCellTelegraphDeadOrOutOfView(t *testing.T) {
	d := dungeon.FromArt([]string{
		"----------------",
		"|@.............|",
		"----------------",
	})
	s := New(d)
	// Dead telegraphed monster at (2,1)
	s.level().Monsters = []dungeon.Monster{
		{
			Name:      "dead.go",
			Glyph:     'd',
			Pos:       dungeon.Point{X: 2, Y: 1},
			HP:        0,
			MaxHP:     10,
			Telegraph: true,
		},
		{
			Name:      "far.go",
			Glyph:     'f',
			Pos:       dungeon.Point{X: 14, Y: 1}, // outside FOV (radius 5 from (1,1))
			HP:        10,
			MaxHP:     10,
			Awake:     true,
			Telegraph: true,
		},
	}

	out := s.Render()
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	mapRow := lines[2]
	// Tile (2,1) should show floor '.', not '!' or 'd'
	if strings.Contains(mapRow, "|@!") || strings.Contains(mapRow, "|@d") {
		t.Errorf("dead monster should not be rendered: %q", mapRow)
	}
	// Far monster outside FOV should not be visible as '!' or 'f'
	if strings.Contains(mapRow, "!") || strings.Contains(mapRow, "f") {
		t.Errorf("out-of-fov telegraphed monster should not be rendered: %q", mapRow)
	}
}

func TestRenderInventoryWeaponFormatting(t *testing.T) {
	d := dungeon.FromArt([]string{
		"---|",
		"|@.|",
		"---|",
	})
	s := New(d)
	s.P.Inv = append(s.P.Inv, dungeon.Item{
		Kind:      dungeon.Weapon,
		Archetype: dungeon.Sword,
		BonusATK:  2,
		Source:    "src/tool.go",
	})
	s.Apply("i")
	out := s.Render()
	want := "a) Sword (+2 ATK) (from src/tool.go)"
	if !strings.Contains(out, want) {
		t.Errorf("inventory rendering weapon got:\n%s\nwant substring: %q", out, want)
	}
}
