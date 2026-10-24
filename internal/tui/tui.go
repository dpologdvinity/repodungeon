// Package tui is a thin Bubble Tea model over game.State.
package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/dpologdvinity/repodungeon/internal/game"
)

type Model struct{ S *game.State }

func (m Model) Init() tea.Cmd { return nil }

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

func (m Model) View() tea.View {
	v := tea.NewView(m.S.Render())
	v.AltScreen = true
	return v
}
