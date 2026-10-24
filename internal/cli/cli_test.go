package cli

import (
	"bytes"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/dpologdvinity/repodungeon/internal/testrepo"
)

func env(tty bool) Env {
	return Env{Version: "1.2.3", IsTTY: tty,
		Now: func() time.Time { return time.Date(2026, 10, 23, 0, 0, 0, 0, time.UTC) },
		RunTUI: func(m tea.Model) (tea.Model, error) {
			m, _ = m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
			return m, nil
		}}
}

func run(e Env, args ...string) (string, string, int) {
	var o, er bytes.Buffer
	c := Run(args, &o, &er, e)
	return o.String(), er.String(), c
}

func goodRepo(t *testing.T) *testrepo.Repo {
	r := testrepo.New(t)
	r.Write("main.go", "package main\n// TODO: win\n")
	r.Write("src/util.go", "package src\n")
	r.Commit("Ada", "ada@example.com", "init")
	return r
}

func TestRunPrintsCard(t *testing.T) {
	r := goodRepo(t)
	out, errOut, code := run(env(true), "-C", r.Dir)
	if code != 0 || errOut != "" || !strings.HasPrefix(out, "+------------------------------------------+\n| repodungeon: ") ||
		!strings.Contains(out, "| fled the dungeon on depth 1") || !strings.Contains(out, "| 2026-10-23") {
		t.Fatalf("%d %q %q", code, out, errOut)
	}
}

func TestRunErrors(t *testing.T) {
	notRepo := t.TempDir()
	bin := testrepo.New(t)
	bin.Write("logo.png", "\x00\x01")
	bin.Commit("A", "a@x", "bin")
	cases := []struct {
		e    Env
		args []string
		msg  string
	}{
		{env(true), []string{"-C", notRepo}, "repodungeon: not a git repository: " + notRepo + "\n"},
		{env(true), []string{"-C", testrepo.New(t).Dir}, "repodungeon: repository has no commits\n"},
		{env(true), []string{"-C", bin.Dir}, "repodungeon: no text files to fight\n"},
		{env(false), []string{"-C", goodRepo(t).Dir}, "repodungeon: needs an interactive terminal\n"},
		{env(true), []string{"extra"}, "repodungeon: unexpected argument: extra\n"},
	}
	for _, c := range cases {
		if _, e, code := run(c.e, c.args...); code != 2 || !strings.HasPrefix(e, c.msg) {
			t.Errorf("%v: %d %q", c.args, code, e)
		}
	}
	if _, e, code := run(env(true), "--bogus"); code != 2 || !strings.Contains(e, "usage: repodungeon [-C dir]") {
		t.Errorf("bogus: %d %q", code, e)
	}
}

func TestHelpVersion(t *testing.T) {
	if o, _, c := run(env(true), "-h"); c != 0 || o != usage {
		t.Fatal("help")
	}
	if o, _, c := run(env(true), "--version"); c != 0 || o != "repodungeon 1.2.3\n" {
		t.Fatal("version")
	}
}
