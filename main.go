// Command repodungeon turns a git repository into a small dungeon game.
//
// This file only wires the real terminal, clock and version into the cli
// package, then exits with the exit code that cli.Run returns.
package main

import (
	"os"
	"runtime/debug"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/term"

	"github.com/dpologdvinity/repodungeon/internal/cli"
)

// version is the release version shown to the user. The default "dev" is
// replaced at link time by the release build. If it is still "dev", main
// falls back to the module version stored in the binary (go install).
var version = "dev"

// isTTY reports whether f is an interactive terminal. It returns false if
// f is nil, a regular file, a pipe, or a special character device like
// /dev/null that cannot host an interactive session.
func isTTY(f *os.File) bool {
	return f != nil && term.IsTerminal(f.Fd())
}

// main resolves the version, builds the cli environment and runs the
// command. The TUI is only allowed when both stdin and stdout are terminals.
func main() {
	// Release builds set version at link time. For go install, read it from
	// the build info. "(devel)" means a local build, so keep "dev".
	if version == "dev" {
		if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
			version = strings.TrimPrefix(bi.Main.Version, "v")
		}
	}
	env := cli.Env{
		Version: version,
		IsTTY:   isTTY(os.Stdin) && isTTY(os.Stdout),
		RunTUI:  func(m tea.Model) (tea.Model, error) { return tea.NewProgram(m).Run() },
		Now:     time.Now,
	}
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr, env))
}
