// Package tui is a thin Bubble Tea model over game.State.
//
// All rules live in package game. The model only forwards key presses to
// State.Apply, draws State.Render and quits when the run is over (SPEC 8).
// It is the only package besides main that imports Bubble Tea.
package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/dpologdvinity/repodungeon/internal/game"
)

// Model is the Bubble Tea model. S is a pointer, so the value receivers below
// still change the one shared game state.
type Model struct{ S *game.State }

// Init returns no command: the game needs no startup work.
func (m Model) Init() tea.Cmd { return nil }

// Update feeds key presses to the game and quits when the run is over.
// Other messages are ignored. A key after the run has ended quits at once.
// The game-over screen says "Press any key", which this implements.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	if m.S.Done() {
		return m, tea.Quit
	}
	m.S.Apply(k.String())
	if m.S.Outcome == game.Quit {
		return m, tea.Quit
	}
	return m, nil
}

// View draws the colored game screen in the alternate screen, so the shell
// scrollback is left alone.
func (m Model) View() tea.View {
	v := tea.NewView(colorize(m.S))
	v.AltScreen = true
	return v
}
