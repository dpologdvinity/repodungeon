// Package cli wires scan, generation, the TUI and the end card together.
//
// Run is the whole program behind main. Everything that touches the outside
// world (terminal check, TUI runner, clock, version) comes in through Env,
// so tests can run it with fake values and capture its output (SPEC 3, 9).
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

// usage is the help text, printed to stdout for -h and to stderr after a bad flag (SPEC 3).
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
	// Version is printed by --version.
	Version string
	// IsTTY is true when stdin and stdout are both terminals. Run refuses to start the TUI otherwise.
	IsTTY bool // stdin and stdout are both terminals
	// RunTUI runs the model to completion and returns the final model. The real one wraps
	// tea.NewProgram(m).Run(). Tests pass a fake that sends keys, so no terminal is needed.
	RunTUI func(m tea.Model) (tea.Model, error)
	// Now is the clock. It is injected only for the date on the card.
	Now func() time.Time
}

// Run executes the program and returns the process exit code.
//
// Flow (SPEC 3): parse flags, scan the repo, generate the dungeon, check
// for a terminal, run the TUI, print the card to stdout.
//
// Exit codes: 0 for success, -h and --version. 2 for every failure: bad
// flag, extra argument, scan or generation error, no terminal, TUI error.
// Errors go to stderr as "repodungeon: <message>". The terminal check comes
// after scan and generation, so repository errors show even without a TTY.
func Run(args []string, stdout, stderr io.Writer, env Env) int {
	// fail prints a one-line error to stderr and returns exit code 2.
	fail := func(format string, a ...any) int {
		_, _ = fmt.Fprintf(stderr, "repodungeon: "+format+"\n", a...)
		return 2
	}
	fs := flag.NewFlagSet("repodungeon", flag.ContinueOnError)
	fs.SetOutput(stderr)
	// Silence the flag package's own usage; Run prints the exact usage text itself.
	fs.Usage = func() {}
	dir := fs.String("C", ".", "repository directory")
	version := fs.Bool("version", false, "print version")
	// -h is not an error: usage goes to stdout and the code is 0.
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			_, _ = io.WriteString(stdout, usage) // best effort: nothing to do if stdout is closed
			return 0
		}
		_, _ = io.WriteString(stderr, usage) // best effort, as above
		return 2
	}
	if *version {
		_, _ = fmt.Fprintf(stdout, "repodungeon %s\n", env.Version)
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
	m, ok := final.(tui.Model)
	if !ok || m.S == nil {
		return fail("unexpected TUI model")
	}
	if _, err := io.WriteString(stdout, m.S.Card(env.Now())); err != nil {
		return fail("%v", err)
	}
	return 0
}
