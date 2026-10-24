package repo

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/dpologdvinity/repodungeon/internal/testrepo"
)

func TestScan(t *testing.T) {
	r := testrepo.New(t)
	r.Write("README.md", "# demo\nTODO: one\nFIXME: two\nTODO three\nTODOS not counted\n")
	r.Write(".gitignore", "ignored.txt\n")
	r.Write("logo.png", "\x89PNG\x00\x00")
	r.Write("noeol.txt", "a\nb")
	r.Write("big.txt", strings.Repeat("a", MaxBlob+1))
	r.Write("src/main.go", "package main\n")
	if err := os.Symlink("README.md", filepath.Join(r.Dir, "link.md")); err != nil {
		t.Skip("symlinks unsupported")
	}
	first := r.Commit("Ada", "ada@example.com", "init")
	r.Write("src/main.go", "package main\n\nfunc main() {}\n")
	r.Commit("Ada", "ada@example.com", "two")
	r.Write("src/main.go", "package main\n\nfunc main() { println() }\n")
	r.Commit("Ada", "ada@example.com", "three")
	// a submodule entry (gitlink) without a real submodule checkout
	r.Git("update-index", "--add", "--cacheinfo", "160000,"+first+",sub")
	r.Git("commit", "-q", "-m", "add gitlink")
	head := strings.TrimSpace(r.Git("rev-parse", "HEAD"))
	r.Write("ignored.txt", "TODO\n")     // ignored, never committed
	r.Write("uncommitted.txt", "TODO\n") // untracked

	got, err := Scan(r.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != filepath.Base(r.Dir) || got.Head != head {
		t.Fatalf("name %q head %q", got.Name, got.Head)
	}
	var paths []string
	by := map[string]File{}
	for _, f := range got.Files {
		paths = append(paths, f.Path)
		by[f.Path] = f
	}
	if !slices.Equal(paths, []string{".gitignore", "README.md", "big.txt", "logo.png", "noeol.txt", "src/main.go"}) {
		t.Fatalf("paths %v", paths)
	}
	if f := by["README.md"]; f.Lines != 5 || f.Todos != 2 || f.Fixmes != 1 || f.Churn != 1 || !f.Text() {
		t.Errorf("README %+v", f)
	}
	if f := by["big.txt"]; !f.Skipped || f.Text() {
		t.Errorf("big %+v", f)
	}
	if f := by["logo.png"]; !f.Binary || f.Text() {
		t.Errorf("logo %+v", f)
	}
	if by["noeol.txt"].Lines != 2 {
		t.Errorf("noeol %+v", by["noeol.txt"])
	}
	if f := by["src/main.go"]; f.Churn != 3 || f.Lines != 3 {
		t.Errorf("main %+v", f)
	}
}

func TestScanErrors(t *testing.T) {
	if _, err := Scan(t.TempDir()); !errors.Is(err, ErrNotRepo) {
		t.Errorf("not repo: %v", err)
	}
	if _, err := Scan(testrepo.New(t).Dir); !errors.Is(err, ErrNoCommits) {
		t.Errorf("empty: %v", err)
	}
}

func TestScanSanitizesControlCharsInPaths(t *testing.T) {
	r := testrepo.New(t)
	r.Write("e\x1b[31mRED\nX.go", "hi\n")
	r.Commit("Ada", "ada@example.com", "init")
	got, err := Scan(r.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Files) != 1 || got.Files[0].Path != "e?[31mRED?X.go" || got.Files[0].Churn != 1 {
		t.Fatalf("%+v", got.Files)
	}
}

func TestScanSkipsBlobsMissingFromPartialClone(t *testing.T) {
	src := testrepo.New(t)
	src.Git("config", "uploadpack.allowFilter", "true")
	src.Git("config", "uploadpack.allowAnySHA1InWant", "true")
	src.Write("a.go", "package a\n")
	src.Commit("Ada", "ada@example.com", "init")
	clone := filepath.Join(t.TempDir(), "clone")
	cmd := exec.Command("git", "clone", "-q", "--no-checkout", "--filter=blob:none", "file://"+src.Dir, clone)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("partial clone unsupported: %v %s", err, out)
	}
	got, err := Scan(clone)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Files) != 1 || !got.Files[0].Skipped {
		t.Fatalf("%+v", got.Files)
	}
}
