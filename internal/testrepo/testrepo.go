// Package testrepo builds throwaway git repositories for tests.
package testrepo

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Repo is a git repository in a temp dir with fixed dates and identity.
type Repo struct {
	// Dir is the absolute path of the work tree. It is removed with the test.
	Dir string
	// t is the test that owns the repo; helpers fail it on any error.
	// n is the number of commits made so far. It picks the next fixture date,
	// so every commit gets a new month and history order is stable.
	t *testing.T
	n int
}

// Date returns 2024-01-15T12:00:00Z plus n months.
func Date(n int) time.Time {
	return time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC).AddDate(0, n, 0)
}

// New creates an empty repository on branch main.
// The branch is named explicitly so tests do not depend on the user's
// init.defaultBranch setting.
func New(t *testing.T) *Repo {
	t.Helper()
	r := &Repo{Dir: t.TempDir(), t: t}
	r.run("Fixture", "fixture@example.com", "init", "-q", "-b", "main")
	return r
}

// run runs git with args in Dir and returns its stdout. name and email set
// the commit author. The committer is always the fixture user. Dates come
// from Date(r.n). User and system git config are switched off so the
// machine's settings (signing, hooks, templates) cannot change the result.
// Any git failure fails the test at once.
func (r *Repo) run(name, email string, args ...string) string {
	r.t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = r.Dir
	d := Date(r.n).Format(time.RFC3339)
	cmd.Env = append(os.Environ(),
		"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME="+name, "GIT_AUTHOR_EMAIL="+email,
		"GIT_COMMITTER_NAME=Fixture", "GIT_COMMITTER_EMAIL=fixture@example.com",
		"GIT_AUTHOR_DATE="+d, "GIT_COMMITTER_DATE="+d)
	out, err := cmd.Output()
	if err != nil {
		r.t.Fatalf("git %s: %v", strings.Join(args, " "), err)
	}
	return string(out)
}

// Write creates path (relative to Dir) with content, making parent dirs.
func (r *Repo) Write(path, content string) {
	r.t.Helper()
	full := filepath.Join(r.Dir, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		r.t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		r.t.Fatal(err)
	}
}

// Remove deletes path (relative to Dir).
func (r *Repo) Remove(path string) {
	r.t.Helper()
	if err := os.Remove(filepath.Join(r.Dir, filepath.FromSlash(path))); err != nil {
		r.t.Fatal(err)
	}
}

// Commit stages everything, commits, and returns the full HEAD hash.
// Each commit gets the next fixture date.
func (r *Repo) Commit(name, email, msg string) string {
	r.t.Helper()
	r.run(name, email, "add", "-A")
	r.run(name, email, "commit", "-q", "--allow-empty", "-m", msg)
	h := strings.TrimSpace(r.run(name, email, "rev-parse", "HEAD"))
	r.n++
	return h
}

// Git runs git in Dir as the fixture user and returns stdout.
func (r *Repo) Git(args ...string) string {
	r.t.Helper()
	return r.run("Fixture", "fixture@example.com", args...)
}
