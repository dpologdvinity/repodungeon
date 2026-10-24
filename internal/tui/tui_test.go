package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/dpologdvinity/repodungeon/internal/dungeon"
	"github.com/dpologdvinity/repodungeon/internal/game"
)

func press(m tea.Model, code rune, text string) (tea.Model, tea.Cmd) {
	return m.Update(tea.KeyPressMsg{Code: code, Text: text})
}

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
}
