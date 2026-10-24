package main

import (
	"os"
	"runtime/debug"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/dpologdvinity/repodungeon/internal/cli"
)

var version = "dev"

func charDev(f *os.File) bool {
	fi, err := f.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

func main() {
	if version == "dev" {
		if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
			version = strings.TrimPrefix(bi.Main.Version, "v")
		}
	}
	env := cli.Env{
		Version: version,
		IsTTY:   charDev(os.Stdin) && charDev(os.Stdout),
		RunTUI:  func(m tea.Model) (tea.Model, error) { return tea.NewProgram(m).Run() },
		Now:     time.Now,
	}
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr, env))
}
