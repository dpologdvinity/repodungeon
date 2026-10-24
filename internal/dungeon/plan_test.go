package dungeon

import (
	"fmt"
	"slices"
	"testing"

	"github.com/dpologdvinity/repodungeon/internal/repo"
)

func tf(path string) repo.File { return repo.File{Path: path, Lines: 1} }

func names(rs []RoomSpec) []string {
	var out []string
	for _, r := range rs {
		out = append(out, r.Name)
	}
	return out
}

func TestPlanLevels(t *testing.T) {
	bin := tf("assets/logo.png")
	bin.Binary = true
	p := Plan([]repo.File{tf("README.md"), tf("go.mod"), tf("main.go"), bin,
		tf("internal/game/game.go"), tf("internal/repo/repo.go"), tf("docs/SPEC.md")})
	if len(p) != 3 {
		t.Fatalf("levels %d", len(p))
	}
	if !slices.Equal(names(p[0]), []string{"/"}) || !slices.Equal(names(p[1]), []string{"docs/", "internal/"}) ||
		!slices.Equal(names(p[2]), []string{"internal/game/", "internal/repo/"}) {
		t.Fatalf("%v %v %v", names(p[0]), names(p[1]), names(p[2]))
	}
	if len(p[0][0].Files) != 3 || p[0][0].Files[2].Path != "main.go" || len(p[1][1].Files) != 0 {
		t.Fatalf("files %+v", p[0][0].Files)
	}
}

func TestPlanFoldsDeepDirs(t *testing.T) {
	p := Plan([]repo.File{tf("a/b/c/d/e/f/g/h/i/deep.txt")})
	if len(p) != 8 || !slices.Equal(names(p[7]), []string{"a/b/c/d/e/f/g/"}) || len(p[7][0].Files) != 1 {
		t.Fatalf("%d levels, last %v", len(p), names(p[len(p)-1]))
	}
}

func TestPlanMisc(t *testing.T) {
	var fs []repo.File
	for i := 0; i < 12; i++ {
		for j := 0; j <= i; j++ {
			fs = append(fs, tf(fmt.Sprintf("d%02d/f%02d.go", i, j)))
		}
	}
	p := Plan(fs)
	want := []string{"d04/", "d05/", "d06/", "d07/", "d08/", "d09/", "d10/", "d11/", "misc/"}
	if !slices.Equal(names(p[1]), want) || len(p[1][8].Files) != 10 {
		t.Fatalf("%v misc=%d", names(p[1]), len(p[1][len(p[1])-1].Files))
	}
}
