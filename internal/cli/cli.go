// Package cli wires scan, generation, the TUI and the end card together.
package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/dpologdvinity/repodungeon/internal/dungeon"
	"github.com/dpologdvinity/repodungeon/internal/game"
	"github.com/dpologdvinity/repodungeon/internal/repo"
	"github.com/dpologdvinity/repodungeon/internal/tui"
)

const usage = `usage: repodungeon [-C dir]

A roguelike generated from your git repository.
Directories are rooms, files are monsters, TODOs are loot.

flags:
  -C dir      repository directory (default ".")
  --version   print version

keys: hjkl or arrows move, . wait, > descend, i inventory, q quit
`

// Env holds everything Run needs from the outside world.
type Env struct {
	Version string
	IsTTY   bool // stdin and stdout are both terminals
	RunTUI  func(m tea.Model) (tea.Model, error)
	Now     func() time.Time
}

func Run(args []string, stdout, stderr io.Writer, env Env) int {
	fail := func(format string, a ...any) int {
		fmt.Fprintf(stderr, "repodungeon: "+format+"\n", a...)
		return 2
	}
	fs := flag.NewFlagSet("repodungeon", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {}
	dir := fs.String("C", ".", "repository directory")
	version := fs.Bool("version", false, "print version")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			io.WriteString(stdout, usage)
			return 0
		}
		io.WriteString(stderr, usage)
		return 2
	}
	if *version {
		fmt.Fprintf(stdout, "repodungeon %s\n", env.Version)
		return 0
	}
	if fs.NArg() > 0 {
		return fail("unexpected argument: %s", fs.Arg(0))
	}

	r, err := repo.Scan(*dir)
	switch {
	case errors.Is(err, repo.ErrNotRepo):
		return fail("%v: %s", err, *dir)
	case err != nil:
		return fail("%v", err)
	}
	d, err := dungeon.Generate(r)
	if err != nil {
		return fail("%v", err)
	}
	if !env.IsTTY {
		return fail("needs an interactive terminal")
	}
	final, err := env.RunTUI(tui.Model{S: game.New(d)})
	if err != nil {
		return fail("%v", err)
	}
	io.WriteString(stdout, final.(tui.Model).S.Card(env.Now()))
	return 0
}
