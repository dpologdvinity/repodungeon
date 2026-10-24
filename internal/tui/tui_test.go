package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/dpologdvinity/repodungeon/internal/dungeon"
	"github.com/dpologdvinity/repodungeon/internal/game"
)

// press sends m a key press with the given key code and text, as the terminal would.
func press(m tea.Model, code rune, text string) (tea.Model, tea.Cmd) {
	return m.Update(tea.KeyPressMsg{Code: code, Text: text})
}

// TestModel proves that Init returns nil, keys move the player (letters and arrows),
// View shows the render in the alternate screen, and q returns tea.Quit.
func TestModel(t *testing.T) {
	m := Model{S: game.New(dungeon.FromArt([]string{"|@..|"}))}
	if m.Init() != nil {
		t.Fatal("Init must return nil")
	}
	next, cmd := press(m, 'l', "l")
	if cmd != nil || next.(Model).S.P.Pos.X != 2 {
		t.Fatal("l must move right")
	}
	next, _ = next.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	if next.(Model).S.P.Pos.X != 1 {
		t.Fatal("left arrow must move left")
	}
	v := next.View()
	if v.Content != next.(Model).S.Render() || !v.AltScreen {
		t.Fatal("view")
	}
	_, cmd = press(next, 'q', "q")
	if cmd == nil {
		t.Fatal("q must quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("q must return tea.Quit")
	}

	// Non-key message is ignored
	if _, cmd := m.Update("unknown"); cmd != nil {
		t.Fatal("non-key message must return nil cmd")
	}

	// Pressing any key after game is over returns tea.Quit
	gameOver := Model{S: game.New(dungeon.FromArt([]string{"|@..|"}))}
	gameOver.S.Outcome = game.Won
	if _, cmd := press(gameOver, 'x', "x"); cmd == nil {
		t.Fatal("key on finished game must quit")
	}
}
