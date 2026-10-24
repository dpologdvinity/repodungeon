package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/dpologdvinity/repodungeon/internal/testrepo"
)

// env returns a fake Env: fixed version and date, and a RunTUI that presses q at once
// so the run ends with a quit card. tty sets IsTTY.
func env(tty bool) Env {
	return Env{Version: "1.2.3", IsTTY: tty,
		Now: func() time.Time { return time.Date(2026, 10, 23, 0, 0, 0, 0, time.UTC) },
		RunTUI: func(m tea.Model) (tea.Model, error) {
			m, _ = m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
			return m, nil
		}}
}

// run calls Run with e and returns stdout, stderr and the exit code.
func run(e Env, args ...string) (string, string, int) {
	var o, er bytes.Buffer
	c := Run(args, &o, &er, e)
	return o.String(), er.String(), c
}

// goodRepo makes a small valid git repo with two text files and one commit.
func goodRepo(t *testing.T) *testrepo.Repo {
	r := testrepo.New(t)
	r.Write("main.go", "package main\n// TODO: win\n")
	r.Write("src/util.go", "package src\n")
	r.Commit("Ada", "ada@example.com", "init")
	return r
}

// TestRunPrintsCard proves the happy path: exit 0, empty stderr, and the card on stdout.
func TestRunPrintsCard(t *testing.T) {
	r := goodRepo(t)
	out, errOut, code := run(env(true), "-C", r.Dir)
	if code != 0 || errOut != "" || !strings.HasPrefix(out, "+------------------------------------------+\n| repodungeon: ") ||
		!strings.Contains(out, "| fled the dungeon on depth 1") || !strings.Contains(out, "| 2026-10-23") {
		t.Fatalf("%d %q %q", code, out, errOut)
	}
}

// TestRunErrors proves each failure gives exit code 2 and the exact stderr message
// (SPEC 3): not a repo, no commits, no text files, no terminal, extra argument,
// and that a bad flag prints the usage.
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

// TestHelpVersion proves that -h prints the usage and --version prints the version,
// both with exit code 0.
func TestHelpVersion(t *testing.T) {
	if o, _, c := run(env(true), "-h"); c != 0 || o != usage {
		t.Fatal("help")
	}
	if o, _, c := run(env(true), "--version"); c != 0 || o != "repodungeon 1.2.3\n" {
		t.Fatal("version")
	}
}

// TestRunUnexpectedTUIModel proves that an unexpected model returned by RunTUI
// returns exit code 2 and an error message rather than panicking.
func TestRunUnexpectedTUIModel(t *testing.T) {
	r := goodRepo(t)
	e := env(true)
	e.RunTUI = func(m tea.Model) (tea.Model, error) {
		return nil, nil
	}
	_, errOut, code := run(e, "-C", r.Dir)
	if code != 2 || !strings.Contains(errOut, "repodungeon: unexpected TUI model") {
		t.Fatalf("expected code 2 and unexpected TUI model message, got code %d %q", code, errOut)
	}
}

// errWriter fails on Write.
type errWriter struct{}

func (errWriter) Write(p []byte) (int, error) {
	return 0, errors.New("write error")
}

// TestRunTUIError proves that a failure returned by RunTUI returns exit code 2.
func TestRunTUIError(t *testing.T) {
	r := goodRepo(t)
	e := env(true)
	e.RunTUI = func(m tea.Model) (tea.Model, error) {
		return nil, errors.New("tui failure")
	}
	_, errOut, code := run(e, "-C", r.Dir)
	if code != 2 || !strings.Contains(errOut, "repodungeon: tui failure") {
		t.Fatalf("expected code 2 and tui failure, got %d %q", code, errOut)
	}
}

// TestRunStdoutError proves that a failure writing the card returns exit code 2.
func TestRunStdoutError(t *testing.T) {
	r := goodRepo(t)
	var er bytes.Buffer
	code := Run([]string{"-C", r.Dir}, errWriter{}, &er, env(true))
	if code != 2 || !strings.Contains(er.String(), "repodungeon: write error") {
		t.Fatalf("expected code 2 and write error, got %d %q", code, er.String())
	}
}

// TestRunOpensOnHelpScreen proves the game starts on the how-to-play screen,
// so a new player sees the rules before the map.
func TestRunOpensOnHelpScreen(t *testing.T) {
	r := goodRepo(t)
	e := env(true)
	var first string
	e.RunTUI = func(m tea.Model) (tea.Model, error) {
		first = m.View().Content
		m, _ = m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
		return m, nil
	}
	if _, _, code := run(e, "-C", r.Dir); code != 0 || !strings.Contains(first, "HOW TO PLAY") {
		t.Fatalf("code %d first screen:\n%s", code, first)
	}
}
