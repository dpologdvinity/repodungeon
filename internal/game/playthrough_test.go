package game

import (
	"strings"
	"testing"
	"time"

	"github.com/dpologdvinity/repodungeon/internal/dungeon"
	"github.com/dpologdvinity/repodungeon/internal/repo"
)

// nextKey returns the key that steps along a shortest walkable path to the
// nearest target; monsters are not obstacles, so the bot bumps into them.
func nextKey(s *State, targets map[dungeon.Point]bool) string {
	l := s.level()
	dirs := []struct {
		d dungeon.Point
		k string
	}{{dungeon.Point{X: 1}, "l"}, {dungeon.Point{X: -1}, "h"}, {dungeon.Point{Y: 1}, "j"}, {dungeon.Point{Y: -1}, "k"}}
	// first maps each reached tile to the first key of the path that reaches it (breadth-first search).
	first := map[dungeon.Point]string{}
	q := []dungeon.Point{s.P.Pos}
	seen := map[dungeon.Point]bool{s.P.Pos: true}
	for len(q) > 0 {
		p := q[0]
		q = q[1:]
		// The first target popped is a nearest one; its first step is the answer.
		if targets[p] && p != s.P.Pos {
			return first[p]
		}
		for _, dk := range dirs {
			n := dungeon.Point{X: p.X + dk.d.X, Y: p.Y + dk.d.Y}
			if seen[n] || !dungeon.Walkable(l.At(n)) {
				continue
			}
			seen[n] = true
			first[n] = dk.k
			// Neighbours of the start use their own key; others inherit the parent's first key.
			if p != s.P.Pos {
				first[n] = first[p]
			}
			q = append(q, n)
		}
	}
	// No reachable target: wait a turn.
	return "."
}

// TestPlaythrough drives a bot through a generated dungeon: it grabs loot,
// drinks a potion, descends past depth 1, and the run ends with a card.
func TestPlaythrough(t *testing.T) {
	f := func(p string, lines, churn, todos int) repo.File {
		return repo.File{Path: p, Lines: lines, Churn: churn, Todos: todos}
	}
	d, err := dungeon.Generate(&repo.Repo{Name: "bot", Head: "00000000000000ff" + strings.Repeat("0", 24), Files: []repo.File{
		f("a.go", 4, 1, 3), f("b.go", 4, 1, 0), f("pkg/c.go", 4, 1, 0), f("pkg/d.go", 9, 2, 0),
	}})
	if err != nil {
		t.Fatal(err)
	}
	s := New(d)
	// The bot is a test helper only; this makes the first potion matter.
	s.P.HP = 20 // so the first potion actually heals
	drank, deepest := false, 0
	for i := 0; i < 5000 && !s.Done(); i++ {
		l := s.level()
		targets := map[dungeon.Point]bool{}
		for _, it := range l.Items {
			targets[it.Pos] = true
		}
		switch {
		// Priority: heal when hurt, then collect loot, then take stairs, then hunt the boss.
		case len(s.P.Inv) > 0 && s.P.HP < s.P.MaxHP:
			keys(s, "i", "a")
			drank = true
		case len(targets) > 0:
			keys(s, nextKey(s, targets))
		case l.HasStairs && s.P.Pos == l.Stairs:
			keys(s, ">")
		case l.HasStairs:
			keys(s, nextKey(s, map[dungeon.Point]bool{l.Stairs: true}))
		default:
			for _, m := range l.Monsters {
				if m.Boss && m.HP > 0 {
					targets[m.Pos] = true
				}
			}
			keys(s, nextKey(s, targets))
		}
		// Track depth reached, to prove stairs work.
		deepest = max(deepest, s.Depth)
	}
	if !s.Done() || deepest < 1 || !drank {
		t.Fatalf("outcome %v deepest %d drank %v turn %d", s.Outcome, deepest, drank, s.Turn)
	}
	// The run is over by now, so the card must be printable. Its short hash comes from Head.
	card := s.Card(time.Date(2026, 10, 23, 0, 0, 0, 0, time.UTC))
	if !strings.Contains(card, "| repodungeon: bot @ 0000000") {
		t.Fatalf("card:\n%s", card)
	}
	t.Logf("outcome %v depth %d turns %d level %d", s.Outcome, s.Depth+1, s.Turn, s.P.Level)
}
