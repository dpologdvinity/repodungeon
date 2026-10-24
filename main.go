package main

import (
	"os"
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
	env := cli.Env{
		Version: version,
		IsTTY:   charDev(os.Stdin) && charDev(os.Stdout),
		RunTUI:  func(m tea.Model) (tea.Model, error) { return tea.NewProgram(m).Run() },
		Now:     time.Now,
	}
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr, env))
}
